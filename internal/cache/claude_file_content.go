package cache

const (
	// ClaudeFileContentCacheCapacity bounds how many uploaded file bodies stay in process memory.
	ClaudeFileContentCacheCapacity = 64

	// ClaudeFileContentMaxEntryBytes keeps one oversized upload from consuming the whole budget.
	ClaudeFileContentMaxEntryBytes = 32 << 20
)

// ClaudeFileContent is the decodable body of one uploaded Claude file.
// OwnerAuthID names the credential whose workspace holds the file id.
type ClaudeFileContent struct {
	Data     []byte
	MIMEType string
}

var claudeFileContents = NewBoundedLRU[string, ClaudeFileContent](ClaudeFileContentCacheCapacity, nil)

// PutClaudeFileContent stores one uploaded body under its file id.
// It reports whether the body was small enough to keep.
func PutClaudeFileContent(fileID string, content ClaudeFileContent) bool {
	if fileID == "" || len(content.Data) == 0 || len(content.Data) > ClaudeFileContentMaxEntryBytes {
		return false
	}
	claudeFileContents.GetOrAdd(fileID, func() ClaudeFileContent { return content })
	return true
}

// GetClaudeFileContent returns the stored body for one file id.
func GetClaudeFileContent(fileID string) (ClaudeFileContent, bool) {
	if fileID == "" {
		return ClaudeFileContent{}, false
	}
	return claudeFileContents.Get(fileID)
}

// DeleteClaudeFileContent drops the stored body for one file id.
func DeleteClaudeFileContent(fileID string) bool {
	if fileID == "" {
		return false
	}
	return claudeFileContents.Delete(fileID)
}

// ClaudeFileContentCacheLen reports how many bodies are currently stored.
func ClaudeFileContentCacheLen() int {
	return claudeFileContents.Len()
}
