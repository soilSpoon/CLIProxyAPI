package claude

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	internalcache "github.com/router-for-me/CLIProxyAPI/v8/internal/cache"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v8/sdk/translator"
)

func TestConvertClaudeRequestToCodex_CachedFileBecomesInputFile(t *testing.T) {
	const fileID = "file-codex-cached"
	if !internalcache.PutClaudeFileContent(fileID, internalcache.ClaudeFileContent{Data: []byte("pdf-bytes"), MIMEType: "application/pdf"}) {
		t.Fatal("put rejected the body")
	}
	t.Cleanup(func() { internalcache.DeleteClaudeFileContent(fileID) })

	want := "data:application/pdf;base64," + base64.StdEncoding.EncodeToString([]byte("pdf-bytes"))
	for _, block := range []string{
		`{"type": "container_upload", "file_id": "` + fileID + `"}`,
		`{"type": "document", "source": {"type": "file", "file_id": "` + fileID + `"}}`,
	} {
		input := []byte(`{"model":"gpt-5","messages":[{"role":"user","content":[{"type":"text","text":"read"},` + block + `]}]}`)
		output := string(ConvertClaudeRequestToCodex("gpt-5", input, false))
		if !strings.Contains(output, `"type":"input_file"`) || !strings.Contains(output, want) {
			t.Fatalf("%s: output = %s", block, output)
		}
	}
}

func TestConvertClaudeRequestToCodex_UncachedFileKeepsOldDrop(t *testing.T) {
	input := []byte(`{"model":"gpt-5","messages":[{"role":"user","content":[{"type":"text","text":"read"},{"type":"container_upload","file_id":"file-absent"}]}]}`)
	output := string(ConvertClaudeRequestToCodex("gpt-5", input, false))
	if strings.Contains(output, "input_file") {
		t.Fatalf("output = %s", output)
	}
	if !strings.Contains(output, "read") {
		t.Fatalf("text was lost: %s", output)
	}
}

func TestClaudeFileOnlyRequestSurfacesUnsupportedPart(t *testing.T) {
	input := []byte(`{"model":"gpt-5","messages":[{"role":"user","content":[{"type":"container_upload","file_id":"file-absent"}]}]}`)
	envelope := sdktranslator.TranslateRequestEnvelope(context.Background(), sdktranslator.FormatClaude, sdktranslator.FormatCodex, sdktranslator.RequestEnvelope{
		Format: sdktranslator.FormatClaude,
		Model:  "gpt-5",
		Body:   input,
	})
	if envelope.Err == nil || !strings.Contains(envelope.Err.Error(), "container_upload") {
		t.Fatalf("err = %v body = %s", envelope.Err, envelope.Body)
	}
}
