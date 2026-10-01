// Package claude keeps a Claude request in Claude format while replacing an
// upload reference with bytes this process already holds.
package claude

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"

	internalcache "github.com/router-for-me/CLIProxyAPI/v8/internal/cache"
	translatorcommon "github.com/router-for-me/CLIProxyAPI/v8/internal/translator/common"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v8/sdk/translator"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ConvertClaudeRequestToClaude inlines stored uploads so a file id that belongs
// to another workspace still reaches the model. Everything else passes through.
func ConvertClaudeRequestToClaude(_ context.Context, req sdktranslator.RequestEnvelope) sdktranslator.RequestEnvelope {
	body := inlineStoredClaudeFiles(req.Body)
	if req.Model != "" && gjson.GetBytes(body, "model").String() != req.Model {
		if updated, errSet := sjson.SetBytes(body, "model", req.Model); errSet == nil {
			body = updated
		}
	}
	req.Body = body
	return req
}

// inlineStoredClaudeFiles replaces every resolvable upload reference with its bytes.
// It returns the original slice when nothing could be resolved.
func inlineStoredClaudeFiles(body []byte) []byte {
	if internalcache.ClaudeFileContentCacheLen() == 0 || !bytes.Contains(body, []byte(`"file_id"`)) {
		return body
	}
	messages := gjson.GetBytes(body, "messages")
	if !messages.IsArray() {
		return body
	}

	updated := body
	changed := false
	messages.ForEach(func(messageIndex, message gjson.Result) bool {
		content := message.Get("content")
		if !content.IsArray() {
			return true
		}
		content.ForEach(func(partIndex, part gjson.Result) bool {
			partType := part.Get("type").String()
			switch partType {
			case "container_upload", "document", "image":
			default:
				return true
			}
			replacement, ok := claudeStoredFilePart(part, partType)
			if !ok {
				return true
			}
			path := fmt.Sprintf("messages.%d.content.%d", messageIndex.Int(), partIndex.Int())
			next, errSet := sjson.SetRawBytes(updated, path, replacement)
			if errSet != nil {
				return true
			}
			updated = next
			changed = true
			return true
		})
		return true
	})
	if !changed {
		return body
	}
	return updated
}

// claudeStoredFilePart builds the inline block for one stored upload.
func claudeStoredFilePart(part gjson.Result, partType string) ([]byte, bool) {
	data, mimeType, ok := translatorcommon.ClaudeStoredFileBytes(part)
	if !ok {
		return nil, false
	}

	emitType := "document"
	if partType == "image" {
		emitType = "image"
	}
	out := []byte(`{"type":"","source":{"type":"base64","media_type":"","data":""}}`)
	out, _ = sjson.SetBytes(out, "type", emitType)
	out, _ = sjson.SetBytes(out, "source.media_type", mimeType)
	out, _ = sjson.SetBytes(out, "source.data", base64.StdEncoding.EncodeToString(data))
	return out, true
}
