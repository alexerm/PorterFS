package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/alexerm/porterfs/internal/config"
)

type Authenticator struct {
	config *config.Config
}

func New(config *config.Config) *Authenticator {
	return &Authenticator{config: config}
}

func (a *Authenticator) Authenticate(r *http.Request) error {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		log.Printf("DEBUG: Missing authorization header\n")
		return fmt.Errorf("missing authorization header")
	}

	if !strings.HasPrefix(authHeader, "AWS4-HMAC-SHA256") {
		log.Printf("DEBUG: Unsupported authorization method: %s\n", authHeader)
		return fmt.Errorf("unsupported authorization method")
	}

	log.Printf("DEBUG: Processing AWS4-HMAC-SHA256 authorization\n")
	return a.validateV4Signature(r, authHeader)
}

func (a *Authenticator) validateV4Signature(r *http.Request, authHeader string) error {
	// Expected format: AWS4-HMAC-SHA256 Credential=..., SignedHeaders=..., Signature=...
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		log.Printf("DEBUG: Invalid authorization header format\n")
		return fmt.Errorf("invalid authorization header format")
	}

	// Skip the "AWS4-HMAC-SHA256" part and parse the rest
	components := parts[1]

	credentialPart := ""
	signaturePart := ""
	signedHeadersPart := ""

	// Split by comma and parse each component
	for _, component := range strings.Split(components, ",") {
		component = strings.TrimSpace(component)
		if strings.HasPrefix(component, "Credential=") {
			credentialPart = strings.TrimPrefix(component, "Credential=")
		} else if strings.HasPrefix(component, "Signature=") {
			signaturePart = strings.TrimPrefix(component, "Signature=")
		} else if strings.HasPrefix(component, "SignedHeaders=") {
			signedHeadersPart = strings.TrimPrefix(component, "SignedHeaders=")
		}
	}

	log.Printf("DEBUG: Parsed components - Credential: %s, Signature: %s, SignedHeaders: %s\n", credentialPart, signaturePart, signedHeadersPart)

	if credentialPart == "" || signaturePart == "" || signedHeadersPart == "" {
		log.Printf("DEBUG: Missing required authorization components\n")
		return fmt.Errorf("missing required authorization components")
	}

	credParts := strings.Split(credentialPart, "/")
	if len(credParts) != 5 {
		log.Printf("DEBUG: Invalid credential format, expected 5 parts, got %d\n", len(credParts))
		return fmt.Errorf("invalid credential format")
	}

	accessKey := credParts[0]
	if accessKey != a.config.Auth.AccessKey {
		log.Printf("DEBUG: Access key mismatch. Expected: %s, Got: %s\n", a.config.Auth.AccessKey, accessKey)
		return fmt.Errorf("invalid access key")
	}

	expectedSignature, err := a.calculateSignature(r, credentialPart, signedHeadersPart)
	if err != nil {
		log.Printf("DEBUG: Failed to calculate signature: %v\n", err)
		return fmt.Errorf("failed to calculate signature: %w", err)
	}

	if signaturePart != expectedSignature {
		log.Printf("DEBUG: Signature mismatch. Expected: %s, Got: %s\n", expectedSignature, signaturePart)
		return fmt.Errorf("signature mismatch")
	}

	log.Printf("DEBUG: Authentication successful\n")
	return nil
}

func (a *Authenticator) calculateSignature(r *http.Request, credential, signedHeaders string) (string, error) {
	canonicalRequest := a.CreateCanonicalRequest(r, signedHeaders)
	log.Printf("DEBUG: Canonical request:\n%s", canonicalRequest)

	credParts := strings.Split(credential, "/")
	dateStamp := credParts[1]
	region := credParts[2]
	service := credParts[3]

	algorithm := "AWS4-HMAC-SHA256"
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, region, service)

	amzDate := r.Header.Get("X-Amz-Date")

	stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s",
		algorithm,
		amzDate,
		credentialScope,
		sha256Hash(canonicalRequest))

	log.Printf("DEBUG: String to sign:\n%s", stringToSign)

	signingKey := a.getSigningKey(a.config.Auth.SecretKey, dateStamp, region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	return signature, nil
}

func (a *Authenticator) CreateCanonicalRequest(r *http.Request, signedHeaders string) string {
	method := r.Method
	uri := r.URL.Path
	if uri == "" {
		uri = "/"
	}

	query := r.URL.RawQuery
	if query != "" {
		values, _ := url.ParseQuery(query)
		var keys []string
		for k := range values {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var parts []string
		for _, k := range keys {
			for _, v := range values[k] {
				parts = append(parts, fmt.Sprintf("%s=%s", url.QueryEscape(k), url.QueryEscape(v)))
			}
		}
		query = strings.Join(parts, "&")
	}

	headerNames := strings.Split(signedHeaders, ";")
	sort.Strings(headerNames)

	var canonicalHeaders []string
	for _, name := range headerNames {
		var value string
		if strings.ToLower(name) == "host" {
			// Use r.Host instead of r.Header.Get("Host")
			value = r.Host
		} else {
			value = r.Header.Get(name)
		}
		canonicalHeaders = append(canonicalHeaders, fmt.Sprintf("%s:%s", strings.ToLower(name), strings.TrimSpace(value)))
	}

	payloadHash := r.Header.Get("X-Amz-Content-Sha256")
	if payloadHash == "" {
		payloadHash = "UNSIGNED-PAYLOAD"
	}

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n\n%s\n%s",
		method,
		uri,
		query,
		strings.Join(canonicalHeaders, "\n"),
		signedHeaders,
		payloadHash)

	return canonicalRequest
}

func (a *Authenticator) getSigningKey(secretKey, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "aws4_request")
	return kSigning
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func sha256Hash(data string) string {
	h := sha256.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func (a *Authenticator) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("DEBUG: AuthMiddleware called for %s %s", r.Method, r.URL.Path)
		if err := a.Authenticate(r); err != nil {
			log.Printf("DEBUG: Authentication failed: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		log.Printf("DEBUG: Authentication successful for %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
