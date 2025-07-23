package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexerm/porterfs/internal/config"
)

func TestAuthenticator(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
		},
	}

	auth := New(cfg)

	t.Run("MissingAuthHeader", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		err := auth.Authenticate(req)
		if err == nil {
			t.Error("Expected error for missing auth header")
		}
		if !strings.Contains(err.Error(), "missing authorization header") {
			t.Errorf("Expected 'missing authorization header' error, got: %v", err)
		}
	})

	t.Run("UnsupportedAuthMethod", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Basic dGVzdDp0ZXN0")
		err := auth.Authenticate(req)
		if err == nil {
			t.Error("Expected error for unsupported auth method")
		}
		if !strings.Contains(err.Error(), "unsupported authorization method") {
			t.Errorf("Expected 'unsupported authorization method' error, got: %v", err)
		}
	})

	t.Run("InvalidAuthHeaderFormat", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "AWS4-HMAC-SHA256")
		err := auth.Authenticate(req)
		if err == nil {
			t.Error("Expected error for invalid auth header format")
		}
		if !strings.Contains(err.Error(), "invalid authorization header format") {
			t.Errorf("Expected 'invalid authorization header format' error, got: %v", err)
		}
	})

	t.Run("MissingCredentialParts", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=invalid")
		err := auth.Authenticate(req)
		if err == nil {
			t.Error("Expected error for missing credential parts")
		}
		if !strings.Contains(err.Error(), "missing required authorization components") {
			t.Errorf("Expected 'missing required authorization components' error, got: %v", err)
		}
	})

	t.Run("InvalidCredentialFormat", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=invalid/format, SignedHeaders=host, Signature=test")
		req.Header.Set("X-Amz-Date", "20230101T000000Z")
		err := auth.Authenticate(req)
		if err == nil {
			t.Error("Expected error for invalid credential format")
		}
		if !strings.Contains(err.Error(), "invalid credential format") {
			t.Errorf("Expected 'invalid credential format' error, got: %v", err)
		}
	})

	t.Run("InvalidAccessKey", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=wrong-key/20230101/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=test")
		req.Header.Set("X-Amz-Date", "20230101T000000Z")
		err := auth.Authenticate(req)
		if err == nil {
			t.Error("Expected error for invalid access key")
		}
		if !strings.Contains(err.Error(), "invalid access key") {
			t.Errorf("Expected 'invalid access key' error, got: %v", err)
		}
	})
}

func TestAuthMiddleware(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
		},
	}

	auth := New(cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	middleware := auth.AuthMiddleware(handler)

	t.Run("UnauthorizedRequest", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("AuthorizedRequest", func(t *testing.T) {
		// This test would require a properly signed request
		// For now, we'll just test that the middleware structure works
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		// Should be unauthorized without proper signature
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})
}

func TestCreateCanonicalRequest(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
		},
	}

	auth := New(cfg)

	req := httptest.NewRequest("GET", "/bucket/object", nil)
	req.Host = "localhost:9000"
	req.Header.Set("Host", "localhost:9000")
	req.Header.Set("X-Amz-Date", "20230101T000000Z")
	req.Header.Set("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")

	canonical := auth.CreateCanonicalRequest(req, "host;x-amz-content-sha256;x-amz-date")

	expectedParts := []string{
		"GET",
		"/bucket/object",
		"",
		"host:localhost:9000",
		"x-amz-content-sha256:UNSIGNED-PAYLOAD",
		"x-amz-date:20230101T000000Z",
		"",
		"host;x-amz-content-sha256;x-amz-date",
		"UNSIGNED-PAYLOAD",
	}

	expected := strings.Join(expectedParts, "\n")
	if canonical != expected {
		t.Errorf("Canonical request mismatch.\nExpected:\n%s\nGot:\n%s", expected, canonical)
	}
}
