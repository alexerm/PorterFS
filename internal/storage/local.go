package storage

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type LocalStorage struct {
	rootPath string
}

func NewLocalStorage(rootPath string) (*LocalStorage, error) {
	if err := os.MkdirAll(rootPath, 0755); err != nil {
		return nil, err
	}

	return &LocalStorage{rootPath: rootPath}, nil
}

func (l *LocalStorage) bucketPath(bucket string) string {
	return filepath.Join(l.rootPath, bucket)
}

func (l *LocalStorage) objectPath(bucket, key string) string {
	return filepath.Join(l.bucketPath(bucket), key)
}

func (l *LocalStorage) CreateBucket(ctx context.Context, bucket string) error {
	return os.MkdirAll(l.bucketPath(bucket), 0755)
}

func (l *LocalStorage) DeleteBucket(ctx context.Context, bucket string) error {
	return os.Remove(l.bucketPath(bucket))
}

func (l *LocalStorage) ListBuckets(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(l.rootPath)
	if err != nil {
		return nil, err
	}

	var buckets []string
	for _, entry := range entries {
		if entry.IsDir() {
			buckets = append(buckets, entry.Name())
		}
	}

	return buckets, nil
}

func (l *LocalStorage) PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) error {
	objectPath := l.objectPath(bucket, key)

	dir := filepath.Dir(objectPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(objectPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	return err
}

func (l *LocalStorage) GetObject(ctx context.Context, bucket, key string, rangeHeader string) (io.ReadCloser, *ObjectInfo, error) {
	objectPath := l.objectPath(bucket, key)

	file, err := os.Open(objectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, err
	}

	info := &ObjectInfo{
		Key:          key,
		Size:         stat.Size(),
		LastModified: stat.ModTime(),
		ETag:         fmt.Sprintf("%x", md5.Sum([]byte(key))),
		ContentType:  "application/octet-stream",
	}

	// Handle HTTP Range requests
	if rangeHeader != "" {
		return l.handleRangeRequest(file, info, rangeHeader)
	}

	return file, info, nil
}

func (l *LocalStorage) handleRangeRequest(file *os.File, info *ObjectInfo, rangeHeader string) (io.ReadCloser, *ObjectInfo, error) {
	// Parse Range header: "bytes=start-end"
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		file.Close()
		return nil, nil, fmt.Errorf("invalid range header format")
	}

	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
	rangeParts := strings.Split(rangeSpec, "-")

	if len(rangeParts) != 2 {
		file.Close()
		return nil, nil, fmt.Errorf("invalid range specification")
	}

	var start, end int64
	var err error

	// Parse start
	if rangeParts[0] != "" {
		start, err = strconv.ParseInt(rangeParts[0], 10, 64)
		if err != nil {
			file.Close()
			return nil, nil, fmt.Errorf("invalid range start: %v", err)
		}
	}

	// Parse end
	if rangeParts[1] != "" {
		end, err = strconv.ParseInt(rangeParts[1], 10, 64)
		if err != nil {
			file.Close()
			return nil, nil, fmt.Errorf("invalid range end: %v", err)
		}
	} else {
		end = info.Size - 1
	}

	// Validate range
	if start < 0 || end >= info.Size || start > end {
		file.Close()
		return nil, nil, fmt.Errorf("invalid range: %d-%d for size %d", start, end, info.Size)
	}

	// Seek to start position
	_, err = file.Seek(start, io.SeekStart)
	if err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("failed to seek: %v", err)
	}

	// Create limited reader for the range
	rangeSize := end - start + 1
	limitedReader := io.LimitReader(file, rangeSize)

	// Update info for the range
	rangeInfo := *info
	rangeInfo.Size = rangeSize

	return &rangeReadCloser{
		Reader: limitedReader,
		closer: file,
	}, &rangeInfo, nil
}

type rangeReadCloser struct {
	io.Reader
	closer io.Closer
}

func (r *rangeReadCloser) Close() error {
	return r.closer.Close()
}

func (l *LocalStorage) DeleteObject(ctx context.Context, bucket, key string) error {
	objectPath := l.objectPath(bucket, key)
	return os.Remove(objectPath)
}

func (l *LocalStorage) HeadObject(ctx context.Context, bucket, key string) (*ObjectInfo, error) {
	objectPath := l.objectPath(bucket, key)

	stat, err := os.Stat(objectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}

	return &ObjectInfo{
		Key:          key,
		Size:         stat.Size(),
		LastModified: stat.ModTime(),
		ETag:         fmt.Sprintf("%x", md5.Sum([]byte(key))),
		ContentType:  "application/octet-stream",
	}, nil
}

func (l *LocalStorage) ListObjects(ctx context.Context, bucket, prefix, delimiter string, maxKeys int) ([]ObjectInfo, bool, error) {
	bucketPath := l.bucketPath(bucket)

	entries, err := os.ReadDir(bucketPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, os.ErrNotExist
		}
		return nil, false, err
	}

	var objects []ObjectInfo
	count := 0

	for _, entry := range entries {
		if count >= maxKeys {
			return objects, true, nil
		}

		if !entry.IsDir() {
			name := entry.Name()
			if prefix != "" && !strings.HasPrefix(name, prefix) {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue
			}

			objects = append(objects, ObjectInfo{
				Key:          name,
				Size:         info.Size(),
				LastModified: info.ModTime(),
				ETag:         fmt.Sprintf("%x", md5.Sum([]byte(name))),
				ContentType:  "application/octet-stream",
			})
			count++
		}
	}

	return objects, false, nil
}

func (l *LocalStorage) InitMultipartUpload(ctx context.Context, bucket, key string) (string, error) {
	uploadID := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create multipart directory
	multipartDir := filepath.Join(l.rootPath, ".multipart", bucket, uploadID)
	if err := os.MkdirAll(multipartDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create multipart directory: %v", err)
	}

	// Store metadata
	metaFile := filepath.Join(multipartDir, "metadata")
	metadata := fmt.Sprintf("bucket=%s\nkey=%s\ninitiated=%s\n", bucket, key, time.Now().Format(time.RFC3339))
	if err := os.WriteFile(metaFile, []byte(metadata), 0644); err != nil {
		return "", fmt.Errorf("failed to write metadata: %v", err)
	}

	return uploadID, nil
}

func (l *LocalStorage) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int, reader io.Reader, size int64) (string, error) {
	multipartDir := filepath.Join(l.rootPath, ".multipart", bucket, uploadID)

	// Check if multipart upload exists
	if _, err := os.Stat(multipartDir); os.IsNotExist(err) {
		return "", fmt.Errorf("multipart upload not found")
	}

	// Write part to file
	partFile := filepath.Join(multipartDir, fmt.Sprintf("part-%05d", partNumber))
	file, err := os.Create(partFile)
	if err != nil {
		return "", fmt.Errorf("failed to create part file: %v", err)
	}
	defer file.Close()

	hasher := md5.New()
	writer := io.MultiWriter(file, hasher)

	_, err = io.Copy(writer, reader)
	if err != nil {
		return "", fmt.Errorf("failed to write part: %v", err)
	}

	etag := fmt.Sprintf("%x", hasher.Sum(nil))
	return etag, nil
}

func (l *LocalStorage) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []Part) error {
	multipartDir := filepath.Join(l.rootPath, ".multipart", bucket, uploadID)

	// Check if multipart upload exists
	if _, err := os.Stat(multipartDir); os.IsNotExist(err) {
		return fmt.Errorf("multipart upload not found")
	}

	// Create final object path
	objectPath := l.objectPath(bucket, key)
	objectDir := filepath.Dir(objectPath)
	if err := os.MkdirAll(objectDir, 0755); err != nil {
		return fmt.Errorf("failed to create object directory: %v", err)
	}

	// Create final file
	finalFile, err := os.Create(objectPath)
	if err != nil {
		return fmt.Errorf("failed to create final object: %v", err)
	}
	defer finalFile.Close()

	// Concatenate parts in order
	for _, part := range parts {
		partFile := filepath.Join(multipartDir, fmt.Sprintf("part-%05d", part.PartNumber))
		partReader, err := os.Open(partFile)
		if err != nil {
			return fmt.Errorf("failed to open part %d: %v", part.PartNumber, err)
		}

		_, err = io.Copy(finalFile, partReader)
		partReader.Close()
		if err != nil {
			return fmt.Errorf("failed to copy part %d: %v", part.PartNumber, err)
		}
	}

	// Clean up multipart directory
	os.RemoveAll(multipartDir)

	return nil
}

func (l *LocalStorage) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	multipartDir := filepath.Join(l.rootPath, ".multipart", bucket, uploadID)
	return os.RemoveAll(multipartDir)
}

func (l *LocalStorage) ListMultipartUploads(ctx context.Context, bucket string) ([]MultipartUpload, error) {
	multipartRoot := filepath.Join(l.rootPath, ".multipart", bucket)

	entries, err := os.ReadDir(multipartRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []MultipartUpload{}, nil
		}
		return nil, err
	}

	var uploads []MultipartUpload
	for _, entry := range entries {
		if entry.IsDir() {
			metaFile := filepath.Join(multipartRoot, entry.Name(), "metadata")
			metaData, err := os.ReadFile(metaFile)
			if err != nil {
				continue
			}

			// Parse metadata
			lines := strings.Split(string(metaData), "\n")
			var key, initiated string
			for _, line := range lines {
				if strings.HasPrefix(line, "key=") {
					key = strings.TrimPrefix(line, "key=")
				} else if strings.HasPrefix(line, "initiated=") {
					initiated = strings.TrimPrefix(line, "initiated=")
				}
			}

			initiatedTime, _ := time.Parse(time.RFC3339, initiated)
			uploads = append(uploads, MultipartUpload{
				UploadID:  entry.Name(),
				Key:       key,
				Initiated: initiatedTime,
			})
		}
	}

	return uploads, nil
}
