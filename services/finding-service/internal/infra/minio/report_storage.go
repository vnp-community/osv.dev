package minio

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type ReportStorage struct {
	client *minio.Client
	bucket string
}

func NewReportStorage(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*ReportStorage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio.New: %w", err)
	}
	// Ensure bucket exists
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket %q: %w", bucket, err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create bucket %q: %w", bucket, err)
		}
	}
	return &ReportStorage{client: client, bucket: bucket}, nil
}

func (s *ReportStorage) Upload(ctx context.Context, key string, r io.Reader) (int64, error) {
	// For MinIO we can use PutObject with -1 size and application/octet-stream if size is unknown
	info, err := s.client.PutObject(ctx, s.bucket, key,
		r, -1,
		minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		return 0, err
	}
	return info.Size, nil
}

func (s *ReportStorage) PresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("presigned URL for %q: %w", key, err)
	}
	return u.String(), nil
}

func (s *ReportStorage) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}
