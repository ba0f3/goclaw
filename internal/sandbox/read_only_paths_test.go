package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeReadOnlyHostPaths_FiltersAndSorts(t *testing.T) {
	tmp := t.TempDir()
	workspaceRoot := filepath.Join(tmp, "workspace")
	outsideA := filepath.Join(tmp, "outside-a")
	outsideB := filepath.Join(tmp, "outside-b")
	insideWorkspace := filepath.Join(workspaceRoot, "nested")
	mustMkdirAll(t, outsideA)
	mustMkdirAll(t, outsideB)
	mustMkdirAll(t, insideWorkspace)

	got := normalizeReadOnlyHostPaths([]string{
		outsideB,
		outsideA,
		outsideA,
		insideWorkspace,
		filepath.Join(tmp, "missing"),
		".",
	}, workspaceRoot)

	want := []string{outsideA, outsideB}
	if len(got) != len(want) {
		t.Fatalf("normalizeReadOnlyHostPaths() len=%d, want %d (got=%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeReadOnlyHostPaths()[%d] = %q, want %q (got=%#v)", i, got[i], want[i], got)
		}
	}
}

func TestReadOnlyHostPathsKey_IsOrderIndependent(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a")
	b := filepath.Join(tmp, "b")
	mustMkdirAll(t, a)
	mustMkdirAll(t, b)

	k1 := readOnlyHostPathsKey([]string{b, a}, "")
	k2 := readOnlyHostPathsKey([]string{a, b, a}, "")
	if k1 == "" || k2 == "" {
		t.Fatalf("expected non-empty keys, got %q and %q", k1, k2)
	}
	if k1 != k2 {
		t.Fatalf("expected equal keys for same path set, got %q and %q", k1, k2)
	}
}

func TestBuildBwrapArgs_IncludesReadOnlyHostPaths(t *testing.T) {
	tmp := t.TempDir()
	workspaceRoot := filepath.Join(tmp, "workspace")
	skillsDir := filepath.Join(tmp, "skills")
	mustMkdirAll(t, workspaceRoot)
	mustMkdirAll(t, skillsDir)

	cfg := DefaultConfig()
	cfg.WorkspaceAccess = AccessRW
	cfg.ReadOnlyHostPaths = []string{skillsDir}

	args := buildBwrapArgs(cfg, workspaceRoot)
	if !containsArgTriplet(args, "--ro-bind", skillsDir, skillsDir) {
		t.Fatalf("expected --ro-bind %s %s in args, got %#v", skillsDir, skillsDir, args)
	}
}

func TestDockerReadOnlyPathMountArgs_IncludesReadOnlyMounts(t *testing.T) {
	tmp := t.TempDir()
	workspaceRoot := filepath.Join(tmp, "workspace")
	skillsDir := filepath.Join(tmp, "skills")
	mustMkdirAll(t, workspaceRoot)
	mustMkdirAll(t, skillsDir)

	cfg := DefaultConfig()
	cfg.ReadOnlyHostPaths = []string{skillsDir}

	args := dockerReadOnlyPathMountArgs(cfg, workspaceRoot)
	want := []string{"-v", skillsDir + ":" + skillsDir + ":ro"}
	if len(args) != len(want) {
		t.Fatalf("dockerReadOnlyPathMountArgs() len=%d, want %d (got=%#v)", len(args), len(want), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("dockerReadOnlyPathMountArgs()[%d] = %q, want %q (got=%#v)", i, args[i], want[i], args)
		}
	}
}

func containsArgTriplet(args []string, a, b, c string) bool {
	for i := 0; i+2 < len(args); i++ {
		if args[i] == a && args[i+1] == b && args[i+2] == c {
			return true
		}
	}
	return false
}

func mustMkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}
