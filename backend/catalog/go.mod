module chimera/catalog

go 1.26.0

require (
	chimera/contracts v0.0.0
	chimera/platform v0.0.0
	chimera/testkit v0.0.0
	github.com/jackc/pgx/v5 v5.11.0
	github.com/minio/minio-go/v7 v7.3.0
	github.com/pashagolub/pgxmock/v5 v5.2.0
)

require (
	github.com/allure-framework/allure-go/commons v1.3.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/golang-migrate/migrate/v4 v4.20.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgerrcode v0.0.0-20220416144525-469b46aa5efa // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	github.com/klauspost/compress v1.20.3 // indirect
	github.com/klauspost/cpuid/v2 v2.4.0 // indirect
	github.com/klauspost/crc32 v1.3.0 // indirect
	github.com/minio/crc64nvme v1.1.1 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.33 // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/tinylib/msgp v1.6.4 // indirect
	github.com/twmb/franz-go v1.22.3 // indirect
	github.com/twmb/franz-go/pkg/kmsg v1.15.1 // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.57.0 // indirect
	golang.org/x/net v0.60.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260817212433-ac3dfec99bb1 // indirect
	google.golang.org/grpc v1.85.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	gopkg.in/ini.v1 v1.67.3 // indirect
)

replace (
	chimera/contracts => ../contracts
	chimera/platform => ../platform
	chimera/testkit => ../testkit
)
