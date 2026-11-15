module chimera/testkit

go 1.26.0

require (
	chimera/platform v0.0.0
	github.com/jackc/pgx/v5 v5.11.0
)

require (
	github.com/stretchr/testify v1.12.1 // indirect
	go.opentelemetry.io/otel/metric v1.45.0 // indirect
	go.opentelemetry.io/otel/trace v1.45.0 // indirect
)

require (
	github.com/allure-framework/allure-go/commons v1.3.1
	github.com/golang-migrate/migrate/v4 v4.20.1 // indirect
	github.com/jackc/pgerrcode v0.0.0-20220416144525-469b46aa5efa // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/text v0.42.0 // indirect
)

replace (
	chimera/contracts => ../contracts
	chimera/platform => ../platform
)
