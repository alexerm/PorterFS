package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStorage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "porter-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	t.Run("CreateBucket", func(t *testing.T) {
		err := storage.CreateBucket(ctx, "test-bucket")
		if err != nil {
			t.Errorf("CreateBucket failed: %v", err)
		}

		bucketPath := filepath.Join(tmpDir, "test-bucket")
		if _, err := os.Stat(bucketPath); os.IsNotExist(err) {
			t.Error("Bucket directory was not created")
		}
	})

	t.Run("ListBuckets", func(t *testing.T) {
		buckets, err := storage.ListBuckets(ctx)
		if err != nil {
			t.Errorf("ListBuckets failed: %v", err)
		}

		if len(buckets) != 1 || buckets[0] != "test-bucket" {
			t.Errorf("Expected [test-bucket], got %v", buckets)
		}
	})

	t.Run("PutObject", func(t *testing.T) {
		content := "test content"
		reader := strings.NewReader(content)

		err := storage.PutObject(ctx, "test-bucket", "test-object.txt", reader, int64(len(content)), "text/plain")
		if err != nil {
			t.Errorf("PutObject failed: %v", err)
		}

		objectPath := filepath.Join(tmpDir, "test-bucket", "test-object.txt")
		if _, err := os.Stat(objectPath); os.IsNotExist(err) {
			t.Error("Object file was not created")
		}
	})

	t.Run("GetObject", func(t *testing.T) {
		reader, info, err := storage.GetObject(ctx, "test-bucket", "test-object.txt", "")
		if err != nil {
			t.Errorf("GetObject failed: %v", err)
		}
		defer reader.Close()

		if info.Key != "test-object.txt" {
			t.Errorf("Expected key 'test-object.txt', got '%s'", info.Key)
		}

		if info.Size != 12 {
			t.Errorf("Expected size 12, got %d", info.Size)
		}

		content := make([]byte, info.Size)
		_, err = reader.Read(content)
		if err != nil {
			t.Errorf("Failed to read object content: %v", err)
		}

		if string(content) != "test content" {
			t.Errorf("Expected 'test content', got '%s'", string(content))
		}
	})

	t.Run("HeadObject", func(t *testing.T) {
		info, err := storage.HeadObject(ctx, "test-bucket", "test-object.txt")
		if err != nil {
			t.Errorf("HeadObject failed: %v", err)
		}

		if info.Key != "test-object.txt" {
			t.Errorf("Expected key 'test-object.txt', got '%s'", info.Key)
		}

		if info.Size != 12 {
			t.Errorf("Expected size 12, got %d", info.Size)
		}
	})

	t.Run("ListObjects", func(t *testing.T) {
		objects, isTruncated, err := storage.ListObjects(ctx, "test-bucket", "", "", 1000)
		if err != nil {
			t.Errorf("ListObjects failed: %v", err)
		}

		if isTruncated {
			t.Error("Expected isTruncated to be false")
		}

		if len(objects) != 1 {
			t.Errorf("Expected 1 object, got %d", len(objects))
		}

		if objects[0].Key != "test-object.txt" {
			t.Errorf("Expected key 'test-object.txt', got '%s'", objects[0].Key)
		}
	})

	t.Run("DeleteObject", func(t *testing.T) {
		err := storage.DeleteObject(ctx, "test-bucket", "test-object.txt")
		if err != nil {
			t.Errorf("DeleteObject failed: %v", err)
		}

		objectPath := filepath.Join(tmpDir, "test-bucket", "test-object.txt")
		if _, err := os.Stat(objectPath); !os.IsNotExist(err) {
			t.Error("Object file was not deleted")
		}
	})

	t.Run("DeleteBucket", func(t *testing.T) {
		err := storage.DeleteBucket(ctx, "test-bucket")
		if err != nil {
			t.Errorf("DeleteBucket failed: %v", err)
		}

		bucketPath := filepath.Join(tmpDir, "test-bucket")
		if _, err := os.Stat(bucketPath); !os.IsNotExist(err) {
			t.Error("Bucket directory was not deleted")
		}
	})
}
