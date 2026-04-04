package rag

import "strings"

// GroupDocumentPathPrefix is the memory_documents / memory_chunks path prefix for shared group RAG docs.
func GroupDocumentPathPrefix(ragGroupID string) string {
	if ragGroupID == "" {
		return ""
	}
	return "rag/group/" + ragGroupID + "/"
}

// VisibleRAGMemoryChunk applies RAG sharing rules to a memory chunk (path + optional owner on chunk).
// Non-rag/ paths are always visible. When sharedMemory is true, RAG rules are skipped.
// ragPersonalOwnerID is the channel sender id in group runs (e.g. Telegram numeric id); querierID is often
// group-scoped (e.g. group:telegram:-100...) and does not match DM-indexed chunk user_id.
func VisibleRAGMemoryChunk(path string, chunkUserID *string, querierID, ragGroupID, ragPersonalOwnerID string, sharedMemory bool) bool {
	if sharedMemory || !strings.HasPrefix(path, "rag/") {
		return true
	}
	cu := ""
	if chunkUserID != nil {
		cu = *chunkUserID
	}
	if ragGroupID != "" {
		prefix := GroupDocumentPathPrefix(ragGroupID)
		if strings.HasPrefix(path, prefix) {
			return true
		}
		if querierID == "" && ragPersonalOwnerID == "" {
			return false
		}
		if strings.HasPrefix(path, "rag/dm/") {
			if ragPersonalOwnerID != "" && cu == ragPersonalOwnerID {
				return true
			}
			if querierID != "" && cu == querierID {
				return true
			}
			return false
		}
		return false
	}
	if querierID == "" {
		return false
	}
	return cu == querierID
}
