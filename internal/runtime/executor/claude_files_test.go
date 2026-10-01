package executor

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/config"
)

func TestUploadClaudeFilePostsMultipartAndReturnsID(t *testing.T) {
	var gotMethod, gotPath, gotBeta, gotAuth, gotAPIKey, gotContentType, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBeta = r.Header.Get("anthropic-beta")
		gotAuth = r.Header.Get("Authorization")
		gotAPIKey = r.Header.Get("x-api-key")
		gotContentType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"file","id":"file-abc","filename":"a.pdf","mime_type":"application/pdf"}`))
	}))
	defer server.Close()

	auth := &cliproxyauth.Auth{Attributes: map[string]string{"api_key": "key-123", "base_url": server.URL}}
	fileID, err := UploadClaudeFile(context.Background(), server.Client(), &config.Config{}, auth, "a.pdf", "application/pdf", []byte("pdf-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if fileID != "file-abc" {
		t.Fatalf("fileID = %q", fileID)
	}
	if gotMethod != http.MethodPost || gotPath != "/v1/files" {
		t.Fatalf("request = %s %s", gotMethod, gotPath)
	}
	if !strings.Contains(gotBeta, claudeFilesAPIBeta) {
		t.Fatalf("anthropic-beta = %q", gotBeta)
	}
	// A non-Anthropic base is reached with a bearer credential, matching the Messages path.
	if gotAuth != "Bearer key-123" || gotAPIKey != "" {
		t.Fatalf("auth = %q, x-api-key = %q", gotAuth, gotAPIKey)
	}
	if !strings.HasPrefix(gotContentType, "multipart/form-data") {
		t.Fatalf("Content-Type = %q", gotContentType)
	}
	if !strings.Contains(gotBody, `name="file"`) || !strings.Contains(gotBody, "pdf-bytes") {
		t.Fatalf("multipart body = %q", gotBody)
	}
}

func TestUploadClaudeFileReportsUpstreamRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"bad file"}}`))
	}))
	defer server.Close()

	auth := &cliproxyauth.Auth{Attributes: map[string]string{"api_key": "key-123", "base_url": server.URL}}
	_, err := UploadClaudeFile(context.Background(), server.Client(), &config.Config{}, auth, "a.pdf", "application/pdf", []byte("pdf-bytes"))
	if err == nil || !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "bad file") {
		t.Fatalf("err = %v", err)
	}
}

func TestUploadClaudeFileRequiresCredential(t *testing.T) {
	_, err := UploadClaudeFile(context.Background(), nil, &config.Config{}, &cliproxyauth.Auth{}, "a.pdf", "application/pdf", []byte("pdf-bytes"))
	if err == nil || !strings.Contains(err.Error(), "claude credential") {
		t.Fatalf("err = %v", err)
	}
}

func TestUploadClaudeFileCarriesUpstreamStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_, _ = w.Write([]byte(`{"error":{"message":"file too large"}}`))
	}))
	defer server.Close()

	auth := &cliproxyauth.Auth{Attributes: map[string]string{"api_key": "key-123", "base_url": server.URL}}
	_, err := UploadClaudeFile(context.Background(), server.Client(), &config.Config{}, auth, "a.pdf", "application/pdf", []byte("pdf-bytes"))
	var uploadErr *ClaudeFileUploadError
	if !errors.As(err, &uploadErr) {
		t.Fatalf("err = %v", err)
	}
	if uploadErr.Status != http.StatusRequestEntityTooLarge || !strings.Contains(uploadErr.Message, "file too large") {
		t.Fatalf("uploadErr = %#v", uploadErr)
	}
}
