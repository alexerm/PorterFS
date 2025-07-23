package handlers

import (
	"encoding/xml"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/alexerm/porterfs/internal/config"
	"github.com/alexerm/porterfs/internal/storage"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	storage storage.Storage
	config  *config.Config
}

func New(storage storage.Storage, config *config.Config) *Handler {
	return &Handler{
		storage: storage,
		config:  config,
	}
}

type ListBucketsResult struct {
	XMLName xml.Name `xml:"ListAllMyBucketsResult"`
	Owner   Owner    `xml:"Owner"`
	Buckets Buckets  `xml:"Buckets"`
}

type Owner struct {
	ID          string `xml:"ID"`
	DisplayName string `xml:"DisplayName"`
}

type Buckets struct {
	Bucket []Bucket `xml:"Bucket"`
}

type Bucket struct {
	Name         string    `xml:"Name"`
	CreationDate time.Time `xml:"CreationDate"`
}

type ListObjectsV2Result struct {
	XMLName               xml.Name `xml:"ListBucketResult"`
	Name                  string   `xml:"Name"`
	Prefix                string   `xml:"Prefix"`
	KeyCount              int      `xml:"KeyCount"`
	MaxKeys               int      `xml:"MaxKeys"`
	IsTruncated           bool     `xml:"IsTruncated"`
	ContinuationToken     string   `xml:"ContinuationToken,omitempty"`
	NextContinuationToken string   `xml:"NextContinuationToken,omitempty"`
	Contents              []Object `xml:"Contents"`
}

type Object struct {
	Key          string    `xml:"Key"`
	LastModified time.Time `xml:"LastModified"`
	ETag         string    `xml:"ETag"`
	Size         int64     `xml:"Size"`
	StorageClass string    `xml:"StorageClass"`
}

func (h *Handler) ListBuckets(w http.ResponseWriter, r *http.Request) {
	buckets, err := h.storage.ListBuckets(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := ListBucketsResult{
		Owner: Owner{
			ID:          "porter",
			DisplayName: "porter",
		},
		Buckets: Buckets{
			Bucket: make([]Bucket, len(buckets)),
		},
	}

	for i, bucket := range buckets {
		result.Buckets.Bucket[i] = Bucket{
			Name:         bucket,
			CreationDate: time.Now(),
		}
	}

	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(result)
}

func (h *Handler) CreateBucket(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	if bucket == "" {
		http.Error(w, "bucket name required", http.StatusBadRequest)
		return
	}

	err := h.storage.CreateBucket(r.Context(), bucket)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) DeleteBucket(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	if bucket == "" {
		http.Error(w, "bucket name required", http.StatusBadRequest)
		return
	}

	err := h.storage.DeleteBucket(r.Context(), bucket)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListObjects(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	if bucket == "" {
		http.Error(w, "bucket name required", http.StatusBadRequest)
		return
	}

	query := r.URL.Query()

	if query.Get("list-type") == "2" {
		h.listObjectsV2(w, r, bucket)
		return
	}

	h.listObjectsV1(w, r, bucket)
}

func (h *Handler) listObjectsV2(w http.ResponseWriter, r *http.Request, bucket string) {
	query := r.URL.Query()
	prefix := query.Get("prefix")
	delimiter := query.Get("delimiter")
	maxKeysStr := query.Get("max-keys")

	maxKeys := 1000
	if maxKeysStr != "" {
		if mk, err := strconv.Atoi(maxKeysStr); err == nil && mk > 0 {
			maxKeys = mk
		}
	}

	objects, isTruncated, err := h.storage.ListObjects(r.Context(), bucket, prefix, delimiter, maxKeys)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := ListObjectsV2Result{
		Name:        bucket,
		Prefix:      prefix,
		KeyCount:    len(objects),
		MaxKeys:     maxKeys,
		IsTruncated: isTruncated,
		Contents:    make([]Object, len(objects)),
	}

	for i, obj := range objects {
		result.Contents[i] = Object{
			Key:          obj.Key,
			LastModified: obj.LastModified,
			ETag:         obj.ETag,
			Size:         obj.Size,
			StorageClass: "STANDARD",
		}
	}

	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(result)
}

func (h *Handler) listObjectsV1(w http.ResponseWriter, r *http.Request, bucket string) {
	query := r.URL.Query()
	prefix := query.Get("prefix")
	delimiter := query.Get("delimiter")
	maxKeysStr := query.Get("max-keys")

	maxKeys := 1000
	if maxKeysStr != "" {
		if mk, err := strconv.Atoi(maxKeysStr); err == nil && mk > 0 {
			maxKeys = mk
		}
	}

	objects, isTruncated, err := h.storage.ListObjects(r.Context(), bucket, prefix, delimiter, maxKeys)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type ListBucketResult struct {
		XMLName     xml.Name `xml:"ListBucketResult"`
		Name        string   `xml:"Name"`
		Prefix      string   `xml:"Prefix"`
		MaxKeys     int      `xml:"MaxKeys"`
		IsTruncated bool     `xml:"IsTruncated"`
		Contents    []Object `xml:"Contents"`
	}

	result := ListBucketResult{
		Name:        bucket,
		Prefix:      prefix,
		MaxKeys:     maxKeys,
		IsTruncated: isTruncated,
		Contents:    make([]Object, len(objects)),
	}

	for i, obj := range objects {
		result.Contents[i] = Object{
			Key:          obj.Key,
			LastModified: obj.LastModified,
			ETag:         obj.ETag,
			Size:         obj.Size,
			StorageClass: "STANDARD",
		}
	}

	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(result)
}
func (h *Handler) GetObject(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	object := chi.URLParam(r, "object")

	if bucket == "" || object == "" {
		http.Error(w, "bucket and object required", http.StatusBadRequest)
		return
	}

	rangeHeader := r.Header.Get("Range")

	reader, info, err := h.storage.GetObject(r.Context(), bucket, object, rangeHeader)
	if err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "Object not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("ETag", info.ETag)
	w.Header().Set("Last-Modified", info.LastModified.Format(http.TimeFormat))
	w.Header().Set("Accept-Ranges", "bytes")

	// Handle range requests
	if rangeHeader != "" {
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	}

	io.Copy(w, reader)
}
func (h *Handler) PutObject(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	object := chi.URLParam(r, "object")

	if bucket == "" || object == "" {
		http.Error(w, "bucket and object required", http.StatusBadRequest)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	contentLengthStr := r.Header.Get("Content-Length")
	contentLength := int64(-1)
	if contentLengthStr != "" {
		if cl, err := strconv.ParseInt(contentLengthStr, 10, 64); err == nil {
			contentLength = cl
		}
	}

	err := h.storage.PutObject(r.Context(), bucket, object, r.Body, contentLength, contentType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) DeleteObject(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	object := chi.URLParam(r, "object")

	if bucket == "" || object == "" {
		http.Error(w, "bucket and object required", http.StatusBadRequest)
		return
	}

	err := h.storage.DeleteObject(r.Context(), bucket, object)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) HeadObject(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	object := chi.URLParam(r, "object")

	if bucket == "" || object == "" {
		http.Error(w, "bucket and object required", http.StatusBadRequest)
		return
	}

	info, err := h.storage.HeadObject(r.Context(), bucket, object)
	if err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "Object not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	w.Header().Set("ETag", info.ETag)
	w.Header().Set("Last-Modified", info.LastModified.Format(http.TimeFormat))

	w.WriteHeader(http.StatusOK)
}
