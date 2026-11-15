package env

import (
	"errors"
	"os"
	"time"
)

type Base struct {
	Mode     Mode
	LogLevel string
	// Instance names this process in logs, metrics and the X-Chimera-Instance header.
	Instance string
	// OTLPEndpoint is the collector URL for spans. Empty keeps them in the process.
	OTLPEndpoint string
	// MetricsAddr serves Prometheus metrics. Empty serves none.
	MetricsAddr string
}

type Postgres struct {
	DSN string
	// ReadDSN points at a hot standby. Empty means reads use DSN as well.
	ReadDSN           string
	StatementTimeout  time.Duration
	MinConns          int32
	MaxConns          int32
	MaxConnIdleTime   time.Duration
	MaxConnLifetime   time.Duration
	HealthCheckPeriod time.Duration
}

type Kafka struct {
	Brokers            []string
	OutboxPollInterval time.Duration
	OutboxBatchSize    int
	ProduceTimeout     time.Duration
	// A consumer retries a failing record this many times, then moves it to the DLQ.
	RetryMaxAttempts int
	RetryBackoff     time.Duration
	RetryMaxBackoff  time.Duration
}

type Health struct {
	Interval time.Duration
	Timeout  time.Duration
}

type GRPCServer struct {
	Addr             string
	ShutdownTimeout  time.Duration
	MaxConnectionAge time.Duration
}

func LoadBase() (Base, error) {
	mode, err := AppMode()
	if err != nil {
		return Base{}, err
	}
	instance := Optional("INSTANCE_NAME")
	if instance == "" {
		instance, err = os.Hostname()
		if err != nil {
			return Base{}, err
		}
	}
	return Base{
		Mode:         mode,
		LogLevel:     Optional("LOG_LEVEL"),
		Instance:     instance,
		OTLPEndpoint: Optional("OTEL_EXPORTER_OTLP_ENDPOINT"),
		MetricsAddr:  Optional("METRICS_ADDR"),
	}, nil
}

func LoadPostgres(health Health) (Postgres, error) {
	dsn, err := String("DATABASE_URL")
	if err != nil {
		return Postgres{}, err
	}
	minConns, err := Int32("POSTGRES_MIN_CONNS")
	if err != nil {
		return Postgres{}, err
	}
	maxConns, err := Int32("POSTGRES_MAX_CONNS")
	if err != nil {
		return Postgres{}, err
	}
	if minConns > maxConns {
		return Postgres{}, errors.New("POSTGRES_MIN_CONNS must be <= POSTGRES_MAX_CONNS")
	}
	idle, err := Duration("POSTGRES_MAX_CONN_IDLE")
	if err != nil {
		return Postgres{}, err
	}
	lifetime, err := Duration("POSTGRES_MAX_CONN_LIFETIME")
	if err != nil {
		return Postgres{}, err
	}
	statementTimeout, err := Duration("POSTGRES_STATEMENT_TIMEOUT")
	if err != nil {
		return Postgres{}, err
	}
	return Postgres{
		DSN:               dsn,
		ReadDSN:           Optional("DATABASE_READ_URL"),
		StatementTimeout:  statementTimeout,
		MinConns:          minConns,
		MaxConns:          maxConns,
		MaxConnIdleTime:   idle,
		MaxConnLifetime:   lifetime,
		HealthCheckPeriod: health.Interval,
	}, nil
}

// LoadKafka reads the settings of a service that both publishes through the
// outbox and consumes.
func LoadKafka() (Kafka, error) {
	k, err := LoadKafkaConsumer()
	if err != nil {
		return Kafka{}, err
	}
	poll, err := Duration("OUTBOX_POLL_INTERVAL")
	if err != nil {
		return Kafka{}, err
	}
	batch, err := Int("OUTBOX_BATCH_SIZE")
	if err != nil {
		return Kafka{}, err
	}
	k.OutboxPollInterval = poll
	k.OutboxBatchSize = batch
	return k, nil
}

// LoadKafkaConsumer reads the settings of a consumer. Its producer only writes to
// the DLQ, so it needs no outbox settings.
func LoadKafkaConsumer() (Kafka, error) {
	brokers, err := List("KAFKA_BROKERS")
	if err != nil {
		return Kafka{}, err
	}
	produce, err := Duration("KAFKA_PRODUCE_TIMEOUT")
	if err != nil {
		return Kafka{}, err
	}
	attempts, err := Int("KAFKA_RETRY_MAX_ATTEMPTS")
	if err != nil {
		return Kafka{}, err
	}
	if attempts < 1 {
		return Kafka{}, errors.New("KAFKA_RETRY_MAX_ATTEMPTS must be >= 1")
	}
	backoff, err := Duration("KAFKA_RETRY_BACKOFF")
	if err != nil {
		return Kafka{}, err
	}
	maxBackoff, err := Duration("KAFKA_RETRY_MAX_BACKOFF")
	if err != nil {
		return Kafka{}, err
	}
	return Kafka{
		Brokers:          brokers,
		ProduceTimeout:   produce,
		RetryMaxAttempts: attempts,
		RetryBackoff:     backoff,
		RetryMaxBackoff:  maxBackoff,
	}, nil
}

func LoadHealth() (Health, error) {
	interval, err := Duration("HEALTH_CHECK_INTERVAL")
	if err != nil {
		return Health{}, err
	}
	timeout, err := Duration("HEALTH_CHECK_TIMEOUT")
	if err != nil {
		return Health{}, err
	}
	if timeout >= interval {
		return Health{}, errors.New("HEALTH_CHECK_TIMEOUT must be < HEALTH_CHECK_INTERVAL")
	}
	return Health{Interval: interval, Timeout: timeout}, nil
}

// GRPCClient bounds each call to another service: one attempt gets Timeout, and a
// call is tried at most MaxAttempts times with exponential backoff from Backoff.
type GRPCClient struct {
	Timeout     time.Duration
	MaxAttempts int
	Backoff     time.Duration
}

func LoadGRPCClient() (GRPCClient, error) {
	timeout, err := Duration("GRPC_CLIENT_TIMEOUT")
	if err != nil {
		return GRPCClient{}, err
	}
	attempts, err := Int("GRPC_RETRY_MAX_ATTEMPTS")
	if err != nil {
		return GRPCClient{}, err
	}
	if attempts < 1 {
		return GRPCClient{}, errors.New("GRPC_RETRY_MAX_ATTEMPTS must be >= 1")
	}
	backoff, err := Duration("GRPC_RETRY_BACKOFF")
	if err != nil {
		return GRPCClient{}, err
	}
	return GRPCClient{Timeout: timeout, MaxAttempts: attempts, Backoff: backoff}, nil
}

func LoadGRPCServer() (GRPCServer, error) {
	addr, err := String("GRPC_ADDR")
	if err != nil {
		return GRPCServer{}, err
	}
	shutdown, err := Duration("SHUTDOWN_TIMEOUT")
	if err != nil {
		return GRPCServer{}, err
	}
	age, err := Duration("GRPC_MAX_CONNECTION_AGE")
	if err != nil {
		return GRPCServer{}, err
	}
	return GRPCServer{Addr: addr, ShutdownTimeout: shutdown, MaxConnectionAge: age}, nil
}
