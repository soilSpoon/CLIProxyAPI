package common

import (
	"strings"

	internalcache "github.com/router-for-me/CLIProxyAPI/v8/internal/cache"
	"github.com/tidwall/gjson"
)

// ClaudeStoredFileBytes resolves the stored upload a content part references.
// The id sits in source.file_id for a document or image, and at the top level of
// a container_upload block or an OpenAI file object, which carry no source.
func ClaudeStoredFileBytes(part gjson.Result) (data []byte, mimeType string, ok bool) {
	source := part.Get("source")
	fileID := strings.TrimSpace(source.Get("file_id").String())
	if fileID == "" {
		fileID = strings.TrimSpace(part.Get("file_id").String())
	}
	if fileID == "" {
		return nil, "", false
	}
	content, found := internalcache.GetClaudeFileContent(fileID)
	if !found || len(content.Data) == 0 {
		return nil, "", false
	}

	mimeType = strings.TrimSpace(source.Get("media_type").String())
	if mimeType == "" {
		mimeType = strings.TrimSpace(part.Get("mime_type").String())
	}
	if mimeType == "" {
		mimeType = strings.TrimSpace(content.MIMEType)
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return content.Data, mimeType, true
}
