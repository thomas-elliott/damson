package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
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
	client   *s3.Client
	uploader *manager.Uploader
	bucket   string
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

	return &S3{client: client, uploader: manager.NewUploader(client), bucket: cfg.Bucket}, nil
}

func (s *S3) PutBlob(ctx context.Context, r io.Reader, meta ObjectMeta) (string, int64, error) {
	return s.putHashed(ctx, "blobs", r, meta)
}

func (s *S3) GetBlob(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.get(ctx, "blobs/"+shardedKey(key))
}

func (s *S3) HeadBlob(ctx context.Context, key string) (int64, bool, error) {
	return s.head(ctx, "blobs/"+shardedKey(key))
}

func (s *S3) PutOriginal(ctx context.Context, r io.Reader, meta ObjectMeta) (string, int64, error) {
	return s.putHashed(ctx, "originals", r, meta)
}

func (s *S3) GetOriginal(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.get(ctx, "originals/"+shardedKey(key))
}

func (s *S3) HeadOriginal(ctx context.Context, key string) (int64, bool, error) {
	return s.head(ctx, "originals/"+shardedKey(key))
}

func (s *S3) DeleteOriginal(ctx context.Context, key string) error {
	return s.Delete(ctx, "originals", key)
}

func (s *S3) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	var out []ObjectInfo
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: &s.bucket,
		Prefix: aws.String(prefix + "/"),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			// Keys are "<prefix>/<hh>/<hash>"; the bare hash is the last segment.
			info := ObjectInfo{Key: path.Base(*obj.Key)}
			if obj.Size != nil {
				info.Size = *obj.Size
			}
			if obj.LastModified != nil {
				info.LastModified = *obj.LastModified
			}
			out = append(out, info)
		}
	}
	return out, nil
}

func (s *S3) StatMeta(ctx context.Context, prefix, key string) (ObjectMeta, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    aws.String(prefix + "/" + shardedKey(key)),
	})
	if err != nil {
		return ObjectMeta{}, err
	}
	return metaFromS3(out.Metadata), nil
}

func (s *S3) Delete(ctx context.Context, prefix, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    aws.String(prefix + "/" + shardedKey(key)),
	})
	return err
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

// putHashed streams r to a temp file (in os.TempDir()) while hashing, then
// multipart-uploads the seekable file. Memory stays bounded regardless of
// upload size.
func (s *S3) putHashed(ctx context.Context, prefix string, r io.Reader, meta ObjectMeta) (string, int64, error) {
	tmp, err := os.CreateTemp("", "damson-upload-*")
	if err != nil {
		return "", 0, err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), r)
	if err != nil {
		return "", 0, err
	}
	key := hex.EncodeToString(h.Sum(nil))
	objectKey := prefix + "/" + shardedKey(key)

	// HEAD first — skip upload if the blob is already there.
	_, err = s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    &objectKey,
	})
	if err == nil {
		return key, n, nil
	}
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "NotFound" {
		return "", 0, err
	}

	// The hashing io.Copy left the offset at EOF; rewind before the upload.
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return "", 0, err
	}
	_, err = s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:   &s.bucket,
		Key:      &objectKey,
		Body:     tmp,
		Metadata: metaToS3(meta),
	})
	if err != nil {
		return "", 0, err
	}
	return key, n, nil
}

// metaToS3 renders ObjectMeta as S3 user metadata. Keys are lowercase-hyphen
// header tokens; the filename is URL-encoded so non-ASCII values stay valid
// header content. Returns nil when nothing is set so PutObject omits it.
func metaToS3(m ObjectMeta) map[string]string {
	md := map[string]string{}
	if m.Filename != "" {
		md["filename"] = url.QueryEscape(m.Filename)
	}
	if m.LibraryID != 0 {
		md["library-id"] = strconv.FormatUint(uint64(m.LibraryID), 10)
	}
	if m.AssetID != 0 {
		md["asset-id"] = strconv.FormatUint(uint64(m.AssetID), 10)
	}
	if m.VersionID != 0 {
		md["version-id"] = strconv.FormatUint(uint64(m.VersionID), 10)
	}
	if m.Kind != "" {
		md["kind"] = m.Kind
	}
	if len(md) == 0 {
		return nil
	}
	return md
}

// metaFromS3 reverses metaToS3, decoding S3 user metadata back into ObjectMeta.
// Malformed numeric fields decode to zero rather than erroring — the metadata
// is a best-effort fallback, not authoritative.
func metaFromS3(md map[string]string) ObjectMeta {
	var m ObjectMeta
	if v, err := url.QueryUnescape(md["filename"]); err == nil {
		m.Filename = v
	}
	if v, err := strconv.ParseUint(md["library-id"], 10, 64); err == nil {
		m.LibraryID = uint(v)
	}
	if v, err := strconv.ParseUint(md["asset-id"], 10, 64); err == nil {
		m.AssetID = uint(v)
	}
	if v, err := strconv.ParseUint(md["version-id"], 10, 64); err == nil {
		m.VersionID = uint(v)
	}
	m.Kind = md["kind"]
	return m
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
