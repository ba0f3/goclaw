package agent

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestBuildSandboxReadOnlyHostPaths_IncludesExistingSources(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	dataDir := filepath.Join(tmp, "data")
	tenantID := uuid.New()
	tenantSlug := "acme"

	mustMkdirAll(t, filepath.Join(homeDir, ".goclaw", "skills"))
	mustMkdirAll(t, filepath.Join(homeDir, ".agents", "skills"))
	mustMkdirAll(t, filepath.Join(dataDir, "skills-store"))
	mustMkdirAll(t, filepath.Join(dataDir, "tenants", tenantSlug, "skills-store"))
	t.Setenv("HOME", homeDir)

	got := buildSandboxReadOnlyHostPaths(dataDir, tenantID, tenantSlug, filepath.Join(tmp, "workspace"))
	want := []string{
		filepath.Join(dataDir, "skills-store"),
		filepath.Join(dataDir, "tenants", tenantSlug, "skills-store"),
		filepath.Join(homeDir, ".agents", "skills"),
		filepath.Join(homeDir, ".goclaw", "skills"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildSandboxReadOnlyHostPaths() = %#v, want %#v", got, want)
	}
}

func TestBuildSandboxReadOnlyHostPaths_SkipsWorkspaceNestedPaths(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeDir := filepath.Join(workspaceRoot, "home")
	dataDir := filepath.Join(workspaceRoot, "data")

	mustMkdirAll(t, filepath.Join(homeDir, ".goclaw", "skills"))
	mustMkdirAll(t, filepath.Join(homeDir, ".agents", "skills"))
	mustMkdirAll(t, filepath.Join(dataDir, "skills-store"))
	t.Setenv("HOME", homeDir)

	got := buildSandboxReadOnlyHostPaths(dataDir, uuid.New(), "slug", workspaceRoot)
	if len(got) != 0 {
		t.Fatalf("expected no read-only host paths under workspace root, got %#v", got)
	}
}

func mustMkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}
