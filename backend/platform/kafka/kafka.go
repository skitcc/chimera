// Package kafka publishes outbox records and consumes domain events with retries and a DLQ.
package kafka

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/proto"

	eventsv1 "chimera/contracts/gen/events/v1"
	"chimera/contracts/events"
	"chimera/platform/apperr"
	"chimera/platform/env"
)

const (
	HeaderID          = "ce_id"
	HeaderType        = "ce_type"
	HeaderSource      = "ce_source"
	HeaderContentType = "content-type"
	ContentType       = "application/protobuf"
)

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

// NewProducer returns an idempotent producer that waits for all in-sync replicas.
func NewProducer(cfg env.Kafka) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
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
// record to the DLQ; any other error is retried with backoff.
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
	client     *kgo.Client
	handler    Handler
	log        Logger
	group      string
	maxBackoff time.Duration
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
	)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInternal, "open kafka consumer", err)
	}
	return &Consumer{client: client, handler: handler, log: log, group: group, maxBackoff: 30 * time.Second}, nil
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
func (c *Consumer) process(ctx context.Context, rec *kgo.Record) bool {
	backoff := 100 * time.Millisecond
	for {
		err := c.handle(ctx, rec)
		if err == nil {
			return true
		}
		if isPermanent(err) {
			dlqErr := c.toDLQ(ctx, rec, err)
			if dlqErr == nil {
				return true
			}
			err = dlqErr
		}
		c.log.ErrorContext(ctx, "kafka handle", "group", c.group, "topic", rec.Topic,
			"partition", rec.Partition, "offset", rec.Offset, "retry_in", backoff, "error", err)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, c.maxBackoff)
	}
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

func (c *Consumer) toDLQ(ctx context.Context, rec *kgo.Record, cause error) error {
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
	)
	if err := c.client.ProduceSync(ctx, dlq).FirstErr(); err != nil {
		return err
	}
	c.log.ErrorContext(ctx, "kafka record moved to dlq", "group", c.group, "topic", rec.Topic,
		"partition", rec.Partition, "offset", rec.Offset, "error", cause)
	return nil
}
