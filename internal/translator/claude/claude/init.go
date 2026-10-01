package claude

import (
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v8/sdk/translator"
)

func init() {
	sdktranslator.Default().RegisterRequestEnvelope(sdktranslator.FormatClaude, sdktranslator.FormatClaude, ConvertClaudeRequestToClaude)
}
