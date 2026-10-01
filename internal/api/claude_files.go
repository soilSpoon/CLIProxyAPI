package api

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/cache"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/runtime/executor"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/auth"
)

// claudeFileUploadMaxBytes caps one upload so a single file cannot exhaust the cache budget.
const claudeFileUploadMaxBytes = cache.ClaudeFileContentMaxEntryBytes

// uploadClaudeFileToAnthropic is a seam so the route can be tested without an upstream.
var uploadClaudeFileToAnthropic = executor.UploadClaudeFile

func writeClaudeFileError(c *gin.Context, status int, kind, message string) {
	c.JSON(status, gin.H{"type": "error", "error": gin.H{"type": kind, "message": message}})
}

// pickClaudeFileAuth returns the first usable Claude credential, or nil when none is loaded.
func (s *Server) pickClaudeFileAuth() *auth.Auth {
	if s == nil || s.handlers == nil || s.handlers.AuthManager == nil {
		return nil
	}
	now := time.Now()
	for _, candidate := range s.handlers.AuthManager.List() {
		if candidate == nil || candidate.Disabled || candidate.Unavailable || candidate.Status != auth.StatusActive {
			continue
		}
		if !candidate.NextRetryAfter.IsZero() && candidate.NextRetryAfter.After(now) {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(candidate.Provider), "claude") {
			return candidate
		}
	}
	return nil
}

// ClaudeFileUpload accepts a Claude Files API upload, forwards it to Anthropic and keeps
// the bytes so a later container_upload block can be inlined for another provider.
func (s *Server) ClaudeFileUpload(c *gin.Context) {
	file, fileHeader, errForm := c.Request.FormFile("file")
	if errForm != nil {
		writeClaudeFileError(c, http.StatusBadRequest, "invalid_request_error", "missing multipart field: file")
		return
	}
	defer func() { _ = file.Close() }()

	data, errRead := io.ReadAll(io.LimitReader(file, claudeFileUploadMaxBytes+1))
	if errRead != nil {
		writeClaudeFileError(c, http.StatusBadRequest, "invalid_request_error", "failed to read the uploaded file")
		return
	}
	if len(data) == 0 {
		writeClaudeFileError(c, http.StatusBadRequest, "invalid_request_error", "uploaded file is empty")
		return
	}
	if len(data) > claudeFileUploadMaxBytes {
		writeClaudeFileError(c, http.StatusRequestEntityTooLarge, "invalid_request_error", "uploaded file exceeds the proxy limit")
		return
	}

	credential := s.pickClaudeFileAuth()
	if credential == nil {
		writeClaudeFileError(c, http.StatusServiceUnavailable, "api_error", "no active claude credential for file upload")
		return
	}

	filename := strings.TrimSpace(fileHeader.Filename)
	mimeType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))

	fileID, errUpload := uploadClaudeFileToAnthropic(c.Request.Context(), http.DefaultClient, s.getConfig(), credential, filename, mimeType, data)
	if errUpload != nil {
		var upstream *executor.ClaudeFileUploadError
		if errors.As(errUpload, &upstream) && upstream.Status >= 400 && upstream.Status < 500 {
			writeClaudeFileError(c, upstream.Status, "invalid_request_error", upstream.Message)
			return
		}
		writeClaudeFileError(c, http.StatusBadGateway, "api_error", errUpload.Error())
		return
	}
	cache.PutClaudeFileContent(fileID, cache.ClaudeFileContent{Data: data, MIMEType: mimeType})

	c.JSON(http.StatusOK, gin.H{
		"type":       "file",
		"id":         fileID,
		"filename":   filename,
		"mime_type":  mimeType,
		"size_bytes": len(data),
	})
}
