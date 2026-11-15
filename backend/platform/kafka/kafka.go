// Package kafka publishes outbox records and consumes domain events with bounded retries and a DLQ.
package kafka

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"

	"chimera/contracts/events"
	eventsv1 "chimera/contracts/gen/events/v1"
	"chimera/platform/apperr"
	"chimera/platform/env"
)

const (
	HeaderID          = "ce_id"
	HeaderType        = "ce_type"
	HeaderSource      = "ce_source"
	HeaderContentType = "content-type"
	// HeaderTraceParent carries the W3C trace context of the request that caused the event.
	HeaderTraceParent = "traceparent"
	ContentType       = "application/protobuf"
)

var tracer = otel.Tracer("chimera/platform/kafka")

type Logger interface {
	InfoContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
}

type Record struct {
	Topic   string
	Key     string
	Value   []byte
	Headers map[string]string
}

type Producer struct {
	client *kgo.Client
}

// NewProducer returns an idempotent producer that waits for all in-sync replicas
// and gives up on a record after cfg.ProduceTimeout.
func NewProducer(cfg env.Kafka) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordDeliveryTimeout(cfg.ProduceTimeout),
	)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInternal, "open kafka producer", err)
	}
	return &Producer{client: client}, nil
}

// Publish sends recs in order and returns one error slot per record.
func (p *Producer) Publish(ctx context.Context, recs []Record) []error {
	krecs := make([]*kgo.Record, 0, len(recs))
	for _, r := range recs {
		krecs = append(krecs, toKafka(r))
	}
	results := p.client.ProduceSync(ctx, krecs...)
	errs := make([]error, len(recs))
	for i, res := range results {
		errs[i] = res.Err
	}
	return errs
}

func (p *Producer) Ping(ctx context.Context) error {
	return p.client.Ping(ctx)
}

func (p *Producer) Close() {
	p.client.Close()
}

func toKafka(r Record) *kgo.Record {
	rec := &kgo.Record{Topic: r.Topic, Key: []byte(r.Key), Value: r.Value}
	for k, v := range r.Headers {
		rec.Headers = append(rec.Headers, kgo.RecordHeader{Key: k, Value: []byte(v)})
	}
	return rec
}

// Handler processes one event. Returning an error built by Permanent sends the
// record to the DLQ at once; any other error is retried with backoff up to the
// attempt limit, and then the record goes to the DLQ too.
type Handler func(ctx context.Context, event *eventsv1.Envelope) error

type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

// Permanent marks err as one that will not succeed on retry.
func Permanent(err error) error { return permanentError{err: err} }

func isPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p)
}

type Consumer struct {
	client      *kgo.Client
	handler     Handler
	log         Logger
	group       string
	maxAttempts int
	backoff     time.Duration
	maxBackoff  time.Duration
}

// NewConsumer joins group on topics. Offsets are committed only after a record is
// handled or moved to the DLQ, so a crash replays the unfinished records.
func NewConsumer(cfg env.Kafka, group string, topics []string, handler Handler, log Logger) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topics...),
		kgo.DisableAutoCommit(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordDeliveryTimeout(cfg.ProduceTimeout),
	)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInternal, "open kafka consumer", err)
	}
	return &Consumer{
		client:      client,
		handler:     handler,
		log:         log,
		group:       group,
		maxAttempts: cfg.RetryMaxAttempts,
		backoff:     cfg.RetryBackoff,
		maxBackoff:  cfg.RetryMaxBackoff,
	}, nil
}

func (c *Consumer) Ping(ctx context.Context) error {
	return c.client.Ping(ctx)
}

func (c *Consumer) Close() {
	c.client.Close()
}

func (c *Consumer) Run(ctx context.Context) {
	for {
		fetches := c.client.PollFetches(ctx)
		if fetches.IsClientClosed() || ctx.Err() != nil {
			return
		}
		fetches.EachError(func(topic string, partition int32, err error) {
			c.log.ErrorContext(ctx, "kafka fetch", "group", c.group, "topic", topic, "partition", partition, "error", err)
		})
		var done []*kgo.Record
		for _, rec := range fetches.Records() {
			if !c.process(ctx, rec) {
				break
			}
			done = append(done, rec)
		}
		if len(done) == 0 {
			continue
		}
		if err := c.client.CommitRecords(ctx, done...); err != nil {
			c.log.ErrorContext(ctx, "kafka commit", "group", c.group, "error", err)
		}
	}
}

// process returns false only when ctx ended before the record was finished.
// The handler runs in a consumer span that continues the producer's trace.
func (c *Consumer) process(ctx context.Context, rec *kgo.Record) bool {
	ctx, span := c.startSpan(ctx, rec)
	defer span.End()

	backoff := c.backoff
	for attempt := 1; ; attempt++ {
		err := c.handle(ctx, rec)
		if err == nil {
			return true
		}
		span.RecordError(err, trace.WithAttributes(attribute.Int("attempt", attempt)))
		if isPermanent(err) || attempt >= c.maxAttempts {
			dlqErr := c.toDLQ(ctx, rec, err, attempt)
			if dlqErr == nil {
				span.SetStatus(codes.Error, "moved to dlq")
				return true
			}
			err = dlqErr
		}
		c.log.ErrorContext(ctx, "kafka handle", "group", c.group, "topic", rec.Topic,
			"partition", rec.Partition, "offset", rec.Offset, "attempt", attempt, "retry_in", backoff, "error", err)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, c.maxBackoff)
	}
}

func (c *Consumer) startSpan(ctx context.Context, rec *kgo.Record) (context.Context, trace.Span) {
	ctx = otel.GetTextMapPropagator().Extract(ctx, &headerCarrier{rec: rec})
	return tracer.Start(ctx, rec.Topic+" process",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination.name", rec.Topic),
			attribute.String("messaging.consumer.group.name", c.group),
			attribute.Int("messaging.destination.partition.id", int(rec.Partition)),
			attribute.Int64("messaging.kafka.offset", rec.Offset),
		),
	)
}

// headerCarrier lets the propagator read and write the trace context in record headers.
type headerCarrier struct{ rec *kgo.Record }

func (h *headerCarrier) Get(key string) string {
	for _, hd := range h.rec.Headers {
		if hd.Key == key {
			return string(hd.Value)
		}
	}
	return ""
}

func (h *headerCarrier) Set(key, value string) {
	h.rec.Headers = append(h.rec.Headers, kgo.RecordHeader{Key: key, Value: []byte(value)})
}

func (h *headerCarrier) Keys() []string {
	keys := make([]string, 0, len(h.rec.Headers))
	for _, hd := range h.rec.Headers {
		keys = append(keys, hd.Key)
	}
	return keys
}

func (c *Consumer) handle(ctx context.Context, rec *kgo.Record) error {
	var event eventsv1.Envelope
	if err := proto.Unmarshal(rec.Value, &event); err != nil {
		return Permanent(err)
	}
	if event.GetEventId() == "" {
		return Permanent(errors.New("event without id"))
	}
	return c.handler(ctx, &event)
}

func (c *Consumer) toDLQ(ctx context.Context, rec *kgo.Record, cause error, attempts int) error {
	dlq := &kgo.Record{
		Topic:   rec.Topic + events.DLQSuffix,
		Key:     rec.Key,
		Value:   rec.Value,
		Headers: append([]kgo.RecordHeader(nil), rec.Headers...),
	}
	dlq.Headers = append(dlq.Headers,
		kgo.RecordHeader{Key: "error", Value: []byte(cause.Error())},
		kgo.RecordHeader{Key: "consumer_group", Value: []byte(c.group)},
		kgo.RecordHeader{Key: "source_topic", Value: []byte(rec.Topic)},
		kgo.RecordHeader{Key: "source_partition", Value: []byte(strconv.Itoa(int(rec.Partition)))},
		kgo.RecordHeader{Key: "source_offset", Value: []byte(strconv.FormatInt(rec.Offset, 10))},
		kgo.RecordHeader{Key: "attempts", Value: []byte(strconv.Itoa(attempts))},
	)
	if err := c.client.ProduceSync(ctx, dlq).FirstErr(); err != nil {
		return err
	}
	c.log.ErrorContext(ctx, "kafka record moved to dlq", "group", c.group, "topic", rec.Topic,
		"partition", rec.Partition, "offset", rec.Offset, "attempts", attempts, "error", cause)
	return nil
}
