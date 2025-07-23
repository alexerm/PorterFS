package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var ErrNotFound = errors.New("not found")

type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	ContentType  string
}

type Storage interface {
	CreateBucket(ctx context.Context, bucket string) error
	DeleteBucket(ctx context.Context, bucket string) error
	ListBuckets(ctx context.Context) ([]string, error)

	PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) error
	GetObject(ctx context.Context, bucket, key string, rangeHeader string) (io.ReadCloser, *ObjectInfo, error)
	DeleteObject(ctx context.Context, bucket, key string) error
	HeadObject(ctx context.Context, bucket, key string) (*ObjectInfo, error)
	ListObjects(ctx context.Context, bucket, prefix, delimiter string, maxKeys int) ([]ObjectInfo, bool, error)

	InitMultipartUpload(ctx context.Context, bucket, key string) (string, error)
	UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int, reader io.Reader, size int64) (string, error)
	CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []Part) error
	AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error
	ListMultipartUploads(ctx context.Context, bucket string) ([]MultipartUpload, error)
}

type Part struct {
	PartNumber int
	ETag       string
}

type MultipartUpload struct {
	UploadID  string
	Key       string
	Initiated time.Time
}
