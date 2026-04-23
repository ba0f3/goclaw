package providers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCursorBuildArgsDoesNotUseSessionIDFlag(t *testing.T) {
	p := NewCursorCLIProvider("agent")
	workDir := t.TempDir()

	args := p.buildArgs("gpt-5.4", workDir, "", false)
	joined := strings.Join(args, " ")

	if strings.Contains(joined, "--session-id") {
		t.Fatalf("buildArgs() includes unsupported --session-id flag: %q", joined)
	}
}

func TestCursorBuildArgsWithChatID(t *testing.T) {
	p := NewCursorCLIProvider("agent")
	workDir := t.TempDir()

	args := p.buildArgs("gpt-5.4", workDir, "chat-abc123", false)
	joined := strings.Join(args, " ")

	if !strings.Contains(joined, "--resume chat-abc123") {
		t.Fatalf("buildArgs() should include --resume with chat ID: %q", joined)
	}
}

func TestCursorBuildArgsWithoutChatID(t *testing.T) {
	p := NewCursorCLIProvider("agent")
	workDir := t.TempDir()

	args := p.buildArgs("gpt-5.4", workDir, "", false)
	joined := strings.Join(args, " ")

	if strings.Contains(joined, "--resume") {
		t.Fatalf("buildArgs() should not include --resume when chatID is empty: %q", joined)
	}
}

func TestResetCursorCLISession(t *testing.T) {
	baseDir := t.TempDir()
	sessionKey := "test-session-reset"

	registry := getCursorChatIDRegistry(baseDir)
	registry.Store(sessionKey, "chat-123")

	safe := sanitizePathSegment(sessionKey)
	workDir := filepath.Join(baseDir, safe)
	chatIDFile := filepath.Join(workDir, ".cursor", "chatid")
	if err := os.MkdirAll(filepath.Dir(chatIDFile), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(chatIDFile, []byte("chat-123"), 0600); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	ResetCursorCLISession(baseDir, sessionKey)

	if _, ok := registry.Load(sessionKey); ok {
		t.Fatal("registry should not contain session key after reset")
	}

	if _, err := os.Stat(chatIDFile); !os.IsNotExist(err) {
		t.Fatal("chat ID file should be deleted after reset")
	}
}

func TestGetOrCreateChatID_LoadsFromDisk(t *testing.T) {
	p := NewCursorCLIProvider("agent")
	p.baseWorkDir = t.TempDir()
	sessionKey := "test-session-disk"

	workDir := p.ensureWorkDir(sessionKey)
	chatIDFile := filepath.Join(workDir, ".cursor", "chatid")
	if err := os.MkdirAll(filepath.Dir(chatIDFile), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	expectedChatID := "disk-chat-456"
	if err := os.WriteFile(chatIDFile, []byte(expectedChatID), 0600); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	chatID, err := p.getOrCreateChatID(context.Background(), sessionKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chatID != expectedChatID {
		t.Fatalf("expected chat ID %q, got %q", expectedChatID, chatID)
	}

	registry := getCursorChatIDRegistry(p.baseWorkDir)
	if cached, ok := registry.Load(sessionKey); !ok || cached.(string) != expectedChatID {
		t.Fatal("registry should contain loaded chat ID")
	}
}
