package claude

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	internalcache "github.com/router-for-me/CLIProxyAPI/v8/internal/cache"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v8/sdk/translator"
)

func runClaudeToClaude(body []byte) sdktranslator.RequestEnvelope {
	return sdktranslator.TranslateRequestEnvelope(context.Background(), sdktranslator.FormatClaude, sdktranslator.FormatClaude, sdktranslator.RequestEnvelope{
		Format: sdktranslator.FormatClaude,
		Model:  "claude-opus-5",
		Body:   body,
	})
}

func TestClaudeRequestWithoutFilesPassesThrough(t *testing.T) {
	input := []byte(`{"model":"claude-opus-5","max_tokens":64,"thinking":{"type":"enabled","display":"summarized"},"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`)
	envelope := runClaudeToClaude(input)
	if string(envelope.Body) != string(input) {
		t.Fatalf("body changed:\n%s", envelope.Body)
	}
	if envelope.Err != nil {
		t.Fatalf("err = %v", envelope.Err)
	}
}

func TestClaudeRequestInlinesStoredUploads(t *testing.T) {
	const fileID = "file-claude-inline"
	if !internalcache.PutClaudeFileContent(fileID, internalcache.ClaudeFileContent{Data: []byte("pdf-bytes"), MIMEType: "application/pdf"}) {
		t.Fatal("put rejected the body")
	}
	t.Cleanup(func() { internalcache.DeleteClaudeFileContent(fileID) })

	want := base64.StdEncoding.EncodeToString([]byte("pdf-bytes"))
	for _, block := range []string{
		`{"type":"container_upload","file_id":"` + fileID + `"}`,
		`{"type":"document","source":{"type":"file","file_id":"` + fileID + `"}}`,
	} {
		input := []byte(`{"model":"claude-opus-5","messages":[{"role":"user","content":[` + block + `]}]}`)
		body := string(runClaudeToClaude(input).Body)
		if strings.Contains(body, "container_upload") || strings.Contains(body, `"type":"file"`) {
			t.Fatalf("%s: reference survived: %s", block, body)
		}
		if !strings.Contains(body, `"type":"document"`) || !strings.Contains(body, want) {
			t.Fatalf("%s: body = %s", block, body)
		}
	}
}

func TestClaudeRequestLeavesUnknownFileID(t *testing.T) {
	input := []byte(`{"model":"claude-opus-5","messages":[{"role":"user","content":[{"type":"container_upload","file_id":"file-absent"}]}]}`)
	envelope := runClaudeToClaude(input)
	if string(envelope.Body) != string(input) {
		t.Fatalf("body changed: %s", envelope.Body)
	}
}

func TestClaudeRequestInlinesStoredImageUpload(t *testing.T) {
	const fileID = "file-claude-image"
	if !internalcache.PutClaudeFileContent(fileID, internalcache.ClaudeFileContent{Data: []byte("png-bytes"), MIMEType: "image/png"}) {
		t.Fatal("put rejected the body")
	}
	t.Cleanup(func() { internalcache.DeleteClaudeFileContent(fileID) })

	input := []byte(`{"model":"claude-opus-5","messages":[{"role":"user","content":[{"type":"image","source":{"type":"file","file_id":"` + fileID + `"}}]}]}`)
	body := string(runClaudeToClaude(input).Body)
	if !strings.Contains(body, `"type":"image"`) || !strings.Contains(body, `"source":{"type":"base64"`) {
		t.Fatalf("body = %s", body)
	}
	if !strings.Contains(body, base64.StdEncoding.EncodeToString([]byte("png-bytes"))) {
		t.Fatalf("bytes missing: %s", body)
	}
}

func TestClaudeRequestRenamesModel(t *testing.T) {
	input := []byte(`{"model":"alias-name","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`)
	body := string(runClaudeToClaude(input).Body)
	if !strings.Contains(body, `"model":"claude-opus-5"`) {
		t.Fatalf("model not renamed: %s", body)
	}
}

// TestClaudeRealisticRequestPassesThrough covers the main Claude path with the
// shapes production traffic carries, so the registration itself adds no change.
func TestClaudeRealisticRequestPassesThrough(t *testing.T) {
	input := []byte(`{` +
		`"model":"claude-opus-5","max_tokens":32000,"temperature":0.2,"stream":true,` +
		`"system":[{"type":"text","text":"You are a careful engineer.","cache_control":{"type":"ephemeral"}}],` +
		`"thinking":{"type":"enabled","budget_tokens":8000,"display":"summarized"},` +
		`"tools":[{"name":"read_file","description":"Read a file","input_schema":{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}}],` +
		`"tool_choice":{"type":"auto"},` +
		`"messages":[` +
		`{"role":"user","content":[{"type":"text","text":"look at this"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"aGVsbG8="}}]},` +
		`{"role":"assistant","content":[{"type":"thinking","thinking":"plan","signature":"sig-abc"},{"type":"text","text":"reading"},{"type":"tool_use","id":"toolu_1","name":"read_file","input":{"path":"a.go"}}]},` +
		`{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"package main"}]}` +
		`]}`)

	envelope := runClaudeToClaude(input)
	if envelope.Err != nil {
		t.Fatalf("err = %v", envelope.Err)
	}
	if string(envelope.Body) != string(input) {
		t.Fatalf("body changed:\n%s", envelope.Body)
	}
}
