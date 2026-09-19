# Backend testing

Backend tests run in Docker. The host needs Docker with Compose and `make`; it
does not need Go, gobco, Allure, PostgreSQL, or any test package installed.

From `backend/`:

```bash
make test-image
make test
make test-shuffle
make test-offline
make test-integration
make coverage
make gobco-condition
make gobco-branch
make allure-report
make test-all
```

Copy `.env.example` to the repository-root `.env` before integration tests.
Test PostgreSQL credentials are test-only values from that file. The database
runs as `postgres-test`, stores its data on `tmpfs`, is exposed only to the
Compose network, and is removed after the command.

## Targets and options

- `test-image` builds `Dockerfile.test`. All modules from `go.mod`, including
  test-only libraries (allure-go, pgxmock), and gobco are baked into the image.
  The production `Dockerfile` is separate: it drops `*_test.go` and
  `internal/testkit` before `go build`, so test libraries are neither
  downloaded nor linked into the API binary.

## Test-only dependencies

Go has no `devDependencies`: a single `go.mod` lists every module the module's
packages *and their tests* import. The split is kept explicit instead:

- `go.mod` has a dedicated `require` block for test-only modules
  (`go mod tidy` preserves it). Add new test libraries there.
- The production `Dockerfile` never runs `go mod download`; it deletes
  `*_test.go` and `internal/testkit`, builds `./cmd/api`, and then fails the
  build if `go version -m /api` reports a test-only module. Importing a test
  library from production code therefore breaks `docker compose build`.

| Module | Used from | Purpose |
| --- | --- | --- |
| `github.com/allure-framework/allure-go/commons` | `internal/testkit/report.go` | Allure adapter: wraps `t.Run`, emits result JSON with suites, AAA step, and test-technique labels |
| `github.com/pashagolub/pgxmock/v5` | `internal/infra/adapters/postgres/*_test.go` | In-memory stand-in for `pgxpool.Pool`: expected SQL, canned rows/errors, `ExpectationsWereMet` |

`gobco` is a binary installed in `Dockerfile.test`, not a `go.mod` dependency.
- `test` runs `go test` against `TEST_PACKAGES` (default `./...`).
- `test-shuffle` uses `-shuffle`. Its default `SHUFFLE_SEED=on` prints a random
  seed. Reproduce a failure with, for example,
  `make test-shuffle SHUFFLE_SEED=1700000000000000000`.
- `test-offline` requires an existing image from `make test-image`, forbids
  pulls, and runs that exact image with `--network none`. A test failure
  usually means a dependency or runtime network assumption was not captured
  by the image.
- `test-integration` starts only the ephemeral database and runs tests with
  the `integration` build tag. Override `INTEGRATION_TAGS` or
  `INTEGRATION_PACKAGES` if the suite adopts another convention.
- `test-all` runs the normal, shuffled, and offline suites, both coverage
  mechanisms, and integration tests. It does not build the Allure HTML site,
  because report rendering should not decide whether the suite passes.

Useful overrides:

```bash
make test TEST_PACKAGES=./internal/usecase/...
make test GO_TEST_P=2 GO_TEST_PARALLEL=8
make test-image TEST_IMAGE=registry.example/chimera-test:dev
```

`GO_TEST_P` controls how many package test binaries Go may build/run
concurrently (`go test -p`). `GO_TEST_PARALLEL` controls the maximum number of
tests within each test binary that may execute after calling `t.Parallel`
(`go test -parallel`). Each package normally runs in a separate process, so
package globals are not shared across packages. Keep database fixtures
independent before increasing either value.

## Test design

Use Arrange, Act, Assert (AAA) so setup, the single behavior under test, and
observations remain distinct. Prefer assertions against observable behavior
instead of implementation details.

Use the classic testing style by default: instantiate real value objects and
small collaborators, replacing only slow or nondeterministic boundaries such
as databases, clocks, object storage, or token generators. A London-style
interaction test, where every collaborator is mocked, is appropriate when the
interaction itself is the contract, but it tends to couple tests to call
order and internal orchestration. Interfaces are mocked on the consumer side.

`TrackService.Like` is intentionally duplicated:

- `TestTrackServiceLikeClassic` uses stateful fakes and asserts that the like
  exists after the operation. It remains stable if internal calls are
  reordered.
- `TestTrackServiceLikeLondon` uses a repository mock and a like-repository
  spy. It asserts exact IDs and verifies that invalid or not-ready tracks do
  not reach `Add`.

Keep fixed fixtures small and explicit. Use:

- a **Builder** when a test needs readable per-field variation from valid
  defaults;
- an **Object Mother** for a few named, stable business scenarios;
- helper functions only when they improve intent and still surface important
  values at the call site.

Do not share mutable fixtures between tests. Integration tests should create
their own rows and clean them transactionally or with unique identifiers.

Choose test data systematically:

- equivalence partitions for representative valid and invalid classes;
- boundary values immediately below, at, and above a limit;
- decision tables for combinations of business rules;
- pairwise combinations when exhaustive combinations are too large;
- malformed, empty, duplicate, unauthorized, and dependency-failure cases;
- deterministic values for ordinary tests and seeded generation/fuzzing for
  broader exploration.

## Coverage

`make coverage` writes Go statement coverage to:

- `coverage.out`
- `coverage.html`

Go's built-in coverage measures instrumented statements, not branch or
condition coverage. A high percentage does not prove that both outcomes of a
decision or each operand of a compound boolean expression were exercised.

`make gobco-condition` and `make gobco-branch` write
`condition-coverage.txt` and `branch-coverage.txt`. Gobco complements rather
than replaces `go test -cover`: it detects syntactically recognizable boolean conditions and
branches, but does not report completely unused functions that contain no
condition, does not cover `select` statements, and cannot infer semantic
input partitions. Review both reports and the tests themselves.

## Allure

`make allure-results` clears `allure-results` and runs all scenarios through
the official `allure-framework/allure-go` adapter. Every reported case has an
AAA description and a test-technique label selected from boundary values,
equivalence classes, state transitions, decision tables, and error guessing.
`make allure-report` then uses a containerized Allure CLI to generate
`allure-report/`. Do not open `index.html` as a local file: the UI loads JSON
over HTTP, so the browser or a file preview often shows a blank page or HTTP
500. Serve it instead:

```bash
make allure-open
```

That pulls `nginx:1.27-alpine` on the first run if needed and serves
`http://127.0.0.1:5252`. Override the port with `ALLURE_PORT`. Neither Java
nor Allure is installed on the host.
