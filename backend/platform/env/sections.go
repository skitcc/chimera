package env

import (
	"errors"
	"time"
)

type Base struct {
	Mode     Mode
	LogLevel string
}

type Postgres struct {
	DSN               string
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
}

type Health struct {
	Interval time.Duration
	Timeout  time.Duration
}

type GRPCServer struct {
	Addr            string
	ShutdownTimeout time.Duration
}

func LoadBase() (Base, error) {
	mode, err := AppMode()
	if err != nil {
		return Base{}, err
	}
	return Base{Mode: mode, LogLevel: Optional("LOG_LEVEL")}, nil
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
	return Postgres{
		DSN:               dsn,
		MinConns:          minConns,
		MaxConns:          maxConns,
		MaxConnIdleTime:   idle,
		MaxConnLifetime:   lifetime,
		HealthCheckPeriod: health.Interval,
	}, nil
}

func LoadKafka() (Kafka, error) {
	brokers, err := List("KAFKA_BROKERS")
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
	return Kafka{Brokers: brokers, OutboxPollInterval: poll, OutboxBatchSize: batch}, nil
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

func LoadGRPCServer() (GRPCServer, error) {
	addr, err := String("GRPC_ADDR")
	if err != nil {
		return GRPCServer{}, err
	}
	shutdown, err := Duration("SHUTDOWN_TIMEOUT")
	if err != nil {
		return GRPCServer{}, err
	}
	return GRPCServer{Addr: addr, ShutdownTimeout: shutdown}, nil
}
