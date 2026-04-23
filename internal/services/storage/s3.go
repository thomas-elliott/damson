package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// S3Config carries the subset of AWS config we want to expose. The MinIO
// deployment uses a custom endpoint + force-path-style; real AWS uses neither.
type S3Config struct {
	Endpoint       string
	Region         string
	Bucket         string
	AccessKey      string
	SecretKey      string
	ForcePathStyle bool
}

// S3 implements Service against an S3-compatible bucket. Blobs and thumbs
// share the bucket under two prefixes; both are content-addressed by SHA-256.
type S3 struct {
	client *s3.Client
	bucket string
}

func NewS3(ctx context.Context, cfg S3Config) (*S3, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.Region == "" {
		return nil, errors.New("s3 storage: endpoint, bucket, and region are required")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = cfg.ForcePathStyle
	})

	return &S3{client: client, bucket: cfg.Bucket}, nil
}

func (s *S3) PutBlob(ctx context.Context, r io.Reader) (string, int64, error) {
	return s.putHashed(ctx, "blobs", r)
}

func (s *S3) GetBlob(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.get(ctx, "blobs/"+shardedKey(key))
}

func (s *S3) HeadBlob(ctx context.Context, key string) (int64, bool, error) {
	return s.head(ctx, "blobs/"+shardedKey(key))
}

func (s *S3) PutThumb(ctx context.Context, key string, r io.Reader) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    aws.String("thumbs/" + shardedKey(key)),
		Body:   bytes.NewReader(body),
	})
	return err
}

func (s *S3) GetThumb(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.get(ctx, "thumbs/"+shardedKey(key))
}

func (s *S3) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	presigner := s3.NewPresignClient(s.client)
	req, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    aws.String("blobs/" + shardedKey(key)),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// putHashed buffers + hashes before uploading. S3 needs Content-Length and
// seekable bodies; the streaming variant would need multipart uploads which
// we defer until real-world sizes justify the complexity.
func (s *S3) putHashed(ctx context.Context, prefix string, r io.Reader) (string, int64, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return "", 0, err
	}
	sum := sha256.Sum256(buf)
	key := hex.EncodeToString(sum[:])
	objectKey := prefix + "/" + shardedKey(key)

	// HEAD first — skip upload if the blob is already there.
	_, err = s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    &objectKey,
	})
	if err == nil {
		return key, int64(len(buf)), nil
	}
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "NotFound" {
		return "", 0, err
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    &objectKey,
		Body:   bytes.NewReader(buf),
	})
	if err != nil {
		return "", 0, err
	}
	return key, int64(len(buf)), nil
}

func (s *S3) get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (s *S3) head(ctx context.Context, key string) (int64, bool, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NotFound" {
			return 0, false, nil
		}
		return 0, false, err
	}
	if out.ContentLength == nil {
		return 0, true, nil
	}
	return *out.ContentLength, true, nil
}

func shardedKey(hash string) string {
	if len(hash) < 2 {
		return hash
	}
	return hash[:2] + "/" + hash
}
