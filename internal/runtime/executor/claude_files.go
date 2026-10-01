package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/config"
	"github.com/tidwall/gjson"
)

// claudeFilesAPIBeta is the beta identifier Anthropic requires for /v1/files.
const claudeFilesAPIBeta = "files-api-2025-04-14"

// ClaudeFileUploadError carries the upstream status so a caller can answer with it.
type ClaudeFileUploadError struct {
	Status  int
	Message string
}

func (e *ClaudeFileUploadError) Error() string {
	return fmt.Sprintf("claude file upload failed: status %d: %s", e.Status, e.Message)
}

// UploadClaudeFile sends one file body to the Anthropic Files API and returns its file id.
// It requires a Claude credential: the id only means something inside that workspace.
func UploadClaudeFile(ctx context.Context, client *http.Client, cfg *config.Config, auth *cliproxyauth.Auth, filename, mimeType string, data []byte) (string, error) {
	apiKey, baseURL := claudeCreds(auth)
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	if apiKey == "" {
		return "", fmt.Errorf("claude file upload requires a claude credential")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf("form-data; name=%q; filename=%q", "file", filename))
	if mimeType != "" {
		partHeader.Set("Content-Type", mimeType)
	}
	part, errPart := writer.CreatePart(partHeader)
	if errPart != nil {
		return "", errPart
	}
	if _, errWrite := part.Write(data); errWrite != nil {
		return "", errWrite
	}
	if errClose := writer.Close(); errClose != nil {
		return "", errClose
	}

	target := strings.TrimRight(baseURL, "/") + "/v1/files"
	req, errRequest := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body.Bytes()))
	if errRequest != nil {
		return "", errRequest
	}
	if errHeaders := applyClaudeHeaders(req, auth, apiKey, false, []string{claudeFilesAPIBeta}, nil, cfg, nil, false); errHeaders != nil {
		return "", errHeaders
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	httpClient := client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, errDo := httpClient.Do(req)
	if errDo != nil {
		return "", errDo
	}
	defer func() { _ = resp.Body.Close() }()

	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &ClaudeFileUploadError{Status: resp.StatusCode, Message: strings.TrimSpace(string(payload))}
	}
	fileID := strings.TrimSpace(gjson.GetBytes(payload, "id").String())
	if fileID == "" {
		return "", fmt.Errorf("claude file upload returned no file id")
	}
	return fileID, nil
}
