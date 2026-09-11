package s3

import (
	"context"
	"net/url"

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
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, domain.Wrap(domain.CodeInternal, "open s3", err)
	}

	presign, err := url.Parse(cfg.PresignEndpoint)
	if err != nil {
		return nil, domain.Wrap(domain.CodeInternal, "parse s3 presign endpoint", err)
	}

	return &Store{client: client, cfg: cfg, presign: presign}, nil
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
