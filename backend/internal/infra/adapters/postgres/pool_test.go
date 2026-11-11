package postgres

import (
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

func TestMapError(t *testing.T) {
	runSpec(t, "Postgres", testkit.Spec{
		ID: "DA-PG-MAP-01", Method: "mapError",
		Title:     "insufficient privilege becomes a read only forbidden error",
		Given:     "a postgres error with SQLSTATE 42501",
		When:      "mapError is called",
		Then:      "forbidden: read only",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"sqlstate": "42501"},
	}, func(t *testing.T, r testkit.Report) {
		var err error
		r.Act(func(t *testing.T) {
			err = mapError(&pgconn.PgError{Code: "42501"}, "create user")
		})
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeForbidden, "read only")
		})
	})

	runSpec(t, "Postgres", testkit.Spec{
		ID: "DA-PG-MAP-02", Method: "mapError",
		Title:     "a read only transaction becomes a read only forbidden error",
		Given:     "a postgres error with SQLSTATE 25006",
		When:      "mapError is called",
		Then:      "forbidden: read only",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"sqlstate": "25006"},
	}, func(t *testing.T, r testkit.Report) {
		var err error
		r.Act(func(t *testing.T) {
			err = mapError(&pgconn.PgError{Code: "25006"}, "create user")
		})
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeForbidden, "read only")
		})
	})
}
