package storage

import (
	"errors"
	"strings"
	"testing"

	"github.com/aws/smithy-go"
)

func TestWrapS3_Nil(t *testing.T) {
	s := &S3{bucket: "assets"}
	if err := s.wrapS3("upload", "blobs/ab/abcd", nil); err != nil {
		t.Fatalf("wrapS3 on nil err: got %v, want nil", err)
	}
}

func TestWrapS3_PlainError(t *testing.T) {
	s := &S3{bucket: "assets"}
	base := errors.New("connection reset")
	got := s.wrapS3("upload", "blobs/ab/abcd", base)
	if got == nil {
		t.Fatal("wrapS3 on non-nil err: got nil")
	}
	if !errors.Is(got, base) {
		t.Fatalf("wrapS3 must wrap the base error via %%w; errors.Is failed for %v", got)
	}
	msg := got.Error()
	for _, want := range []string{"upload", "assets", "blobs/ab/abcd"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("wrapped message %q missing %q — context not carried", msg, want)
		}
	}
}

func TestWrapS3_APIErrorCode(t *testing.T) {
	s := &S3{bucket: "assets"}
	apiErr := &smithy.GenericAPIError{Code: "AccessDenied", Message: "forbidden"}
	got := s.wrapS3("head", "originals/cd/cdef", apiErr)
	if got == nil {
		t.Fatal("wrapS3 on API err: got nil")
	}
	var recovered smithy.APIError
	if !errors.As(got, &recovered) {
		t.Fatalf("wrapped error must still expose the smithy.APIError; errors.As failed for %v", got)
	}
	if recovered.ErrorCode() != "AccessDenied" {
		t.Fatalf("recovered code: got %q, want AccessDenied", recovered.ErrorCode())
	}
	if !strings.Contains(got.Error(), "AccessDenied") {
		t.Fatalf("wrapped message %q must include the AWS error code", got.Error())
	}
}
