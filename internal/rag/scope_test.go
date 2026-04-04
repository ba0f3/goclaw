package rag

import "testing"

func TestParseScope_DM(t *testing.T) {
	s := ParseScope("agent:fox:telegram:direct:52007861", "telegram:52007861")
	if s.GroupID != "" {
		t.Fatalf("GroupID = %q, want empty", s.GroupID)
	}
	if s.OwnerID != "telegram:52007861" {
		t.Fatalf("OwnerID = %q", s.OwnerID)
	}
	if got, want := s.MemoryPath("a.pdf"), "rag/dm/a.pdf"; got != want {
		t.Fatalf("MemoryPath = %q, want %q", got, want)
	}
}

func TestParseScope_Group(t *testing.T) {
	s := ParseScope("agent:fox:telegram:group:-1001234", "telegram:111")
	if got, want := s.GroupID, "telegram:group:-1001234"; got != want {
		t.Fatalf("GroupID = %q, want %q", got, want)
	}
	if got, want := s.MemoryPath("doc.pdf"), "rag/group/telegram:group:-1001234/doc.pdf"; got != want {
		t.Fatalf("MemoryPath = %q, want %q", got, want)
	}
}

func TestParseScope_GroupTopic(t *testing.T) {
	s := ParseScope("agent:fox:telegram:group:-100:topic:9", "telegram:111")
	if got, want := s.GroupID, "telegram:group:-100"; got != want {
		t.Fatalf("GroupID = %q, want %q", got, want)
	}
}

func TestParseScope_WSDirect(t *testing.T) {
	s := ParseScope("agent:fox:ws:direct:conv123", "ws:user:1")
	if s.GroupID != "" {
		t.Fatalf("GroupID = %q, want empty", s.GroupID)
	}
	if s.OwnerID != "ws:user:1" {
		t.Fatalf("OwnerID = %q, want ws:user:1", s.OwnerID)
	}
	if got, want := s.MemoryPath("x.txt"), "rag/dm/x.txt"; got != want {
		t.Fatalf("MemoryPath = %q, want %q", got, want)
	}
}

func TestParseScope_MalformedKeyFallsBackToDM(t *testing.T) {
	s := ParseScope("not-a-session-key", "u1")
	if s.GroupID != "" {
		t.Fatalf("GroupID = %q, want empty", s.GroupID)
	}
	if s.OwnerID != "u1" {
		t.Fatalf("OwnerID = %q, want u1", s.OwnerID)
	}
}
