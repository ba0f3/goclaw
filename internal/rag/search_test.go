package rag

import "testing"

func TestVisibleRAGMemoryChunk_NonRAG(t *testing.T) {
	if !VisibleRAGMemoryChunk("MEMORY.md", nil, "u1", "", "", false) {
		t.Fatal("non-rag path should be visible")
	}
}

func TestVisibleRAGMemoryChunk_GroupShared(t *testing.T) {
	path := "rag/group/telegram:group:-100/a.pdf"
	if !VisibleRAGMemoryChunk(path, strPtr("telegram:1"), "telegram:2", "telegram:group:-100", "", false) {
		t.Fatal("group path visible to any querier in group context")
	}
}

func TestVisibleRAGMemoryChunk_GroupSharedNoQuerierID(t *testing.T) {
	path := "rag/group/telegram:group:-100/a.pdf"
	if !VisibleRAGMemoryChunk(path, strPtr("telegram:1"), "", "telegram:group:-100", "", false) {
		t.Fatal("group bucket visible even when querier id missing; path match is enough")
	}
}

func TestVisibleRAGMemoryChunk_GroupSeesOwnDM(t *testing.T) {
	path := "rag/dm/b.pdf"
	u := "telegram:1"
	if !VisibleRAGMemoryChunk(path, strPtr(u), u, "telegram:group:-100", "", false) {
		t.Fatal("querier should see own DM rag in group context")
	}
}

func TestVisibleRAGMemoryChunk_GroupSeesDMDocsViaPersonalOwner(t *testing.T) {
	path := "rag/dm/b.pdf"
	chunkOwner := "12345"
	groupQuerier := "group:telegram:-100123"
	if !VisibleRAGMemoryChunk(path, strPtr(chunkOwner), groupQuerier, "telegram:group:-100123", chunkOwner, false) {
		t.Fatal("group-scoped querier should see own DM rag when personal owner id matches chunk")
	}
}

func TestVisibleRAGMemoryChunk_GroupHidesOthersDM(t *testing.T) {
	path := "rag/dm/b.pdf"
	if VisibleRAGMemoryChunk(path, strPtr("telegram:1"), "telegram:2", "telegram:group:-100", "", false) {
		t.Fatal("other user's DM rag hidden in group context")
	}
}

func TestVisibleRAGMemoryChunk_DMOnlySelf(t *testing.T) {
	u := "telegram:1"
	if !VisibleRAGMemoryChunk("rag/group/g/x.pdf", strPtr(u), u, "", "", false) {
		t.Fatal("owner sees own group upload in DM context")
	}
	if VisibleRAGMemoryChunk("rag/group/g/x.pdf", strPtr("telegram:9"), u, "", "", false) {
		t.Fatal("cannot see other user's group upload in DM context")
	}
}

func strPtr(s string) *string { return &s }
