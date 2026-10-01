package cache

import (
	"bytes"
	"fmt"
	"testing"
)

func TestClaudeFileContentRoundTrip(t *testing.T) {
	const id = "file-round-trip"
	t.Cleanup(func() { DeleteClaudeFileContent(id) })

	if !PutClaudeFileContent(id, ClaudeFileContent{Data: []byte("pdf-bytes"), MIMEType: "application/pdf"}) {
		t.Fatal("put rejected a valid body")
	}
	got, ok := GetClaudeFileContent(id)
	if !ok {
		t.Fatal("stored body was not found")
	}
	if !bytes.Equal(got.Data, []byte("pdf-bytes")) || got.MIMEType != "application/pdf" {
		t.Fatalf("got %#v", got)
	}
	if !DeleteClaudeFileContent(id) {
		t.Fatal("delete reported a missing body")
	}
	if _, ok := GetClaudeFileContent(id); ok {
		t.Fatal("body survived deletion")
	}
}

func TestClaudeFileContentRejectsUnusableBodies(t *testing.T) {
	if PutClaudeFileContent("", ClaudeFileContent{Data: []byte("x")}) {
		t.Fatal("empty file id was accepted")
	}
	if PutClaudeFileContent("file-empty-body", ClaudeFileContent{}) {
		t.Fatal("empty body was accepted")
	}
	if PutClaudeFileContent("file-oversize", ClaudeFileContent{Data: make([]byte, ClaudeFileContentMaxEntryBytes+1)}) {
		t.Fatal("oversize body was accepted")
	}
	if _, ok := GetClaudeFileContent(""); ok {
		t.Fatal("empty file id returned a body")
	}
	if DeleteClaudeFileContent("") {
		t.Fatal("empty file id reported a deletion")
	}
}

func TestClaudeFileContentCacheStaysBounded(t *testing.T) {
	previous := claudeFileContents
	claudeFileContents = NewBoundedLRU[string, ClaudeFileContent](ClaudeFileContentCacheCapacity, nil)
	t.Cleanup(func() { claudeFileContents = previous })

	ids := make([]string, 0, ClaudeFileContentCacheCapacity+1)
	for i := 0; i <= ClaudeFileContentCacheCapacity; i++ {
		id := fmt.Sprintf("file-bounded-%d", i)
		ids = append(ids, id)
		if !PutClaudeFileContent(id, ClaudeFileContent{Data: []byte("x")}) {
			t.Fatalf("put %d rejected", i)
		}
	}
	if got := ClaudeFileContentCacheLen(); got != ClaudeFileContentCacheCapacity {
		t.Fatalf("len = %d, want %d", got, ClaudeFileContentCacheCapacity)
	}
	if _, ok := GetClaudeFileContent(ids[0]); ok {
		t.Fatalf("oldest body %q survived the bound", ids[0])
	}
}
