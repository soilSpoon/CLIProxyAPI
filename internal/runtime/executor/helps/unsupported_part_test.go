package helps

import (
	"context"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/clienterror"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v8/sdk/translator"
	"github.com/tidwall/gjson"
)

func TestClaudeContainerUploadToGeminiReturns400(t *testing.T) {
	payload := []byte(`{"messages":[{"role":"user","content":[{"type":"container_upload","file_id":"file-example"}]}]}`)
	for _, compat := range []bool{false, true} {
		_, body, err := TranslateRequestPairReturningError(
			context.Background(),
			nil,
			nil,
			sdktranslator.FormatClaude,
			sdktranslator.FormatGemini,
			"gemini-test",
			payload,
			payload,
			false,
			compat,
		)
		if err == nil {
			t.Fatalf("compat=%v: dropped part was sent, body=%s", compat, body)
		}
		if got := err.Error(); got != "unsupported content part: container_upload" {
			t.Fatalf("compat=%v: error = %q", compat, got)
		}
		if status := clienterror.HTTPStatusFromError(err); status != 400 {
			t.Fatalf("compat=%v: status = %d, want 400", compat, status)
		}
		_, err = TranslateRequestReturningError(context.Background(), nil, nil, sdktranslator.FormatClaude, sdktranslator.FormatGemini, "gemini-test", payload, false, compat)
		if err == nil || err.Error() != "unsupported content part: container_upload" || clienterror.HTTPStatusFromError(err) != 400 {
			t.Fatalf("compat=%v: count path error = %v", compat, err)
		}
	}
}

func TestClaudeContainerUploadToOpenAIReturns400(t *testing.T) {
	payload := []byte(`{"messages":[{"role":"user","content":[{"type":"container_upload","file_id":"file-example"}]}]}`)
	_, err := TranslateRequestReturningError(context.Background(), nil, nil, sdktranslator.FormatClaude, sdktranslator.FormatOpenAI, "gpt-test", payload, false, true)
	if err == nil {
		t.Fatal("dropped part was sent")
	}
	if got := err.Error(); got != "unsupported content part: container_upload" {
		t.Fatalf("error = %q", got)
	}
	if status := clienterror.HTTPStatusFromError(err); status != 400 {
		t.Fatalf("status = %d, want 400", status)
	}
}

func TestClaudeContainerUploadToCodexReturns400(t *testing.T) {
	payload := []byte(`{"messages":[{"role":"user","content":[{"type":"container_upload","file_id":"file-example"}]}]}`)
	_, err := TranslateRequestReturningError(context.Background(), nil, nil, sdktranslator.FormatClaude, sdktranslator.FormatCodex, "gpt-5", payload, false, true)
	if err == nil {
		t.Fatal("dropped part was sent")
	}
	if got := err.Error(); got != "unsupported content part: container_upload" {
		t.Fatalf("error = %q", got)
	}
	if status := clienterror.HTTPStatusFromError(err); status != 400 {
		t.Fatalf("status = %d, want 400", status)
	}
}

func TestClaudeContainerUploadToInteractionsReturns400(t *testing.T) {
	payload := []byte(`{"messages":[{"role":"user","content":[{"type":"container_upload","file_id":"file-example"}]}]}`)
	_, err := TranslateRequestReturningError(context.Background(), nil, nil, sdktranslator.FormatClaude, sdktranslator.FormatInteractions, "gemini-3-flash", payload, false, true)
	if err == nil {
		t.Fatal("dropped part was sent")
	}
	if got := err.Error(); got != "unsupported content part: container_upload" {
		t.Fatalf("error = %q", got)
	}
	if status := clienterror.HTTPStatusFromError(err); status != 400 {
		t.Fatalf("status = %d, want 400", status)
	}
}

// TestByteWrappersKeepAJSONBodyForAnUnsendableFile guards the executors that still
// ignore the error: they must get a request body, never an empty one.
func TestByteWrappersKeepAJSONBodyForAnUnsendableFile(t *testing.T) {
	payload := []byte(`{"model":"m","messages":[{"role":"user","content":[{"type":"container_upload","file_id":"file-absent"}]}]}`)
	for _, to := range []sdktranslator.Format{sdktranslator.FormatOpenAI, sdktranslator.FormatCodex, sdktranslator.FormatInteractions, sdktranslator.FormatGemini} {
		for _, compat := range []bool{false, true} {
			original, working := TranslateRequestPairWithAPIKeyModelCompatibility(context.Background(), nil, nil, sdktranslator.FormatClaude, to, "m", payload, payload, false, compat)
			for _, body := range [][]byte{original, working} {
				if !gjson.ValidBytes(body) || len(body) < 2 {
					t.Fatalf("to=%s compat=%v: body = %q", to, compat, body)
				}
			}
			if _, _, err := TranslateRequestPairReturningError(context.Background(), nil, nil, sdktranslator.FormatClaude, to, "m", payload, payload, false, compat); err == nil {
				t.Fatalf("to=%s compat=%v: the checked variant lost the error", to, compat)
			}
		}
	}
}
