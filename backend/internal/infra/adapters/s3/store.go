package s3

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"chimera/internal/config"
	"chimera/internal/domain"
)

type Store struct {
	client  *minio.Client
	cfg     config.S3
	presign *url.URL
}

func New(cfg config.S3) (*Store, error) {
	presign, err := url.Parse(cfg.PresignEndpoint)
	if err != nil || presign.Host == "" {
		return nil, domain.Wrap(domain.CodeInternal, "parse s3 presign endpoint", err)
	}

	// Sign for the host the browser will send. API calls still dial the
	// internal endpoint, otherwise the signature does not match.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if presign.Host != cfg.Endpoint {
		dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
		internal := cfg.Endpoint
		public := presign.Host
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
	if err := s.client.MakeBucket(ctx, s.cfg.Bucket, minio.MakeBucketOptions{Region: s.cfg.Region}); err != nil {
		return domain.Wrap(domain.CodeInternal, "create s3 bucket", err)
	}
	return nil
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
