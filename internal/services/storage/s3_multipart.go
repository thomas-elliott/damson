package storage

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func (s *S3) BeginMultipart(ctx context.Context, prefix, objectKey string, meta ObjectMeta) (MultipartUpload, error) {
	key := prefix + "/" + shardedKey(objectKey)
	out, err := s.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:   &s.bucket,
		Key:      &key,
		Metadata: metaToS3(meta),
	})
	if err != nil {
		return nil, s.wrapS3("multipart-create", key, err)
	}
	return &s3Multipart{s: s, key: key, bare: objectKey, uploadID: aws.ToString(out.UploadId)}, nil
}

func (s *S3) ResumeMultipart(_ context.Context, prefix, objectKey, uploadID string) (MultipartUpload, error) {
	if uploadID == "" {
		return nil, errors.New("s3 multipart: empty upload id")
	}
	return &s3Multipart{
		s:        s,
		key:      prefix + "/" + shardedKey(objectKey),
		bare:     objectKey,
		uploadID: uploadID,
	}, nil
}

type s3Multipart struct {
	s        *S3
	key      string
	bare     string
	uploadID string
}

func (m *s3Multipart) ID() string { return m.uploadID }

func (m *s3Multipart) UploadPart(ctx context.Context, n int, size int64, r io.Reader) (string, error) {
	if n < 1 || n > s3MaxParts {
		return "", fmt.Errorf("s3 multipart: part number %d out of range", n)
	}
	out, err := m.s.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:        &m.s.bucket,
		Key:           &m.key,
		UploadId:      &m.uploadID,
		PartNumber:    aws.Int32(int32(n)),
		ContentLength: aws.Int64(size),
		Body:          r,
	}, func(o *s3.Options) {
		// r is the HTTP request body and cannot be rewound, but both the SigV4
		// payload hash and the default checksum want to read it before sending.
		// Over TLS the SDK already switches to UNSIGNED-PAYLOAD; this makes it
		// unconditional. The object's SHA-256 is verified server-side before
		// anything is published, and buffering the part instead would put back
		// the memory pressure the chunked path exists to avoid.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.APIOptions = append(o.APIOptions, v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware)
	})
	if err != nil {
		return "", m.s.wrapS3("multipart-part", m.key, err)
	}
	return aws.ToString(out.ETag), nil
}

func (m *s3Multipart) Complete(ctx context.Context, parts []Part) (string, error) {
	if len(parts) == 0 {
		return "", errors.New("s3 multipart: no parts to complete")
	}
	ordered := sortedParts(parts)

	completed := make([]types.CompletedPart, 0, len(ordered))
	for _, p := range ordered {
		completed = append(completed, types.CompletedPart{
			PartNumber: aws.Int32(int32(p.Number)),
			ETag:       aws.String(p.ETag),
		})
	}
	_, err := m.s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          &m.s.bucket,
		Key:             &m.key,
		UploadId:        &m.uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: completed},
	})
	if err != nil {
		return "", m.s.wrapS3("multipart-complete", m.key, err)
	}
	return m.bare, nil
}

func (m *s3Multipart) Abort(ctx context.Context) error {
	_, err := m.s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   &m.s.bucket,
		Key:      &m.key,
		UploadId: &m.uploadID,
	})
	if err != nil {
		return m.s.wrapS3("multipart-abort", m.key, err)
	}
	return nil
}
