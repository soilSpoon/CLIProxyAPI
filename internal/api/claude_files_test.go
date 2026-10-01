package api

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/cache"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/runtime/executor"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/api/handlers"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/auth"
)

func newClaudeFileTestServer(manager *auth.Manager) *Server {
	return &Server{handlers: &handlers.BaseAPIHandler{AuthManager: manager}}
}

func multipartUploadBody(t *testing.T, field, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, errPart := writer.CreateFormFile(field, filename)
	if errPart != nil {
		t.Fatal(errPart)
	}
	if _, errWrite := part.Write(data); errWrite != nil {
		t.Fatal(errWrite)
	}
	if errClose := writer.Close(); errClose != nil {
		t.Fatal(errClose)
	}
	return body, writer.FormDataContentType()
}

func postClaudeFile(server *Server, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/v1/files", server.ClaudeFileUpload)
	req := httptest.NewRequest(http.MethodPost, "/v1/files", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func activeClaudeManager(t *testing.T) *auth.Manager {
	t.Helper()
	manager := auth.NewManager(nil, nil, nil)
	if _, errRegister := manager.Register(context.Background(), &auth.Auth{ID: "claude-1", Provider: "claude", Status: auth.StatusActive}); errRegister != nil {
		t.Fatal(errRegister)
	}
	return manager
}

func TestClaudeFileUploadStoresBytes(t *testing.T) {
	previous := uploadClaudeFileToAnthropic
	uploadClaudeFileToAnthropic = func(_ context.Context, _ *http.Client, _ *config.Config, _ *auth.Auth, _, _ string, _ []byte) (string, error) {
		return "file-up-1", nil
	}
	t.Cleanup(func() {
		uploadClaudeFileToAnthropic = previous
		cache.DeleteClaudeFileContent("file-up-1")
	})

	body, contentType := multipartUploadBody(t, "file", "a.pdf", []byte("pdf-bytes"))
	rec := postClaudeFile(newClaudeFileTestServer(activeClaudeManager(t)), body, contentType)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	stored, ok := cache.GetClaudeFileContent("file-up-1")
	if !ok || string(stored.Data) != "pdf-bytes" {
		t.Fatalf("cache = %#v ok=%v", stored, ok)
	}
	if !strings.Contains(rec.Body.String(), "file-up-1") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestClaudeFileUploadRequiresAFile(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if errClose := writer.Close(); errClose != nil {
		t.Fatal(errClose)
	}
	rec := postClaudeFile(newClaudeFileTestServer(activeClaudeManager(t)), body, writer.FormDataContentType())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestClaudeFileUploadNeedsAClaudeCredential(t *testing.T) {
	body, contentType := multipartUploadBody(t, "file", "a.pdf", []byte("pdf-bytes"))
	rec := postClaudeFile(newClaudeFileTestServer(auth.NewManager(nil, nil, nil)), body, contentType)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestClaudeFileUploadSkipsUnavailableCredentials(t *testing.T) {
	manager := auth.NewManager(nil, nil, nil)
	for _, candidate := range []*auth.Auth{
		{ID: "claude-down", Provider: "claude", Status: auth.StatusActive, Unavailable: true},
		{ID: "claude-up", Provider: "claude", Status: auth.StatusActive},
	} {
		if _, errRegister := manager.Register(context.Background(), candidate); errRegister != nil {
			t.Fatal(errRegister)
		}
	}

	var used string
	previous := uploadClaudeFileToAnthropic
	uploadClaudeFileToAnthropic = func(_ context.Context, _ *http.Client, _ *config.Config, credential *auth.Auth, _, _ string, _ []byte) (string, error) {
		used = credential.ID
		return "file-up-skip", nil
	}
	t.Cleanup(func() {
		uploadClaudeFileToAnthropic = previous
		cache.DeleteClaudeFileContent("file-up-skip")
	})

	body, contentType := multipartUploadBody(t, "file", "a.pdf", []byte("pdf-bytes"))
	rec := postClaudeFile(newClaudeFileTestServer(manager), body, contentType)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if used != "claude-up" {
		t.Fatalf("upload used %q", used)
	}
}

func TestClaudeFileUploadRejectsAllUnavailableCredentials(t *testing.T) {
	manager := auth.NewManager(nil, nil, nil)
	if _, errRegister := manager.Register(context.Background(), &auth.Auth{ID: "claude-down", Provider: "claude", Status: auth.StatusActive, Unavailable: true}); errRegister != nil {
		t.Fatal(errRegister)
	}
	body, contentType := multipartUploadBody(t, "file", "a.pdf", []byte("pdf-bytes"))
	rec := postClaudeFile(newClaudeFileTestServer(manager), body, contentType)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func seamReturningUploadError(t *testing.T, status int, message string) {
	t.Helper()
	previous := uploadClaudeFileToAnthropic
	uploadClaudeFileToAnthropic = func(_ context.Context, _ *http.Client, _ *config.Config, _ *auth.Auth, _, _ string, _ []byte) (string, error) {
		return "", &executor.ClaudeFileUploadError{Status: status, Message: message}
	}
	t.Cleanup(func() { uploadClaudeFileToAnthropic = previous })
}

func TestClaudeFileUploadPassesUpstreamClientError(t *testing.T) {
	seamReturningUploadError(t, http.StatusRequestEntityTooLarge, "file too large")
	body, contentType := multipartUploadBody(t, "file", "a.pdf", []byte("pdf-bytes"))
	rec := postClaudeFile(newClaudeFileTestServer(activeClaudeManager(t)), body, contentType)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "file too large") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestClaudeFileUploadReportsUpstreamServerErrorAsBadGateway(t *testing.T) {
	seamReturningUploadError(t, http.StatusInternalServerError, "upstream is down")
	body, contentType := multipartUploadBody(t, "file", "a.pdf", []byte("pdf-bytes"))
	rec := postClaudeFile(newClaudeFileTestServer(activeClaudeManager(t)), body, contentType)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}
