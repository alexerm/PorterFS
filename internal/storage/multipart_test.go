package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMultipartUpload(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "porter-multipart-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	bucket := "test-bucket"
	key := "test-multipart-object"

	// Create bucket first
	err = storage.CreateBucket(ctx, bucket)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("InitiateMultipartUpload", func(t *testing.T) {
		uploadID, err := storage.InitMultipartUpload(ctx, bucket, key)
		if err != nil {
			t.Errorf("InitMultipartUpload failed: %v", err)
		}
		if uploadID == "" {
			t.Error("Expected non-empty upload ID")
		}

		// Check if multipart directory was created
		multipartDir := filepath.Join(tmpDir, ".multipart", bucket, uploadID)
		if _, err := os.Stat(multipartDir); os.IsNotExist(err) {
			t.Error("Multipart directory was not created")
		}

		// Test upload parts
		part1Data := "Hello, "
		part2Data := "World!"

		// Upload part 1
		etag1, err := storage.UploadPart(ctx, bucket, key, uploadID, 1, strings.NewReader(part1Data), int64(len(part1Data)))
		if err != nil {
			t.Errorf("UploadPart 1 failed: %v", err)
		}
		if etag1 == "" {
			t.Error("Expected non-empty ETag for part 1")
		}

		// Upload part 2
		etag2, err := storage.UploadPart(ctx, bucket, key, uploadID, 2, strings.NewReader(part2Data), int64(len(part2Data)))
		if err != nil {
			t.Errorf("UploadPart 2 failed: %v", err)
		}
		if etag2 == "" {
			t.Error("Expected non-empty ETag for part 2")
		}

		// Complete multipart upload
		parts := []Part{
			{PartNumber: 1, ETag: etag1},
			{PartNumber: 2, ETag: etag2},
		}

		err = storage.CompleteMultipartUpload(ctx, bucket, key, uploadID, parts)
		if err != nil {
			t.Errorf("CompleteMultipartUpload failed: %v", err)
		}

		// Verify final object
		reader, info, err := storage.GetObject(ctx, bucket, key, "")
		if err != nil {
			t.Errorf("GetObject failed after multipart upload: %v", err)
		}
		defer reader.Close()

		content := make([]byte, info.Size)
		_, err = reader.Read(content)
		if err != nil {
			t.Errorf("Failed to read final object: %v", err)
		}

		expectedContent := part1Data + part2Data
		if string(content) != expectedContent {
			t.Errorf("Expected content '%s', got '%s'", expectedContent, string(content))
		}

		// Check that multipart directory was cleaned up
		if _, err := os.Stat(multipartDir); !os.IsNotExist(err) {
			t.Error("Multipart directory was not cleaned up after completion")
		}
	})

	t.Run("AbortMultipartUpload", func(t *testing.T) {
		uploadID, err := storage.InitMultipartUpload(ctx, bucket, "abort-test")
		if err != nil {
			t.Fatal(err)
		}

		// Upload a part
		_, err = storage.UploadPart(ctx, bucket, "abort-test", uploadID, 1, strings.NewReader("test data"), 9)
		if err != nil {
			t.Fatal(err)
		}

		// Abort the upload
		err = storage.AbortMultipartUpload(ctx, bucket, "abort-test", uploadID)
		if err != nil {
			t.Errorf("AbortMultipartUpload failed: %v", err)
		}

		// Check that multipart directory was cleaned up
		multipartDir := filepath.Join(tmpDir, ".multipart", bucket, uploadID)
		if _, err := os.Stat(multipartDir); !os.IsNotExist(err) {
			t.Error("Multipart directory was not cleaned up after abort")
		}
	})

	t.Run("ListMultipartUploads", func(t *testing.T) {
		// Create a few multipart uploads
		uploadID1, _ := storage.InitMultipartUpload(ctx, bucket, "list-test-1")
		uploadID2, _ := storage.InitMultipartUpload(ctx, bucket, "list-test-2")

		uploads, err := storage.ListMultipartUploads(ctx, bucket)
		if err != nil {
			t.Errorf("ListMultipartUploads failed: %v", err)
		}

		if len(uploads) < 2 {
			t.Errorf("Expected at least 2 uploads, got %d", len(uploads))
		}

		// Check that our uploads are in the list
		found1, found2 := false, false
		for _, upload := range uploads {
			if upload.UploadID == uploadID1 && upload.Key == "list-test-1" {
				found1 = true
			}
			if upload.UploadID == uploadID2 && upload.Key == "list-test-2" {
				found2 = true
			}
		}

		if !found1 || !found2 {
			t.Error("Not all created uploads were found in the list")
		}

		// Clean up
		storage.AbortMultipartUpload(ctx, bucket, "list-test-1", uploadID1)
		storage.AbortMultipartUpload(ctx, bucket, "list-test-2", uploadID2)
	})
}

func TestRangeRequests(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "porter-range-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	bucket := "test-bucket"
	key := "test-range-object"

	// Create bucket and object
	err = storage.CreateBucket(ctx, bucket)
	if err != nil {
		t.Fatal(err)
	}

	testContent := "0123456789abcdefghijklmnopqrstuvwxyz"
	err = storage.PutObject(ctx, bucket, key, strings.NewReader(testContent), int64(len(testContent)), "text/plain")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("ValidRangeRequest", func(t *testing.T) {
		// Request bytes 5-9 (should be "56789")
		reader, info, err := storage.GetObject(ctx, bucket, key, "bytes=5-9")
		if err != nil {
			t.Errorf("Range request failed: %v", err)
		}
		defer reader.Close()

		if info.Size != 5 {
			t.Errorf("Expected range size 5, got %d", info.Size)
		}

		content := make([]byte, info.Size)
		_, err = reader.Read(content)
		if err != nil {
			t.Errorf("Failed to read range content: %v", err)
		}

		expected := "56789"
		if string(content) != expected {
			t.Errorf("Expected range content '%s', got '%s'", expected, string(content))
		}
	})

	t.Run("RangeFromStart", func(t *testing.T) {
		// Request bytes 0-4 (should be "01234")
		reader, info, err := storage.GetObject(ctx, bucket, key, "bytes=0-4")
		if err != nil {
			t.Errorf("Range request failed: %v", err)
		}
		defer reader.Close()

		content := make([]byte, info.Size)
		reader.Read(content)

		expected := "01234"
		if string(content) != expected {
			t.Errorf("Expected range content '%s', got '%s'", expected, string(content))
		}
	})

	t.Run("RangeToEnd", func(t *testing.T) {
		// Request bytes 30- (should be "uvwxyz")
		reader, info, err := storage.GetObject(ctx, bucket, key, "bytes=30-")
		if err != nil {
			t.Errorf("Range request failed: %v", err)
		}
		defer reader.Close()

		content := make([]byte, info.Size)
		reader.Read(content)

		expected := "uvwxyz"
		if string(content) != expected {
			t.Errorf("Expected range content '%s', got '%s'", expected, string(content))
		}
	})

	t.Run("InvalidRangeFormat", func(t *testing.T) {
		_, _, err := storage.GetObject(ctx, bucket, key, "invalid-range")
		if err == nil {
			t.Error("Expected error for invalid range format")
		}
	})

	t.Run("InvalidRangeValues", func(t *testing.T) {
		// Request bytes beyond file size
		_, _, err := storage.GetObject(ctx, bucket, key, "bytes=100-200")
		if err == nil {
			t.Error("Expected error for range beyond file size")
		}
	})
}
