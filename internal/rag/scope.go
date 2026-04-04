package rag

import (
	"fmt"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/sessions"
)

// UploadScope describes who uploaded and which chat bucket RAG paths use.
type UploadScope struct {
	OwnerID string // e.g. "telegram:123456" (Memory / channel user id)
	GroupID string // e.g. "telegram:group:-1001234", empty for DM
}

// ParseScope derives RAG visibility paths from the canonical session key and uploader user id.
// Session rest is like "telegram:direct:peerId", "telegram:group:-100...", or "ws:direct:conv".
func ParseScope(sessionKey, userID string) UploadScope {
	_, rest := sessions.ParseSessionKey(sessionKey)
	if rest == "" {
		return UploadScope{OwnerID: userID}
	}
	parts := strings.Split(rest, ":")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] != "group" {
			continue
		}
		if i < 1 || i+1 >= len(parts) {
			break
		}
		channel := parts[0]
		groupChatID := parts[i+1]
		return UploadScope{
			OwnerID: userID,
			GroupID: channel + ":group:" + groupChatID,
		}
	}
	return UploadScope{OwnerID: userID}
}

// PathPrefix is the directory prefix for memory_documents.path for this scope (trailing slash).
func (s UploadScope) PathPrefix() string {
	if s.GroupID != "" {
		return fmt.Sprintf("rag/group/%s/", s.GroupID)
	}
	return "rag/dm/"
}

// MemoryPath returns the full document path for a file name under this scope.
func (s UploadScope) MemoryPath(fileName string) string {
	return s.PathPrefix() + fileName
}
