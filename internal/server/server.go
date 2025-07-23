package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/alexerm/porterfs/internal/auth"
	"github.com/alexerm/porterfs/internal/config"
	"github.com/alexerm/porterfs/internal/handlers"
	"github.com/alexerm/porterfs/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	config  *config.Config
	storage storage.Storage
	server  *http.Server
}

func New(cfg *config.Config) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	store, err := storage.NewLocalStorage(cfg.Storage.RootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	return &Server{
		config:  cfg,
		storage: store,
	}, nil
}

func (s *Server) ListenAndServe(addr string) error {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(60 * time.Second))

	h := handlers.New(s.storage, s.config)
	authenticator := auth.New(s.config)

	// Test endpoint without authentication (must come before bucket routes)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok", "message": "PorterFS server is running"}`))
	})

	// Test storage endpoints without authentication (must come before bucket routes)
	r.Route("/test-storage", func(r chi.Router) {
		r.Post("/bucket/{bucket}", func(w http.ResponseWriter, r *http.Request) {
			bucket := chi.URLParam(r, "bucket")
			if err := s.storage.CreateBucket(r.Context(), bucket); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "ok", "message": "Bucket created", "bucket": "` + bucket + `"}`))
		})

		r.Get("/bucket/{bucket}", func(w http.ResponseWriter, r *http.Request) {
			bucket := chi.URLParam(r, "bucket")
			objects, _, err := s.storage.ListObjects(r.Context(), bucket, "", "", 1000)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "ok", "bucket": "` + bucket + `", "objects": ` + fmt.Sprintf("%d", len(objects)) + `}`))
		})

		r.Put("/bucket/{bucket}/object/{object:.*}", func(w http.ResponseWriter, r *http.Request) {
			bucket := chi.URLParam(r, "bucket")
			object := chi.URLParam(r, "object")

			// Read body
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			reader := bytes.NewReader(body)
			if err := s.storage.PutObject(r.Context(), bucket, object, reader, int64(len(body)), "application/octet-stream"); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "ok", "message": "Object uploaded", "bucket": "` + bucket + `", "object": "` + object + `"}`))
		})

		r.Get("/bucket/{bucket}/object/{object:.*}", func(w http.ResponseWriter, r *http.Request) {
			bucket := chi.URLParam(r, "bucket")
			object := chi.URLParam(r, "object")

			reader, _, err := s.storage.GetObject(r.Context(), bucket, object, "")
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			defer reader.Close()

			data, err := io.ReadAll(reader)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			w.Write(data)
		})
	})

	// S3 API routes with authentication
	r.Route("/", func(r chi.Router) {
		log.Printf("DEBUG: Applying authentication middleware to S3 routes")
		r.Use(authenticator.AuthMiddleware)
		r.Get("/", h.ListBuckets)
		r.Route("/{bucket}", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				// Check for multipart uploads query
				if r.URL.Query().Get("uploads") != "" {
					h.ListMultipartUploads(w, r)
					return
				}
				h.ListObjects(w, r)
			})
			r.Put("/", h.CreateBucket)
			r.Delete("/", h.DeleteBucket)

			r.Route("/{object:.*}", func(r chi.Router) {
				r.Get("/", h.GetObject)
				r.Put("/", func(w http.ResponseWriter, r *http.Request) {
					// Check for multipart upload operations
					if uploadID := r.URL.Query().Get("uploadId"); uploadID != "" {
						if partNumber := r.URL.Query().Get("partNumber"); partNumber != "" {
							h.UploadPart(w, r)
							return
						}
						h.CompleteMultipartUpload(w, r)
						return
					}
					h.PutObject(w, r)
				})
				r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
					if uploadID := r.URL.Query().Get("uploadId"); uploadID != "" {
						h.AbortMultipartUpload(w, r)
						return
					}
					h.DeleteObject(w, r)
				})
				r.Head("/", h.HeadObject)
				r.Post("/", func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Query().Get("uploads") != "" {
						h.InitiateMultipartUpload(w, r)
						return
					}
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				})
			})
		})
	})

	s.server = &http.Server{
		Addr:    addr,
		Handler: r,
	}

	if s.config.Server.TLS.Enabled {
		log.Printf("Server starting with TLS on %s", addr)
		return s.server.ListenAndServeTLS(s.config.Server.TLS.CertFile, s.config.Server.TLS.KeyFile)
	} else {
		log.Printf("Server starting on %s", addr)
		return s.server.ListenAndServe()
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}
