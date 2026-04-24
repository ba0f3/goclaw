package sandbox

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// normalizeReadOnlyHostPaths validates and canonicalizes extra host paths that
// should be mirrored read-only into the sandbox at the same absolute path.
func normalizeReadOnlyHostPaths(paths []string, workspaceRoot string, extraRoots ...string) []string {
	if len(paths) == 0 {
		return nil
	}
	root := filepath.Clean(workspaceRoot)
	seen := make(map[string]struct{}, len(paths))
	for _, raw := range paths {
		p := filepath.Clean(expandHome(strings.TrimSpace(raw)))
		if p == "" || p == "." || !filepath.IsAbs(p) {
			continue
		}
		fi, err := os.Stat(p)
		if err != nil || !fi.IsDir() {
			continue
		}
		if root != "" && isSameOrNestedPath(p, root) {
			// Already visible via primary workspace bind.
			continue
		}
		var underExtra bool
		for _, extra := range extraRoots {
			if extra != "" && isSameOrNestedPath(p, extra) {
				underExtra = true
				break
			}
		}
		if underExtra {
			continue
		}
		seen[p] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func readOnlyHostPathsKey(paths []string, workspaceRoot string, extraRoots ...string) string {
	return strings.Join(normalizeReadOnlyHostPaths(paths, workspaceRoot, extraRoots...), "\x00")
}

func isSameOrNestedPath(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	return strings.HasPrefix(path+string(filepath.Separator), root+string(filepath.Separator))
}
