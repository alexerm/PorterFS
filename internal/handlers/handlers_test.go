package handlers

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexerm/porterfs/internal/config"
	"github.com/alexerm/porterfs/internal/storage"
	"github.com/go-chi/chi/v5"
)

type mockStorage struct {
	buckets []string
	objects map[string][]storage.ObjectInfo
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		buckets: []string{"test-bucket"},
		objects: map[string][]storage.ObjectInfo{
			"test-bucket": {
				{
					Key:         "test-object.txt",
					Size:        12,
					ContentType: "text/plain",
					ETag:        "abc123",
				},
			},
		},
	}
}

func (m *mockStorage) CreateBucket(ctx context.Context, bucket string) error {
	m.buckets = append(m.buckets, bucket)
	return nil
}

func (m *mockStorage) DeleteBucket(ctx context.Context, bucket string) error {
	for i, b := range m.buckets {
		if b == bucket {
			m.buckets = append(m.buckets[:i], m.buckets[i+1:]...)
			break
		}
	}
	return nil
}

func (m *mockStorage) ListBuckets(ctx context.Context) ([]string, error) {
	return m.buckets, nil
}

func (m *mockStorage) PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) error {
	return nil
}

func (m *mockStorage) GetObject(ctx context.Context, bucket, key string, rangeHeader string) (io.ReadCloser, *storage.ObjectInfo, error) {
	return io.NopCloser(strings.NewReader("test content")), &storage.ObjectInfo{
		Key:         key,
		Size:        12,
		ContentType: "text/plain",
		ETag:        "abc123",
	}, nil
}

func (m *mockStorage) DeleteObject(ctx context.Context, bucket, key string) error {
	return nil
}

func (m *mockStorage) HeadObject(ctx context.Context, bucket, key string) (*storage.ObjectInfo, error) {
	return &storage.ObjectInfo{
		Key:         key,
		Size:        12,
		ContentType: "text/plain",
		ETag:        "abc123",
	}, nil
}

func (m *mockStorage) ListObjects(ctx context.Context, bucket, prefix, delimiter string, maxKeys int) ([]storage.ObjectInfo, bool, error) {
	if objects, exists := m.objects[bucket]; exists {
		return objects, false, nil
	}
	return []storage.ObjectInfo{}, false, nil
}

func (m *mockStorage) InitMultipartUpload(ctx context.Context, bucket, key string) (string, error) {
	return "", nil
}

func (m *mockStorage) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int, reader io.Reader, size int64) (string, error) {
	return "", nil
}

func (m *mockStorage) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []storage.Part) error {
	return nil
}

func (m *mockStorage) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	return nil
}

func (m *mockStorage) ListMultipartUploads(ctx context.Context, bucket string) ([]storage.MultipartUpload, error) {
	return []storage.MultipartUpload{}, nil
}

func TestListBuckets(t *testing.T) {
	mockStore := newMockStorage()
	cfg := config.DefaultConfig()
	handler := New(mockStore, cfg)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ListBuckets(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result ListBucketsResult
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(result.Buckets.Bucket) != 1 {
		t.Errorf("Expected 1 bucket, got %d", len(result.Buckets.Bucket))
	}

	if result.Buckets.Bucket[0].Name != "test-bucket" {
		t.Errorf("Expected bucket name 'test-bucket', got '%s'", result.Buckets.Bucket[0].Name)
	}
}

func TestListObjectsV2(t *testing.T) {
	mockStore := newMockStorage()
	cfg := config.DefaultConfig()
	handler := New(mockStore, cfg)

	r := chi.NewRouter()
	r.Get("/{bucket}", handler.ListObjects)

	req := httptest.NewRequest("GET", "/test-bucket?list-type=2", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result ListObjectsV2Result
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result.Name != "test-bucket" {
		t.Errorf("Expected bucket name 'test-bucket', got '%s'", result.Name)
	}

	if result.KeyCount != 1 {
		t.Errorf("Expected 1 object, got %d", result.KeyCount)
	}

	if len(result.Contents) != 1 {
		t.Errorf("Expected 1 object in contents, got %d", len(result.Contents))
	}

	if result.Contents[0].Key != "test-object.txt" {
		t.Errorf("Expected object key 'test-object.txt', got '%s'", result.Contents[0].Key)
	}
}

func TestGetObject(t *testing.T) {
	mockStore := newMockStorage()
	cfg := config.DefaultConfig()
	handler := New(mockStore, cfg)

	r := chi.NewRouter()
	r.Get("/{bucket}/{object:.*}", handler.GetObject)

	req := httptest.NewRequest("GET", "/test-bucket/test-object.txt", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "text/plain" {
		t.Errorf("Expected Content-Type 'text/plain', got '%s'", w.Header().Get("Content-Type"))
	}

	if w.Body.String() != "test content" {
		t.Errorf("Expected body 'test content', got '%s'", w.Body.String())
	}
}

func TestPutObject(t *testing.T) {
	mockStore := newMockStorage()
	cfg := config.DefaultConfig()
	handler := New(mockStore, cfg)

	r := chi.NewRouter()
	r.Put("/{bucket}/{object:.*}", handler.PutObject)

	body := bytes.NewReader([]byte("test content"))
	req := httptest.NewRequest("PUT", "/test-bucket/new-object.txt", body)
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Content-Length", "12")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
