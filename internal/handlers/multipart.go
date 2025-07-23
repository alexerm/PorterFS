package handlers

import (
	"encoding/xml"
	"net/http"
	"strconv"
	"time"

	"github.com/alexerm/porterfs/internal/storage"
	"github.com/go-chi/chi/v5"
)

type InitiateMultipartUploadResult struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	UploadID string   `xml:"UploadId"`
}

type CompleteMultipartUploadRequest struct {
	XMLName xml.Name                `xml:"CompleteMultipartUpload"`
	Parts   []CompleteMultipartPart `xml:"Part"`
}

type CompleteMultipartPart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

type CompleteMultipartUploadResult struct {
	XMLName  xml.Name `xml:"CompleteMultipartUploadResult"`
	Location string   `xml:"Location"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	ETag     string   `xml:"ETag"`
}

type ListMultipartUploadsResult struct {
	XMLName xml.Name                   `xml:"ListMultipartUploadsResult"`
	Bucket  string                     `xml:"Bucket"`
	Uploads []ListMultipartUploadEntry `xml:"Upload"`
}

type ListMultipartUploadEntry struct {
	Key       string    `xml:"Key"`
	UploadID  string    `xml:"UploadId"`
	Initiated time.Time `xml:"Initiated"`
}

func (h *Handler) InitiateMultipartUpload(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	object := chi.URLParam(r, "object")

	if bucket == "" || object == "" {
		http.Error(w, "bucket and object required", http.StatusBadRequest)
		return
	}

	uploadID, err := h.storage.InitMultipartUpload(r.Context(), bucket, object)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := InitiateMultipartUploadResult{
		Bucket:   bucket,
		Key:      object,
		UploadID: uploadID,
	}

	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(result)
}

func (h *Handler) UploadPart(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	object := chi.URLParam(r, "object")
	uploadID := r.URL.Query().Get("uploadId")
	partNumberStr := r.URL.Query().Get("partNumber")

	if bucket == "" || object == "" || uploadID == "" || partNumberStr == "" {
		http.Error(w, "bucket, object, uploadId, and partNumber required", http.StatusBadRequest)
		return
	}

	partNumber, err := strconv.Atoi(partNumberStr)
	if err != nil {
		http.Error(w, "invalid partNumber", http.StatusBadRequest)
		return
	}

	contentLengthStr := r.Header.Get("Content-Length")
	contentLength := int64(-1)
	if contentLengthStr != "" {
		if cl, err := strconv.ParseInt(contentLengthStr, 10, 64); err == nil {
			contentLength = cl
		}
	}

	etag, err := h.storage.UploadPart(r.Context(), bucket, object, uploadID, partNumber, r.Body, contentLength)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) CompleteMultipartUpload(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	object := chi.URLParam(r, "object")
	uploadID := r.URL.Query().Get("uploadId")

	if bucket == "" || object == "" || uploadID == "" {
		http.Error(w, "bucket, object, and uploadId required", http.StatusBadRequest)
		return
	}

	var req CompleteMultipartUploadRequest
	if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Convert to storage parts
	parts := make([]storage.Part, len(req.Parts))
	for i, part := range req.Parts {
		parts[i] = storage.Part{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		}
	}

	err := h.storage.CompleteMultipartUpload(r.Context(), bucket, object, uploadID, parts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := CompleteMultipartUploadResult{
		Location: "/" + bucket + "/" + object,
		Bucket:   bucket,
		Key:      object,
		ETag:     "\"" + uploadID + "\"", // Simplified ETag
	}

	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(result)
}

func (h *Handler) AbortMultipartUpload(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	object := chi.URLParam(r, "object")
	uploadID := r.URL.Query().Get("uploadId")

	if bucket == "" || object == "" || uploadID == "" {
		http.Error(w, "bucket, object, and uploadId required", http.StatusBadRequest)
		return
	}

	err := h.storage.AbortMultipartUpload(r.Context(), bucket, object, uploadID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListMultipartUploads(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")

	if bucket == "" {
		http.Error(w, "bucket required", http.StatusBadRequest)
		return
	}

	uploads, err := h.storage.ListMultipartUploads(r.Context(), bucket)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entries := make([]ListMultipartUploadEntry, len(uploads))
	for i, upload := range uploads {
		entries[i] = ListMultipartUploadEntry{
			Key:       upload.Key,
			UploadID:  upload.UploadID,
			Initiated: upload.Initiated,
		}
	}

	result := ListMultipartUploadsResult{
		Bucket:  bucket,
		Uploads: entries,
	}

	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(result)
}
