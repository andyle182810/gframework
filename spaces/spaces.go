package spaces

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var ErrNotFound = errors.New("object not found")

type Options struct {
	Region    string
	Endpoint  string
	Bucket    string
	KeyPrefix string // empty, or ends with "/"
	AccessKey string
	SecretKey string
}

type Client struct {
	s3        *s3.Client
	presigner *s3.PresignClient
	bucket    string
	prefix    string
}

func New(ctx context.Context, opts Options) (*Client, error) {
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

func (c *Client) fullKey(logicalKey string) string {
	return c.prefix + logicalKey
}

func (c *Client) PresignPut(
	ctx context.Context,
	logicalKey, contentType string,
	contentLength int64,
	ttl time.Duration,
) (string, error) {
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
