package tools

import (
	"context"
	"path/filepath"
	"testing"
)

func TestValidateExecCommandPaths_DeniesSiblingEscape(t *testing.T) {
	root := t.TempDir()
	userB := filepath.Join(root, "user-b")
	ctx := WithToolWorkspace(context.Background(), userB)
	tool := NewExecTool(root, true)

	err := tool.validateExecCommandPaths(ctx, "cat ../user-a/.env", userB)
	if err == nil {
		t.Fatal("expected sibling escape to be denied")
	}
}

func TestValidateExecCommandPaths_DeniesAbsoluteOutsideAllowed(t *testing.T) {
	root := t.TempDir()
	userB := filepath.Join(root, "user-b")
	ctx := WithToolWorkspace(context.Background(), userB)
	tool := NewExecTool(root, true)

	err := tool.validateExecCommandPaths(ctx, "cat /etc/passwd", userB)
	if err == nil {
		t.Fatal("expected absolute outside path to be denied")
	}
}

func TestValidateExecCommandPaths_AllowsTeamWorkspaceAndAllowedPrefix(t *testing.T) {
	root := t.TempDir()
	userB := filepath.Join(root, "user-b")
	teamWs := filepath.Join(root, "teams", "team-1")
	skillsDir := filepath.Join(root, "skills")
	ctx := WithToolWorkspace(context.Background(), userB)
	ctx = WithToolTeamWorkspace(ctx, teamWs)
	tool := NewExecTool(root, true)
	tool.AllowPaths(skillsDir)

	if err := tool.validateExecCommandPaths(ctx, "cat ../teams/team-1/notes.md", userB); err != nil {
		t.Fatalf("expected team workspace path to be allowed: %v", err)
	}
	if err := tool.validateExecCommandPaths(ctx, "cat ../skills/SKILL.md", userB); err != nil {
		t.Fatalf("expected allowed prefix path to be allowed: %v", err)
	}
}

func TestValidateExecCommandPaths_AllowsCommandWithoutPaths(t *testing.T) {
	root := t.TempDir()
	userB := filepath.Join(root, "user-b")
	ctx := WithToolWorkspace(context.Background(), userB)
	tool := NewExecTool(root, true)

	if err := tool.validateExecCommandPaths(ctx, "echo ok", userB); err != nil {
		t.Fatalf("expected command without paths to be allowed: %v", err)
	}
}

func TestValidateCredentialedArgsPaths(t *testing.T) {
	root := t.TempDir()
	userB := filepath.Join(root, "user-b")
	teamWs := filepath.Join(root, "teams", "team-1")
	ctx := WithToolWorkspace(context.Background(), userB)
	ctx = WithToolTeamWorkspace(ctx, teamWs)
	tool := NewExecTool(root, true)

	if err := tool.validateCredentialedArgsPaths(ctx, []string{"--file", "../teams/team-1/config.yml"}, userB); err != nil {
		t.Fatalf("expected team workspace credentialed arg to be allowed: %v", err)
	}
	if err := tool.validateCredentialedArgsPaths(ctx, []string{"--file", "../user-a/.env"}, userB); err == nil {
		t.Fatal("expected sibling credentialed arg path to be denied")
	}
}
