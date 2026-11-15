package s3

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"chimera/catalog/internal/domain"
)

type Config struct {
	// Endpoint is host:port the service dials, e.g. rustfs:9000.
	Endpoint string
	// PresignEndpoint is the URL browsers use for uploads and streams.
	PresignEndpoint string
	AccessKey       string
	SecretKey       string
	Bucket          string
	Region          string
	PresignTTL      time.Duration
	// Timeout bounds dialing and waiting for response headers of an API call.
	Timeout time.Duration
}

type Store struct {
	client  *minio.Client
	cfg     Config
	presign *url.URL
}

func New(cfg Config) (*Store, error) {
	presign, err := url.Parse(cfg.PresignEndpoint)
	if err != nil || presign.Host == "" {
		return nil, domain.Wrap(domain.CodeInternal, "parse s3 presign endpoint", err)
	}

	// Sign for the host the browser will send. API calls still dial the
	// internal endpoint, otherwise the signature does not match.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = cfg.Timeout
	public := dialAddr(presign)
	if public != cfg.Endpoint {
		dialer := &net.Dialer{Timeout: cfg.Timeout, KeepAlive: 30 * time.Second}
		internal := cfg.Endpoint
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr == public {
				addr = internal
			}
			return dialer.DialContext(ctx, network, addr)
		}
	}

	client, err := minio.New(presign.Host, &minio.Options{
		Creds:     credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:    presign.Scheme == "https",
		Region:    cfg.Region,
		Transport: transport,
	})
	if err != nil {
		return nil, domain.Wrap(domain.CodeInternal, "open s3", err)
	}

	return &Store{client: client, cfg: cfg, presign: presign}, nil
}

// dialAddr is the host:port the HTTP transport dials for u. A presign URL
// behind an ingress often has no port, while the dialer always sees one.
func dialAddr(u *url.URL) string {
	if u.Port() != "" {
		return u.Host
	}
	if u.Scheme == "https" {
		return net.JoinHostPort(u.Hostname(), "443")
	}
	return net.JoinHostPort(u.Hostname(), "80")
}

func (s *Store) Ping(ctx context.Context) error {
	_, err := s.client.BucketExists(ctx, s.cfg.Bucket)
	if err != nil {
		return domain.Wrap(domain.CodeInternal, "ping s3", err)
	}
	return nil
}

func (s *Store) EnsureBucket(ctx context.Context) error {
	ok, err := s.client.BucketExists(ctx, s.cfg.Bucket)
	if err != nil {
		return domain.Wrap(domain.CodeInternal, "check s3 bucket", err)
	}
	if ok {
		return nil
	}
	err = s.client.MakeBucket(ctx, s.cfg.Bucket, minio.MakeBucketOptions{Region: s.cfg.Region})
	if err == nil || bucketCreatedElsewhere(err) {
		return nil
	}
	return domain.Wrap(domain.CodeInternal, "create s3 bucket", err)
}

// bucketCreatedElsewhere reports that another Catalog instance won the race to
// create the bucket between BucketExists and MakeBucket.
func bucketCreatedElsewhere(err error) bool {
	switch minio.ToErrorResponse(err).Code {
	case "BucketAlreadyOwnedByYou", "BucketAlreadyExists":
		return true
	}
	return false
}

func (s *Store) PresignPut(ctx context.Context, key string) (string, error) {
	u, err := s.client.PresignedPutObject(ctx, s.cfg.Bucket, key, s.cfg.PresignTTL)
	return s.presignURL(u, err, "presign upload")
}

func (s *Store) PresignGet(ctx context.Context, key string) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.cfg.Bucket, key, s.cfg.PresignTTL, nil)
	return s.presignURL(u, err, "presign stream")
}

func (s *Store) presignURL(u *url.URL, err error, fallback string) (string, error) {
	if err != nil {
		return "", domain.Wrap(domain.CodeInternal, fallback, err)
	}
	u.Scheme = s.presign.Scheme
	u.Host = s.presign.Host
	return u.String(), nil
}

func (s *Store) Stat(ctx context.Context, key string) (int64, error) {
	info, err := s.client.StatObject(ctx, s.cfg.Bucket, key, minio.StatObjectOptions{})
	if err != nil {
		code := minio.ToErrorResponse(err).Code
		if code == "NoSuchKey" || code == "NotFound" {
			return 0, domain.NotFound("upload not found")
		}
		return 0, domain.Wrap(domain.CodeInternal, "stat upload", err)
	}
	return info.Size, nil
}

// Remove deletes the object. A missing object is not an error, so a
// redelivered TrackDeleted event is harmless.
func (s *Store) Remove(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.cfg.Bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return domain.Wrap(domain.CodeInternal, "remove object", err)
	}
	return nil
}
