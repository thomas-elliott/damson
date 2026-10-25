package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"path"
	"strconv"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
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

func (s *S3) Stat(ctx context.Context, prefix, key string) (ObjectInfo, bool, error) {
	objectKey := prefix + "/" + shardedKey(key)
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &objectKey})
	if isNotFound(err) {
		return ObjectInfo{}, false, nil
	}
	if err != nil {
		return ObjectInfo{}, false, s.wrapS3("head", objectKey, err)
	}
	info := ObjectInfo{Key: key, Size: aws.ToInt64(out.ContentLength)}
	if out.LastModified != nil {
		info.LastModified = *out.LastModified
	}
	return info, true, nil
}

func (s *S3) Touch(ctx context.Context, prefix, key string) (bool, error) {
	return s.touch(ctx, prefix+"/"+shardedKey(key))
}

// touch refreshes an existing object's last-modified time by copying it onto
// itself. S3 refuses a self-copy that changes nothing, so the copy replaces
// the metadata with what the object already carries.
func (s *S3) touch(ctx context.Context, objectKey string) (bool, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &objectKey})
	if isNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, s.wrapS3("head", objectKey, err)
	}
	if err := s.copyObject(ctx, objectKey, objectKey, aws.ToInt64(out.ContentLength), metaFromS3(out.Metadata)); err != nil {
		return false, err
	}
	return true, nil
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

const (
	// Outside cleanup.sweptPrefixes: a staging key is random, not a content
	// hash, so the orphan sweep would list it and then fail to delete it.
	stagingPrefix = "staging"

	// Bodies up to this size are hashed in memory, so their key is known
	// before anything is written and they skip staging entirely.
	inlinePutBytes = 16 << 20

	// Peak buffering is uploadPartBytes × uploadConcurrency. At 16 MiB a
	// 64 GiB object is 4096 parts, inside S3's 10,000-part ceiling.
	uploadPartBytes   = 16 << 20
	uploadConcurrency = 4

	// S3 caps a one-shot CopyObject at 5 GiB; past that the copy runs as
	// multipart part-copies.
	maxSingleCopyBytes = 5 << 30
	copyPartBytes      = 1 << 30

	s3MaxParts = 10000
)

// putHashed writes r to its content-addressed key without spooling to local
// scratch. The key is the hash of the content, so a body too large to hash in
// memory lands at a random staging key first and is copied server-side to its
// final key once the hash is known; that copy never crosses this process.
func (s *S3) putHashed(ctx context.Context, prefix string, r io.Reader, meta ObjectMeta) (string, int64, error) {
	head := make([]byte, inlinePutBytes)
	n, err := io.ReadFull(r, head)
	switch {
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return s.putInline(ctx, prefix, head[:n], meta)
	case err != nil:
		return "", 0, err
	}
	return s.putStaged(ctx, prefix, io.MultiReader(bytes.NewReader(head), r), meta)
}

func (s *S3) putInline(ctx context.Context, prefix string, body []byte, meta ObjectMeta) (string, int64, error) {
	sum := sha256.Sum256(body)
	key := hex.EncodeToString(sum[:])
	objectKey := prefix + "/" + shardedKey(key)

	present, err := s.touch(ctx, objectKey)
	if err != nil {
		return "", 0, err
	}
	if present {
		return key, int64(len(body)), nil
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:   &s.bucket,
		Key:      &objectKey,
		Body:     bytes.NewReader(body),
		Metadata: metaToS3(meta),
	})
	if err != nil {
		return "", 0, s.wrapS3("put", objectKey, err)
	}
	return key, int64(len(body)), nil
}

func (s *S3) putStaged(ctx context.Context, prefix string, r io.Reader, meta ObjectMeta) (string, int64, error) {
	stagingKey, err := newStagingKey()
	if err != nil {
		return "", 0, err
	}

	// Detached from the request: a client abort must still clear the staging
	// object, and the request context is already cancelled by then.
	cleanupCtx := context.WithoutCancel(ctx)
	defer s.dropStaging(cleanupCtx, stagingKey)

	h := sha256.New()
	// Uploader buffers each part and replays from that buffer on retry, so it
	// never re-reads this reader — the running hash stays correct.
	counted := &countingReader{r: io.TeeReader(r, h)}

	_, err = s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    &stagingKey,
		Body:   counted,
	}, func(u *manager.Uploader) {
		u.PartSize = uploadPartBytes
		u.Concurrency = uploadConcurrency
	})
	if err != nil {
		s.abortStagingUpload(cleanupCtx, stagingKey, err)
		return "", 0, err
	}

	size := counted.count()
	key := hex.EncodeToString(h.Sum(nil))
	objectKey := prefix + "/" + shardedKey(key)

	present, err := s.touch(ctx, objectKey)
	if err != nil {
		return "", 0, err
	}
	if present {
		return key, size, nil
	}
	if err := s.copyObject(ctx, stagingKey, objectKey, size, meta); err != nil {
		return "", 0, err
	}
	return key, size, nil
}

func (s *S3) copyObject(ctx context.Context, srcKey, dstKey string, size int64, meta ObjectMeta) error {
	if size > maxSingleCopyBytes {
		return s.copyObjectMultipart(ctx, srcKey, dstKey, size, meta)
	}
	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:            &s.bucket,
		Key:               &dstKey,
		CopySource:        aws.String(s.bucket + "/" + srcKey),
		Metadata:          metaToS3(meta),
		MetadataDirective: types.MetadataDirectiveReplace,
	})
	if err != nil {
		return s.wrapS3("copy", dstKey, err)
	}
	return nil
}

func (s *S3) copyObjectMultipart(ctx context.Context, srcKey, dstKey string, size int64, meta ObjectMeta) error {
	create, err := s.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:   &s.bucket,
		Key:      &dstKey,
		Metadata: metaToS3(meta),
	})
	if err != nil {
		return s.wrapS3("copy-create", dstKey, err)
	}

	abort := func() {
		_, aerr := s.client.AbortMultipartUpload(context.WithoutCancel(ctx), &s3.AbortMultipartUploadInput{
			Bucket:   &s.bucket,
			Key:      &dstKey,
			UploadId: create.UploadId,
		})
		if aerr != nil {
			slog.Error("storage: abandoned multipart copy",
				"object_key", dstKey, "bucket", s.bucket, "upload_id", aws.ToString(create.UploadId), "err", aerr)
		}
	}

	var parts []types.CompletedPart
	for offset, n := int64(0), int32(1); offset < size; offset, n = offset+copyPartBytes, n+1 {
		last := min(offset+copyPartBytes-1, size-1)
		out, err := s.client.UploadPartCopy(ctx, &s3.UploadPartCopyInput{
			Bucket:          &s.bucket,
			Key:             &dstKey,
			UploadId:        create.UploadId,
			PartNumber:      aws.Int32(n),
			CopySource:      aws.String(s.bucket + "/" + srcKey),
			CopySourceRange: aws.String(fmt.Sprintf("bytes=%d-%d", offset, last)),
		})
		if err != nil {
			abort()
			return s.wrapS3("copy-part", dstKey, err)
		}
		var etag *string
		if out.CopyPartResult != nil {
			etag = out.CopyPartResult.ETag
		}
		parts = append(parts, types.CompletedPart{ETag: etag, PartNumber: aws.Int32(n)})
	}

	_, err = s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          &s.bucket,
		Key:             &dstKey,
		UploadId:        create.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: parts},
	})
	if err != nil {
		abort()
		return s.wrapS3("copy-complete", dstKey, err)
	}
	return nil
}

// isNotFound reports whether err is S3's answer for a missing object.
func isNotFound(err error) bool {
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == "NotFound"
}

// abortStagingUpload re-sends the abort the uploader already attempted. The
// uploader uses the request context, which in a client abort is the context
// that just died, so its own abort never reaches the bucket.
func (s *S3) abortStagingUpload(ctx context.Context, stagingKey string, cause error) {
	var failure manager.MultiUploadFailure
	if !errors.As(cause, &failure) || failure.UploadID() == "" {
		return
	}
	uploadID := failure.UploadID()
	_, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   &s.bucket,
		Key:      &stagingKey,
		UploadId: &uploadID,
	})
	if err != nil {
		slog.Error("storage: incomplete multipart upload left behind",
			"object_key", stagingKey, "bucket", s.bucket, "upload_id", uploadID, "err", err)
	}
}

func (s *S3) dropStaging(ctx context.Context, stagingKey string) {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &stagingKey,
	})
	if err != nil {
		slog.Error("storage: staging object left behind",
			"object_key", stagingKey, "bucket", s.bucket, "err", err)
	}
}

func newStagingKey() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("staging key: %w", err)
	}
	return stagingPrefix + "/" + hex.EncodeToString(b[:]), nil
}

type countingReader struct {
	r io.Reader
	n atomic.Int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		c.n.Add(int64(n))
	}
	return n, err
}

func (c *countingReader) count() int64 { return c.n.Load() }

func (s *S3) wrapS3(op, objectKey string, err error) error {
	if err == nil {
		return nil
	}
	code := ""
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code = apiErr.ErrorCode()
	}
	slog.Error("storage: s3 operation failed",
		"op", op, "object_key", objectKey, "bucket", s.bucket, "aws_code", code, "err", err)
	return fmt.Errorf("s3 %s (bucket %s, key %s, code %q): %w", op, s.bucket, objectKey, code, err)
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
		return nil, s.wrapS3("get", key, err)
	}
	return out.Body, nil
}

func (s *S3) head(ctx context.Context, key string) (int64, bool, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	if isNotFound(err) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, s.wrapS3("head", key, err)
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
