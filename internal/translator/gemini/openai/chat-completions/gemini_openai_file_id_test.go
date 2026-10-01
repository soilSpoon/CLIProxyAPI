package chat_completions

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	internalcache "github.com/router-for-me/CLIProxyAPI/v8/internal/cache"
	translatorcommon "github.com/router-for-me/CLIProxyAPI/v8/internal/translator/common"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v8/sdk/translator"
	"github.com/tidwall/gjson"
)

func TestConvertOpenAIRequestToGemini_FileID(t *testing.T) {
	const storedID = "file-openai-gemini-stored"
	internalcache.PutClaudeFileContent(storedID, internalcache.ClaudeFileContent{Data: []byte("pdf-bytes"), MIMEType: "application/pdf"})
	t.Cleanup(func() { internalcache.DeleteClaudeFileContent(storedID) })

	const (
		text    = `{"type":"text","text":"read it"}`
		missing = `{"type":"file","file":{"file_id":"file-absent"}}`
		stored  = `{"type":"file","file":{"file_id":"` + storedID + `"}}`
	)
	cases := []struct {
		name, content string
		wantErr       bool
		wantParts     int
	}{
		{name: "unknown file id alone", content: missing, wantErr: true},
		{name: "text beside an unknown file id", content: text + "," + missing, wantParts: 1},
		{name: "stored upload becomes inline data", content: stored, wantParts: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := `{"model":"gemini-2.5-pro","messages":[{"role":"user","content":[` + tc.content + `]}]}`
			got := sdktranslator.TranslateRequestEnvelope(context.Background(), sdktranslator.FormatOpenAI, sdktranslator.FormatGemini,
				sdktranslator.RequestEnvelope{Format: sdktranslator.FormatOpenAI, Model: "gemini-2.5-pro", Body: []byte(payload)})

			var part *translatorcommon.UnsupportedPartError
			if tc.wantErr {
				if !errors.As(got.Err, &part) || part.Type != "file" {
					t.Fatalf("err = %v, want unsupported file", got.Err)
				}
				return
			}
			if got.Err != nil {
				t.Fatalf("err = %v", got.Err)
			}
			if n := int(gjson.GetBytes(got.Body, "contents.0.parts.#").Int()); n != tc.wantParts {
				t.Fatalf("parts = %d, want %d. Output: %s", n, tc.wantParts, got.Body)
			}
			if tc.content == stored {
				if data := gjson.GetBytes(got.Body, "contents.0.parts.0.inlineData.data").String(); data != base64.StdEncoding.EncodeToString([]byte("pdf-bytes")) {
					t.Fatalf("inline data = %q. Output: %s", data, got.Body)
				}
			}
		})
	}
}
