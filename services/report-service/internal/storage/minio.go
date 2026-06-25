package storage

import (
    "bytes"
    "context"
    "fmt"
    "time"

    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
    "github.com/rs/zerolog"
)

const (
    defaultExpiry = 7 * 24 * time.Hour // 7-day presigned URL
)

// Config holds MinIO/S3 connection settings.
type Config struct {
    Endpoint        string
    AccessKeyID     string
    SecretAccessKey string
    BucketName      string
    UseSSL          bool
    Region          string
}

// MinIOStorage stores report artifacts in MinIO/S3.
type MinIOStorage struct {
    client     *minio.Client
    bucketName string
    logger     zerolog.Logger
}

// New creates a MinIOStorage.
func New(cfg Config, logger zerolog.Logger) (*MinIOStorage, error) {
    client, err := minio.New(cfg.Endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
        Secure: cfg.UseSSL,
    })
    if err != nil {
        return nil, fmt.Errorf("minio init: %w", err)
    }

    return &MinIOStorage{
        client:     client,
        bucketName: cfg.BucketName,
        logger:     logger,
    }, nil
}

// EnsureBucket creates the bucket if it doesn't exist.
func (s *MinIOStorage) EnsureBucket(ctx context.Context) error {
    exists, err := s.client.BucketExists(ctx, s.bucketName)
    if err != nil {
        return fmt.Errorf("check bucket: %w", err)
    }
    if exists {
        return nil
    }

    if err := s.client.MakeBucket(ctx, s.bucketName, minio.MakeBucketOptions{}); err != nil {
        return fmt.Errorf("create bucket: %w", err)
    }

    s.logger.Info().Str("bucket", s.bucketName).Msg("created MinIO bucket")
    return nil
}

// Upload stores a report artifact and returns its object key.
func (s *MinIOStorage) Upload(ctx context.Context, reportID, format string, data []byte) (string, error) {
    objectKey := fmt.Sprintf("reports/%s/%s.%s",
        time.Now().UTC().Format("2006/01/02"),
        reportID,
        formatExtension(format),
    )

    contentType := formatContentType(format)

    _, err := s.client.PutObject(
        ctx,
        s.bucketName,
        objectKey,
        bytes.NewReader(data),
        int64(len(data)),
        minio.PutObjectOptions{ContentType: contentType},
    )
    if err != nil {
        return "", fmt.Errorf("upload report %s: %w", objectKey, err)
    }

    s.logger.Info().
        Str("key", objectKey).
        Int("bytes", len(data)).
        Str("format", format).
        Msg("report uploaded to MinIO")

    return objectKey, nil
}

// PresignedURL generates a time-limited download URL for a report artifact.
func (s *MinIOStorage) PresignedURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
    if expiry == 0 {
        expiry = defaultExpiry
    }

    url, err := s.client.PresignedGetObject(ctx, s.bucketName, objectKey, expiry, nil)
    if err != nil {
        return "", fmt.Errorf("presign url: %w", err)
    }

    return url.String(), nil
}

// Delete removes a report artifact.
func (s *MinIOStorage) Delete(ctx context.Context, objectKey string) error {
    return s.client.RemoveObject(ctx, s.bucketName, objectKey, minio.RemoveObjectOptions{})
}

// formatExtension returns the file extension for a format.
func formatExtension(format string) string {
    switch format {
    case "html":
        return "html"
    case "pdf":
        return "pdf"
    case "csv":
        return "csv"
    case "excel":
        return "xlsx"
    case "json":
        return "json"
    default:
        return "bin"
    }
}

// formatContentType returns the MIME type for a format.
func formatContentType(format string) string {
    switch format {
    case "html":
        return "text/html"
    case "pdf":
        return "application/pdf"
    case "csv":
        return "text/csv"
    case "excel":
        return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
    case "json":
        return "application/json"
    default:
        return "application/octet-stream"
    }
}
