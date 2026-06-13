// Package spaces provides an S3-compatible object storage client for DigitalOcean Spaces
// (or any S3-compatible endpoint) with presigned upload/download URLs, metadata lookup,
// ranged reads, and deletion.
//
// All object keys are namespaced under the configured KeyPrefix and validated before use:
// keys containing "..", ".", or empty path segments, leading slashes, or control characters
// are rejected with ErrInvalidKey so callers cannot escape the prefix namespace
// (e.g. a per-tenant prefix).
//
// Basic usage:
//
//	client, err := spaces.New(ctx, spaces.Options{
//	    Region:    "nyc3",
//	    Endpoint:  "https://nyc3.digitaloceanspaces.com",
//	    Bucket:    "my-bucket",
//	    KeyPrefix: "tenant-42/",
//	    AccessKey: accessKey,
//	    SecretKey: secretKey,
//	})
//	if err != nil {
//	    return err
//	}
//
//	url, err := client.PresignPut(ctx, "avatars/user.png", "image/png", size, 15*time.Minute)
//
// Presign TTLs must be positive and at most MaxPresignTTL (7 days, the S3 protocol limit).
package spaces

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const MaxPresignTTL = 7 * 24 * time.Hour

var (
	ErrNotFound       = errors.New("object not found")
	ErrInvalidKey     = errors.New("spaces: invalid object key")
	ErrInvalidTTL     = errors.New("spaces: presign TTL must be positive and at most 7 days")
	ErrInvalidOptions = errors.New("spaces: invalid options")
)

type Options struct {
	Region    string
	Endpoint  string
	Bucket    string
	KeyPrefix string // empty, or ends with "/"
	AccessKey string
	SecretKey string
}

func (o Options) validate() error {
	switch {
	case o.Region == "":
		return fmt.Errorf("%w: Region is required", ErrInvalidOptions)
	case o.Endpoint == "":
		return fmt.Errorf("%w: Endpoint is required", ErrInvalidOptions)
	case o.Bucket == "":
		return fmt.Errorf("%w: Bucket is required", ErrInvalidOptions)
	case o.AccessKey == "" || o.SecretKey == "":
		return fmt.Errorf("%w: AccessKey and SecretKey are required", ErrInvalidOptions)
	case o.KeyPrefix != "" && !strings.HasSuffix(o.KeyPrefix, "/"):
		return fmt.Errorf("%w: KeyPrefix must be empty or end with \"/\"", ErrInvalidOptions)
	}

	return nil
}

type Client struct {
	s3        *s3.Client
	presigner *s3.PresignClient
	bucket    string
	prefix    string
}

func New(ctx context.Context, opts Options) (*Client, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	creds := credentials.NewStaticCredentialsProvider(
		opts.AccessKey,
		opts.SecretKey,
		"",
	)

	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(opts.Region),
		config.WithCredentialsProvider(creds),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(opts.Endpoint)
		o.UsePathStyle = false
	})

	return &Client{
		s3:        s3Client,
		presigner: s3.NewPresignClient(s3Client),
		bucket:    opts.Bucket,
		prefix:    opts.KeyPrefix,
	}, nil
}

// validateKey rejects keys that could escape the configured prefix namespace or
// produce ambiguous URLs: empty keys, leading slashes, control characters, and
// "..", ".", or empty path segments.
func validateKey(logicalKey string) error {
	if logicalKey == "" {
		return fmt.Errorf("%w: key is empty", ErrInvalidKey)
	}

	if strings.HasPrefix(logicalKey, "/") {
		return fmt.Errorf("%w: key must not start with \"/\"", ErrInvalidKey)
	}

	for _, r := range logicalKey {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%w: key contains control characters", ErrInvalidKey)
		}
	}

	for segment := range strings.SplitSeq(logicalKey, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("%w: key contains an empty, \".\" or \"..\" path segment", ErrInvalidKey)
		}
	}

	return nil
}

func validateTTL(ttl time.Duration) error {
	if ttl <= 0 || ttl > MaxPresignTTL {
		return fmt.Errorf("%w: got %v", ErrInvalidTTL, ttl)
	}

	return nil
}

func (c *Client) fullKey(logicalKey string) string {
	return c.prefix + logicalKey
}

func (c *Client) PresignPut(
	ctx context.Context,
	logicalKey, contentType string,
	contentLength int64,
	ttl time.Duration,
) (string, error) {
	if err := validateKey(logicalKey); err != nil {
		return "", err
	}

	if err := validateTTL(ttl); err != nil {
		return "", err
	}

	//nolint:exhaustruct
	input := &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(c.fullKey(logicalKey)),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(contentLength),
	}

	req, err := c.presigner.PresignPutObject(
		ctx,
		input,
		s3.WithPresignExpires(ttl),
	)
	if err != nil {
		return "", fmt.Errorf("presign put: %w", err)
	}

	return req.URL, nil
}

func (c *Client) PresignGet(ctx context.Context, logicalKey string, ttl time.Duration) (string, error) {
	if err := validateKey(logicalKey); err != nil {
		return "", err
	}

	if err := validateTTL(ttl); err != nil {
		return "", err
	}

	req, err := c.presigner.PresignGetObject(
		ctx, &s3.GetObjectInput{ //nolint:exhaustruct
			Bucket: aws.String(c.bucket),
			Key:    aws.String(c.fullKey(logicalKey)),
		},
		s3.WithPresignExpires(ttl),
	)
	if err != nil {
		return "", fmt.Errorf("presign get: %w", err)
	}

	return req.URL, nil
}

type HeadResult struct {
	ContentLength int64
	ContentType   string
}

func (c *Client) Head(ctx context.Context, logicalKey string) (*HeadResult, error) {
	if err := validateKey(logicalKey); err != nil {
		return nil, err
	}

	out, err := c.s3.HeadObject(
		ctx,
		&s3.HeadObjectInput{ //nolint:exhaustruct
			Bucket: aws.String(c.bucket),
			Key:    aws.String(c.fullKey(logicalKey)),
		},
	)
	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("head object: %w", err)
	}

	res := &HeadResult{} //nolint:exhaustruct
	if out.ContentLength != nil {
		res.ContentLength = *out.ContentLength
	}

	if out.ContentType != nil {
		res.ContentType = *out.ContentType
	}

	return res, nil
}

func (c *Client) RangeGet(ctx context.Context, logicalKey string, maxBytes int64) ([]byte, error) {
	if err := validateKey(logicalKey); err != nil {
		return nil, err
	}

	if maxBytes <= 0 {
		return nil, nil
	}

	rng := fmt.Sprintf("bytes=0-%d", maxBytes-1)
	//nolint:exhaustruct
	out, err := c.s3.GetObject(
		ctx, &s3.GetObjectInput{
			Bucket: aws.String(c.bucket),
			Key:    aws.String(c.fullKey(logicalKey)),
			Range:  aws.String(rng),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("range get: %w", err)
	}
	defer out.Body.Close()

	buf := &bytes.Buffer{}
	if _, err := io.CopyN(buf, out.Body, maxBytes); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read range body: %w", err)
	}

	return buf.Bytes(), nil
}

func (c *Client) Delete(ctx context.Context, logicalKey string) error {
	if err := validateKey(logicalKey); err != nil {
		return err
	}

	_, err := c.s3.DeleteObject(
		ctx,
		&s3.DeleteObjectInput{ //nolint:exhaustruct
			Bucket: aws.String(c.bucket),
			Key:    aws.String(c.fullKey(logicalKey)),
		},
	)
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}

	return nil
}
