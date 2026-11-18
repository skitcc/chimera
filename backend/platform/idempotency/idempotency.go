// Package idempotency makes a create call safe to repeat.
//
// The caller sends a key with the call. The service claims the key in the same
// transaction that creates the resource and stores the resource id with it. A
// repeat with the same key and the same request returns that resource; the same
// key with another request is a conflict. Every service that creates resources
// has the idempotency_keys table in its own database.
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"

	"github.com/jackc/pgx/v5"

	"chimera/platform/apperr"
)

// MaxKeyLength bounds a client-chosen key.
const MaxKeyLength = 128

type keyCtx struct{}

// With attaches key to ctx. An empty key leaves ctx unchanged.
func With(ctx context.Context, key string) (context.Context, error) {
	if key == "" {
		return ctx, nil
	}
	if len(key) > MaxKeyLength {
		return ctx, apperr.Invalid("idempotency key is too long")
	}
	return context.WithValue(ctx, keyCtx{}, key), nil
}

func From(ctx context.Context) string {
	key, _ := ctx.Value(keyCtx{}).(string)
	return key
}

// Claim records the key of ctx for operation and actor inside tx. It returns the
// id of the resource an earlier call with this key created, or "" when this call
// comes first or carries no key. request lists the fields that must match.
// A concurrent call with the same key waits on the primary key until this tx ends.
func Claim(ctx context.Context, tx pgx.Tx, operation, actor string, request ...string) (string, error) {
	key := From(ctx)
	if key == "" {
		return "", nil
	}
	hash := fingerprint(operation, request)
	tag, err := tx.Exec(ctx,
		`INSERT INTO idempotency_keys (operation, actor_id, key, request_hash)
		 VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
		operation, actor, key, hash)
	if err != nil {
		return "", apperr.Wrap(apperr.CodeInternal, "claim idempotency key", err)
	}
	if tag.RowsAffected() == 1 {
		return "", nil
	}

	var stored []byte
	var resourceID string
	if err := tx.QueryRow(ctx,
		`SELECT request_hash, COALESCE(resource_id, '') FROM idempotency_keys
		 WHERE operation = $1 AND actor_id = $2 AND key = $3`,
		operation, actor, key,
	).Scan(&stored, &resourceID); err != nil {
		return "", apperr.Wrap(apperr.CodeInternal, "read idempotency key", err)
	}
	if !bytes.Equal(stored, hash) {
		return "", apperr.Conflict("idempotency key reused with another request")
	}
	if resourceID == "" {
		return "", apperr.Conflict("idempotency key has no result")
	}
	return resourceID, nil
}

// Complete stores the id of the resource the claimed call created.
func Complete(ctx context.Context, tx pgx.Tx, operation, actor, resourceID string) error {
	key := From(ctx)
	if key == "" {
		return nil
	}
	if _, err := tx.Exec(ctx,
		`UPDATE idempotency_keys SET resource_id = $4
		 WHERE operation = $1 AND actor_id = $2 AND key = $3`,
		operation, actor, key, resourceID,
	); err != nil {
		return apperr.Wrap(apperr.CodeInternal, "complete idempotency key", err)
	}
	return nil
}

func fingerprint(operation string, request []string) []byte {
	h := sha256.New()
	h.Write([]byte(operation))
	for _, field := range request {
		h.Write([]byte{0})
		h.Write([]byte(field))
	}
	return h.Sum(nil)
}
