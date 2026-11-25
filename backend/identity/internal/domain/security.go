package domain

import (
	"context"
	"time"
)

// SecurityAccount and its challenges are locked on the primary for the whole operation.
// A rejected attempt is committed; infrastructure errors roll the transaction back.
type SecurityAccount struct {
	User         User
	PasswordHash string
	Failures     int
	LockedUntil  time.Time
	Challenges   map[string]Challenge
}
type Challenge struct {
	ID                string
	Digest            []byte
	Version           int64
	ExpiresAt, SentAt time.Time
}
type SecurityLookup struct{ Email, UserID, ChallengeID string }
type SecurityStore interface {
	WithAccount(context.Context, SecurityLookup, func(*SecurityAccount) error) error
	Version(context.Context, string) (int64, error)
}
