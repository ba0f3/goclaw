# Workspace, Sandbox, and Privacy

This document explains how GoClaw resolves the active workspace, how that workspace is mounted into the sandbox, and how privacy/isolation is enforced across personal and team contexts.

## 1. Core Mental Model

There is exactly one active workspace root per run context.

- File tools (`read_file`, `write_file`, `list_files`, `edit`) resolve against the active path.
- `exec`/`shell` sandbox mounts that same active path at the sandbox workdir (default `/workspace`).
- Team context does not create a second "team tree" in the sandbox. Team vs personal depends on which host directory is selected as the single active workspace.

The source of truth for this model is `WorkspaceContext.ActivePath` in [`internal/workspace/workspace_context.go`](../internal/workspace/workspace_context.go).

## 2. How Workspace Resolution Works

The resolver in [`internal/workspace/resolver_impl.go`](../internal/workspace/resolver_impl.go) chooses one `ActivePath` using this priority:

1. Delegate context
2. Team context
3. Personal/predefined context

### Personal

- Open agents: per-user/per-chat workspace.
- Predefined/shared-memory style agents: shared workspace patterns based on settings.

### Team

- Team workspace root is under tenant data: `.../teams/{teamID}`.
- `workspace_scope = shared`: active path is team root.
- `workspace_scope = isolated`: active path is team root plus chat/user segment.

### Delegate

- Active path is delegate shared path.
- Additional read-only export paths can be present.

## 3. Sandbox Mount Model

Sandbox backends (`bwrap` and Docker) mount the active workspace as the primary bind:

- Host active workspace -> container workdir (`/workspace` by default).

This is built from `ToolWorkspaceFromCtx` via [`internal/tools/sandbox_utils.go`](../internal/tools/sandbox_utils.go).

### Read-only host mirrors

Some directories need to stay accessible by absolute host path from inside sandboxed commands. These are mounted read-only at the same absolute path:

- `${HOME}/.goclaw/skills`
- `${HOME}/.agents/skills`
- `{dataDir}/skills-store`
- tenant skills store from `config.TenantSkillsStoreDir(...)`

These are populated in [`internal/agent/resolver.go`](../internal/agent/resolver.go) and normalized by sandbox path filters in [`internal/sandbox/read_only_paths.go`](../internal/sandbox/read_only_paths.go).

Important constraints:

- Only existing directories are mounted.
- Duplicate paths are removed.
- Paths already under the primary workspace bind are skipped.
- Mirrors are always read-only.

## 4. Privacy Boundaries

Privacy is enforced by multiple layers, not by mount shape alone.

### Filesystem scope

- Personal/team/delegate boundaries are first defined by `ActivePath`.
- Tool-level boundary checks still apply (`restrict_to_workspace`, deny lists, allowed path overlays).

### Team visibility

- Shared mode: members share the same team path.
- Isolated mode: each chat/user gets a subdirectory under team root.
- Delegation can selectively expose shared paths plus read-only exports.

### Sandbox hardening

- `bwrap`: read-only system binds, tmpfs for `/tmp` and `/var`, namespace isolation.
- Docker: isolated container filesystem, policy flags, configurable network.
- Network is disabled by default in sandbox config unless explicitly enabled.

### Tool security overlays

Even if a path is mounted, shell/file policy can still deny access. For example, exec denies sensitive internal data roots except explicit exemptions in [`cmd/gateway_setup.go`](../cmd/gateway_setup.go).

## 5. Reading Debug Logs Correctly

If you see:

- `--bind /host/some/team/path /workspace`
- `resolved_workspace=/host/some/team/path`

that already means the run is using the team workspace as its primary root.

You should no longer expect or rely on a second team mount under `/workspace/teams/...`. The canonical in-sandbox workspace root is `/workspace`.

## 6. Practical Guidance

- Prefer relative paths in prompts/tools so path behavior is consistent across sandbox backends.
- Treat `/workspace` as the primary current project root inside sandboxed execution.
- Use absolute host paths only when needed (for example, managed/personal skills directories); these are provided through read-only mirrors.
