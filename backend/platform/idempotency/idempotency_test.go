package idempotency

import (
	"context"
	"strings"
	"testing"
)

func TestWithRejectsAKeyPastTheLimit(t *testing.T) {
	_, err := With(context.Background(), strings.Repeat("k", MaxKeyLength+1))
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestWithKeepsAnEmptyKeyOutOfTheContext(t *testing.T) {
	ctx, err := With(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if From(ctx) != "" {
		t.Fatalf("key = %q", From(ctx))
	}
}
