// Package outbox writes domain events in the caller's transaction and relays them to Kafka.
//
// Every producing service has this table in its own database:
//
//	outbox(id bigserial, topic, record_key, event_type, event_id uuid, payload bytea, created_at, sent_at)
package outbox

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "chimera/contracts/gen/events/v1"
	"chimera/platform/apperr"
	"chimera/platform/kafka"
	"chimera/platform/pg"
)

// NewEvent starts an envelope with a fresh event id. The caller sets the payload.
func NewEvent(producer, eventType string) *eventsv1.Envelope {
	return &eventsv1.Envelope{
		EventId:    newUUID(),
		Type:       eventType,
		OccurredAt: timestamppb.Now(),
		Producer:   producer,
	}
}

// Write stores event in the outbox within db, which must be the transaction
// that changes the entity, so the row and the event commit together.
func Write(ctx context.Context, db pg.DB, topic, key string, event *eventsv1.Envelope) error {
	payload, err := proto.Marshal(event)
	if err != nil {
		return apperr.Wrap(apperr.CodeInternal, "encode event", err)
	}
	if _, err := db.Exec(ctx,
		`INSERT INTO outbox (topic, record_key, event_type, event_id, payload)
		 VALUES ($1, $2, $3, $4::uuid, $5)`,
		topic, key, event.GetType(), event.GetEventId(), payload,
	); err != nil {
		return apperr.Wrap(apperr.CodeInternal, "write outbox", err)
	}
	return nil
}

type Publisher interface {
	Publish(ctx context.Context, recs []kafka.Record) []error
}

type Logger interface {
	ErrorContext(ctx context.Context, msg string, args ...any)
}

type Relay struct {
	db        pg.TxDB
	publisher Publisher
	log       Logger
	source    string
	interval  time.Duration
	batch     int
	retention time.Duration
}

func NewRelay(db pg.TxDB, publisher Publisher, log Logger, source string, interval time.Duration, batch int) *Relay {
	return &Relay{
		db:        db,
		publisher: publisher,
		log:       log,
		source:    source,
		interval:  interval,
		batch:     batch,
		retention: 24 * time.Hour,
	}
}

func (r *Relay) Run(ctx context.Context) {
	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		for {
			sent, err := r.Flush(ctx)
			if err != nil {
				r.log.ErrorContext(ctx, "outbox relay", "error", err)
				break
			}
			if sent < r.batch {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

type row struct {
	id        int64
	topic     string
	key       string
	eventType string
	eventID   string
	payload   []byte
}

// Flush publishes one batch of unsent rows in id order and marks the published
// prefix as sent. Rows after the first failure stay for the next round, so
// events of one key never overtake each other.
func (r *Relay) Flush(ctx context.Context) (int, error) {
	sent := 0
	err := pg.InTx(ctx, r.db, func(tx pgx.Tx) error {
		rows, err := r.lock(ctx, tx)
		if err != nil || len(rows) == 0 {
			return err
		}
		recs := make([]kafka.Record, 0, len(rows))
		for _, row := range rows {
			recs = append(recs, kafka.Record{
				Topic: row.topic,
				Key:   row.key,
				Value: row.payload,
				Headers: map[string]string{
					kafka.HeaderID:          row.eventID,
					kafka.HeaderType:        row.eventType,
					kafka.HeaderSource:      r.source,
					kafka.HeaderContentType: kafka.ContentType,
				},
			})
		}
		errs := r.publisher.Publish(ctx, recs)
		ids := make([]int64, 0, len(rows))
		var publishErr error
		for i, row := range rows {
			if errs[i] != nil {
				publishErr = errs[i]
				break
			}
			ids = append(ids, row.id)
		}
		if len(ids) > 0 {
			if _, err := tx.Exec(ctx, `UPDATE outbox SET sent_at = now() WHERE id = ANY($1)`, ids); err != nil {
				return apperr.Wrap(apperr.CodeInternal, "mark outbox sent", err)
			}
		}
		if _, err := tx.Exec(ctx,
			`DELETE FROM outbox WHERE sent_at < now() - $1::bigint * interval '1 second'`,
			int64(r.retention.Seconds())); err != nil {
			return apperr.Wrap(apperr.CodeInternal, "trim outbox", err)
		}
		sent = len(ids)
		if publishErr != nil {
			r.log.ErrorContext(ctx, "outbox publish", "sent", len(ids), "pending", len(rows)-len(ids), "error", publishErr)
		}
		return nil
	})
	return sent, err
}

func (r *Relay) lock(ctx context.Context, tx pgx.Tx) ([]row, error) {
	q, err := tx.Query(ctx,
		`SELECT id, topic, record_key, event_type, event_id::text, payload
		 FROM outbox
		 WHERE sent_at IS NULL
		 ORDER BY id
		 LIMIT $1
		 FOR UPDATE SKIP LOCKED`, r.batch)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInternal, "read outbox", err)
	}
	defer q.Close()
	var rows []row
	for q.Next() {
		var x row
		if err := q.Scan(&x.id, &x.topic, &x.key, &x.eventType, &x.eventID, &x.payload); err != nil {
			return nil, apperr.Wrap(apperr.CodeInternal, "scan outbox", err)
		}
		rows = append(rows, x)
	}
	if err := q.Err(); err != nil {
		return nil, apperr.Wrap(apperr.CodeInternal, "read outbox", err)
	}
	return rows, nil
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
