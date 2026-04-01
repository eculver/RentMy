// Package s3 provides an S3-compatible storage client for file operations.
package s3

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client wraps the AWS S3 client with convenience methods.
type Client struct {
	s3 *s3.Client
}

// Config holds S3 connection parameters.
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Region    string
}

// New creates a new S3 client configured for the given endpoint.
func New(ctx context.Context, cfg Config) (*Client, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey, cfg.SecretKey, "",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true // required for MinIO
	})

	slog.Info("s3 client created", "endpoint", cfg.Endpoint)
	return &Client{s3: client}, nil
}

// EnsureBucket creates a bucket if it doesn't exist.
func (c *Client) EnsureBucket(ctx context.Context, bucket string) error {
	_, err := c.s3.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err == nil {
		return nil // bucket exists
	}

	_, err = c.s3.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return fmt.Errorf("create bucket %s: %w", bucket, err)
	}

	slog.Info("created bucket", "bucket", bucket)
	return nil
}

// Upload stores an object in the specified bucket.
func (c *Client) Upload(ctx context.Context, bucket, key string, body io.Reader, contentType string) error {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("upload %s/%s: %w", bucket, key, err)
	}
	return nil
}

// Download retrieves an object from the specified bucket.
func (c *Client) Download(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("download %s/%s: %w", bucket, key, err)
	}
	return out.Body, nil
}

// Delete removes an object from the specified bucket.
func (c *Client) Delete(ctx context.Context, bucket, key string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete %s/%s: %w", bucket, key, err)
	}
	return nil
}

// HealthCheck verifies S3 is reachable by listing buckets.
func (c *Client) HealthCheck(ctx context.Context) error {
	_, err := c.s3.ListBuckets(ctx, &s3.ListBucketsInput{})
	return err
}
