package kafka

import (
	"context"
	"errors"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"chimera/contracts/events"
	"chimera/platform/apperr"
	"chimera/platform/env"
)

// replayGroup is shared by every service, so each DLQ record goes back once.
const replayGroup = "chimera.dlq-replay"

// dlqHeaders are the ones toDLQ adds; a record goes back without them.
var dlqHeaders = map[string]bool{
	"error": true, "consumer_group": true, "source_topic": true,
	"source_partition": true, "source_offset": true, "attempts": true,
}

// ReplayDLQ moves the records of the DLQs of topics back to their source topics
// and returns how many it moved. It stops when a poll finds nothing within idle.
// Every consumer group of the source topic sees a replayed record again; groups
// that handled it before skip it by event id.
func ReplayDLQ(ctx context.Context, cfg env.Kafka, topics []string, idle time.Duration, log Logger) (int, error) {
	dlqs := make([]string, 0, len(topics))
	for _, t := range topics {
		dlqs = append(dlqs, t+events.DLQSuffix)
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ConsumerGroup(replayGroup),
		kgo.ConsumeTopics(dlqs...),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.DisableAutoCommit(),
		kgo.RecordDeliveryTimeout(cfg.ProduceTimeout),
	)
	if err != nil {
		return 0, apperr.Wrap(apperr.CodeInternal, "open kafka client", err)
	}
	defer client.Close()

	moved := 0
	for {
		recs, err := poll(ctx, client, idle)
		if err != nil || len(recs) == 0 {
			return moved, err
		}
		out := make([]*kgo.Record, 0, len(recs))
		for _, rec := range recs {
			out = append(out, replayed(rec))
		}
		if err := client.ProduceSync(ctx, out...).FirstErr(); err != nil {
			return moved, apperr.Wrap(apperr.CodeUnavailable, "produce replayed records", err)
		}
		if err := client.CommitRecords(ctx, recs...); err != nil {
			return moved, apperr.Wrap(apperr.CodeUnavailable, "commit dlq offsets", err)
		}
		moved += len(recs)
		log.InfoContext(ctx, "dlq records replayed", "count", len(recs), "total", moved)
	}
}

func poll(ctx context.Context, client *kgo.Client, idle time.Duration) ([]*kgo.Record, error) {
	pollCtx, cancel := context.WithTimeout(ctx, idle)
	defer cancel()
	fetches := client.PollFetches(pollCtx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, fe := range fetches.Errors() {
		if !errors.Is(fe.Err, context.DeadlineExceeded) {
			return nil, apperr.Wrap(apperr.CodeUnavailable, "poll dlq", fe.Err)
		}
	}
	return fetches.Records(), nil
}

func replayed(rec *kgo.Record) *kgo.Record {
	out := &kgo.Record{Key: rec.Key, Value: rec.Value}
	for _, h := range rec.Headers {
		if h.Key == "source_topic" {
			out.Topic = string(h.Value)
		}
		if !dlqHeaders[h.Key] {
			out.Headers = append(out.Headers, h)
		}
	}
	return out
}
