package tools

import (
	"context"
	"testing"
)

func TestSandboxHostMountRoot(t *testing.T) {
	tests := []struct {
		name              string
		ctxWorkspace      string
		registryWorkspace string
		want              string
	}{
		{
			name:              "no context workspace uses registry",
			ctxWorkspace:      "",
			registryWorkspace: "/home/u/.goclaw/workspace",
			want:              "/home/u/.goclaw/workspace",
		},
		{
			name:              "DM session path is mount root",
			ctxWorkspace:      "/home/u/.goclaw/workspace/fox-spirit/telegram/52007861",
			registryWorkspace: "/home/u/.goclaw/workspace",
			want:              "/home/u/.goclaw/workspace/fox-spirit/telegram/52007861",
		},
		{
			name:              "team group session path",
			ctxWorkspace:      "/home/u/.goclaw/workspace/teams/019d6093-eaa9-7a13-a4f1-d7b8925c300c/-1003819627125",
			registryWorkspace: "/home/u/.goclaw/workspace",
			want:              "/home/u/.goclaw/workspace/teams/019d6093-eaa9-7a13-a4f1-d7b8925c300c/-1003819627125",
		},
		{
			name:              "shared agent base when context is agent root",
			ctxWorkspace:      "/home/u/.goclaw/workspace/fox-spirit",
			registryWorkspace: "/home/u/.goclaw/workspace/fox-spirit",
			want:              "/home/u/.goclaw/workspace/fox-spirit",
		},
		{
			name:              "context outside registry uses context path",
			ctxWorkspace:      "/home/u/workspace/fox/telegram/52007861",
			registryWorkspace: "/home/u/.goclaw/workspace",
			want:              "/home/u/workspace/fox/telegram/52007861",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.ctxWorkspace != "" {
				ctx = WithToolWorkspace(ctx, tt.ctxWorkspace)
			}
			got := SandboxHostMountRoot(ctx, tt.registryWorkspace)
			if got != tt.want {
				t.Errorf("SandboxHostMountRoot() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSandboxHostPathToContainer(t *testing.T) {
	tests := []struct {
		name          string
		hostPath      string
		hostMountRoot string
		containerBase string
		want          string
		wantErr       bool
	}{
		{
			name:          "empty host path falls back to mount root",
			hostPath:      "",
			hostMountRoot: "/app/ws",
			containerBase: "/workspace",
			want:          "/app/ws",
		},
		{
			name:          "mount root returns host path directly",
			hostPath:      "/app/ws",
			hostMountRoot: "/app/ws",
			containerBase: "/workspace",
			want:          "/app/ws",
		},
		{
			name:          "nested cwd returns host path directly",
			hostPath:      "/app/ws/agent/u1/sub",
			hostMountRoot: "/app/ws",
			containerBase: "/workspace",
			want:          "/app/ws/agent/u1/sub",
		},
		{
			name:          "outside mount returns host path directly",
			hostPath:      "/other/x",
			hostMountRoot: "/app/ws",
			containerBase: "/workspace",
			want:          "/other/x",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SandboxHostPathToContainer(tt.hostPath, tt.hostMountRoot, tt.containerBase)
			if tt.wantErr {
				if err == nil {
					t.Errorf("SandboxHostPathToContainer() = %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("SandboxHostPathToContainer() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("SandboxHostPathToContainer() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSandboxCwd(t *testing.T) {
	tests := []struct {
		name            string
		ctxWorkspace    string
		globalWorkspace string
		containerBase   string
		want            string
		wantErr         bool
	}{
		{
			name:            "no workspace in context — fallback to mount root",
			ctxWorkspace:    "",
			globalWorkspace: "/app/workspace",
			containerBase:   "/workspace",
			want:            "/app/workspace",
		},
		{
			name:            "workspace equals global mount",
			ctxWorkspace:    "/app/workspace",
			globalWorkspace: "/app/workspace",
			containerBase:   "/workspace",
			want:            "/app/workspace",
		},
		{
			name:            "session leaf mount cwd is host path",
			ctxWorkspace:    "/app/workspace/fox-spirit/telegram/52007861",
			globalWorkspace: "/app/workspace/fox-spirit/telegram/52007861",
			containerBase:   "/workspace",
			want:            "/app/workspace/fox-spirit/telegram/52007861",
		},
		{
			name:            "team session leaf mount",
			ctxWorkspace:    "/app/workspace/teams/uuid/-1003819627125",
			globalWorkspace: "/app/workspace/teams/uuid/-1003819627125",
			containerBase:   "/workspace",
			want:            "/app/workspace/teams/uuid/-1003819627125",
		},
		{
			name:            "shared workspace mount equals agent base",
			ctxWorkspace:    "/app/workspace/fox-spirit",
			globalWorkspace: "/app/workspace/fox-spirit",
			containerBase:   "/workspace",
			want:            "/app/workspace/fox-spirit",
		},
		{
			name:            "workspace outside host mount — error",
			ctxWorkspace:    "/other/path/agent-a",
			globalWorkspace: "/app/workspace",
			containerBase:   "/workspace",
			wantErr:         true,
		},
		{
			name:            "disjoint tree: mount equals context workspace",
			ctxWorkspace:    "/home/u/workspace/fox/telegram/1",
			globalWorkspace: "/home/u/workspace/fox/telegram/1",
			containerBase:   "/workspace",
			want:            "/home/u/workspace/fox/telegram/1",
		},
		{
			name:            "disjoint tree: nested under context-as-mount",
			ctxWorkspace:    "/home/u/workspace/fox/telegram/1",
			globalWorkspace: "/home/u/workspace/fox",
			containerBase:   "/workspace",
			want:            "/home/u/workspace/fox/telegram/1",
		},
		{
			name:            "custom container base session mount",
			ctxWorkspace:    "/app/workspace/agent-a/sub",
			globalWorkspace: "/app/workspace/agent-a/sub",
			containerBase:   "/home/sandbox",
			want:            "/app/workspace/agent-a/sub",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.ctxWorkspace != "" {
				ctx = WithToolWorkspace(ctx, tt.ctxWorkspace)
			}
			got, err := SandboxCwd(ctx, tt.globalWorkspace, tt.containerBase)
			if tt.wantErr {
				if err == nil {
					t.Errorf("SandboxCwd() = %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Errorf("SandboxCwd() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("SandboxCwd() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveSandboxPath(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		containerCwd string
		want         string
	}{
		{
			name:         "relative path joined with cwd",
			path:         "file.txt",
			containerCwd: "/workspace/agent-a",
			want:         "/workspace/agent-a/file.txt",
		},
		{
			name:         "relative subdirectory path",
			path:         "subdir/file.txt",
			containerCwd: "/workspace/agent-a",
			want:         "/workspace/agent-a/subdir/file.txt",
		},
		{
			name:         "absolute path passed through",
			path:         "/workspace/agent-a/file.txt",
			containerCwd: "/workspace/agent-b",
			want:         "/workspace/agent-a/file.txt",
		},
		{
			name:         "dot path",
			path:         ".",
			containerCwd: "/workspace/agent-a",
			want:         "/workspace/agent-a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSandboxPath(tt.path, tt.containerCwd)
			if got != tt.want {
				t.Errorf("ResolveSandboxPath(%q, %q) = %q, want %q", tt.path, tt.containerCwd, got, tt.want)
			}
		})
	}
}
