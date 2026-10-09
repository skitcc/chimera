package postgres

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

type stubDB struct {
	execFn     func(context.Context, string, ...any) (pgconn.CommandTag, error)
	queryFn    func(context.Context, string, ...any) (pgx.Rows, error)
	queryRowFn func(context.Context, string, ...any) pgx.Row
}

func runSpec(t *testing.T, component string, s testkit.Spec, body func(*testing.T, testkit.Report)) {
	t.Helper()
	s.Layer = testkit.LayerData
	s.Component = component
	if s.Kind == "" {
		s.Kind = testkit.KindStub
	}
	testkit.RunSpec(t, s, body)
}

func (s stubDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if s.execFn == nil {
		panic("unexpected Exec call")
	}
	return s.execFn(ctx, sql, args...)
}

func (s stubDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if s.queryFn == nil {
		panic("unexpected Query call")
	}
	return s.queryFn(ctx, sql, args...)
}

func (s stubDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if s.queryRowFn == nil {
		panic("unexpected QueryRow call")
	}
	return s.queryRowFn(ctx, sql, args...)
}

type stubRow struct {
	values []any
	err    error
}

func (r stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assign(dest, r.values)
}

type stubRows struct {
	rows       [][]any
	index      int
	err        error
	scanErrAt  int
	closed     bool
	commandTag pgconn.CommandTag
}

func newStubRows(rows ...[]any) *stubRows {
	return &stubRows{rows: rows, index: -1, scanErrAt: -1}
}

func (r *stubRows) Close() {
	r.closed = true
}

func (r *stubRows) Err() error {
	return r.err
}

func (r *stubRows) CommandTag() pgconn.CommandTag {
	return r.commandTag
}

func (r *stubRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *stubRows) Next() bool {
	r.index++
	if r.index >= len(r.rows) {
		r.closed = true
		return false
	}
	return true
}

func (r *stubRows) Scan(dest ...any) error {
	if r.index == r.scanErrAt {
		return fmt.Errorf("scan failed")
	}
	if r.index < 0 || r.index >= len(r.rows) {
		return fmt.Errorf("Scan called without a current row")
	}
	return assign(dest, r.rows[r.index])
}

func (r *stubRows) Values() ([]any, error) {
	if r.index < 0 || r.index >= len(r.rows) {
		return nil, fmt.Errorf("Values called without a current row")
	}
	return r.rows[r.index], nil
}

func (r *stubRows) RawValues() [][]byte {
	return nil
}

func (r *stubRows) Conn() *pgx.Conn {
	return nil
}

func (r *stubRows) TypeMap() *pgtype.Map {
	return nil
}

func assign(dest, values []any) error {
	if len(dest) != len(values) {
		return fmt.Errorf("destination count %d does not match value count %d", len(dest), len(values))
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			value, ok := values[i].(string)
			if !ok {
				return fmt.Errorf("value %d is %T, want string", i, values[i])
			}
			*d = value
		case *int64:
			value, ok := values[i].(int64)
			if !ok {
				return fmt.Errorf("value %d is %T, want int64", i, values[i])
			}
			*d = value
		default:
			return fmt.Errorf("unsupported destination %T", dest[i])
		}
	}
	return nil
}

func assertEqual[T any](t *testing.T, got, want T) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func assertDomainError(t *testing.T, err error, code domain.Code, message string) {
	t.Helper()
	app, ok := domain.As(err)
	if !ok {
		t.Fatalf("got error %v, want domain error", err)
	}
	if app.Code != code || app.Message != message {
		t.Fatalf("got domain error (%s, %q), want (%s, %q)", app.Code, app.Message, code, message)
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertWraps(t *testing.T, err, cause error) {
	t.Helper()
	if !errors.Is(err, cause) {
		t.Fatalf("error %v does not wrap %v", err, cause)
	}
}

func assertRowsClosed(t *testing.T, rows *stubRows) {
	t.Helper()
	if !rows.closed {
		t.Fatal("rows were not closed")
	}
}

func userValues(u domain.User) []any {
	return []any{u.ID, u.Email, u.Name}
}

func trackValues(track domain.Track) []any {
	return []any{
		track.ID,
		track.UserID,
		track.Title,
		track.Artist,
		track.ObjectKey,
		track.SizeBytes,
		string(track.Status),
	}
}
