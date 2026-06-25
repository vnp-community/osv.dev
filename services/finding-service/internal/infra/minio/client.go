package minio

import (
    "context"
    "time"

    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
    client     *minio.Client
    bucketName string
}

func NewClient(endpoint, accessKey, secretKey, bucketName string, useSSL bool) (*Client, error) {
    mc, err := minio.New(endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
        Secure: useSSL,
    })
    if err != nil {
        return nil, err
    }
    return &Client{client: mc, bucketName: bucketName}, nil
}

// PresignGetObject generates a presigned download URL for object
func (c *Client) PresignGetObject(ctx context.Context, objectPath string, expiry time.Duration) (string, error) {
    u, err := c.client.PresignedGetObject(ctx, c.bucketName, objectPath, expiry, nil)
    if err != nil {
        return "", err
    }
    return u.String(), nil
}
