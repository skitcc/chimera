module chimera/platform

go 1.26.0

require (
	chimera/contracts v0.0.0
	github.com/golang-migrate/migrate/v4 v4.20.1
	github.com/jackc/pgx/v5 v5.11.0
	github.com/twmb/franz-go v1.22.3
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260817212433-ac3dfec99bb1
	google.golang.org/grpc v1.85.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/jackc/pgerrcode v0.0.0-20220416144525-469b46aa5efa // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	github.com/klauspost/compress v1.20.3 // indirect
	github.com/pierrec/lz4/v4 v4.1.33 // indirect
	github.com/stretchr/testify v1.12.1 // indirect
	github.com/twmb/franz-go/pkg/kmsg v1.15.1 // indirect
	golang.org/x/net v0.60.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
)

replace chimera/contracts => ../contracts
