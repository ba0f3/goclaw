# Workspace, Sandbox, and Privacy

This document explains how GoClaw resolves the active workspace, how that workspace is mounted into the sandbox, and how privacy/isolation is enforced across personal and team contexts.

## 1. Core Mental Model

There is exactly one active workspace root per run context.

- File tools (`read_file`, `write_file`, `list_files`, `edit`) resolve against the active path.
- `exec`/`shell` sandbox mounts workspaces at their **host absolute paths** inside the container. This means paths are consistent whether sandboxed or not — `/data/teams/{teamID}/file.md` is the same path inside and outside the sandbox.
- Team context mounts the team workspace as a secondary bind alongside the primary (personal) workspace.

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

## 3. Team vs Personal: Two Resolution Paths

When an agent is part of a team, there are two distinct scenarios that determine which workspace becomes active. The distinction matters especially for sandbox mounts.

### Scenario A — Dispatched Task (member receiving a task from leader)

The leader sets `RunRequest.TeamWorkspace` explicitly when dispatching. In `injectContext` ([`internal/agent/loop_context.go`](../internal/agent/loop_context.go)):

```
req.TeamWorkspace != ""
→ WithToolTeamWorkspace(ctx, req.TeamWorkspace)
→ WithToolWorkspace(ctx, req.TeamWorkspace)   ← personal workspace is replaced
```

The team workspace **becomes the primary workspace**. All file tools and the sandbox mount resolve against it.

### Scenario B — Leader chatting directly (inbound message, no dispatch)

When a user talks directly to a leader agent (no `req.TeamWorkspace`), the team workspace is auto-resolved from the DB and stored as a **secondary** context key:

```
req.TeamWorkspace == "" && agent has team membership
→ WithToolTeamWorkspace(ctx, wsDir)   ← secondary only
   (personal workspace stays as primary ToolWorkspace)
```

The leader's **personal workspace remains primary** for resolving relative paths. However, the system prompt explicitly tells the leader about the team workspace path and instructs it to save team-relevant files there:

```
Team shared workspace: /path/to/.goclaw/workspace/teams/{teamID}
All team files visible to all members. When delegating, members can ONLY access team workspace files.
Default workspace (relative paths) = personal. Files in task descriptions auto-copied to team workspace.
```

So when you chat with the leader and it decides a file belongs to the team (e.g. you ask it to prepare something for the team), it will use the **absolute team workspace path** with `write_file`. The `allowedWithTeamWorkspace()` merge permits this write even though the team workspace is not the primary. This is intentional — relative paths land in personal, absolute team-workspace paths land in team.

### Quick Reference

| Scenario | Relative path resolves to | Absolute team-ws path resolves to | Sandbox mounts |
|---|---|---|---|
| No team | Personal | N/A | Personal (at host path) |
| Leader: direct chat | Personal | **Team** (via `allowedWithTeamWorkspace`) | Personal + Team (both at host paths) |
| Member: dispatched task | Team (overrides personal) | Team | Team (at host path) |
| Member: direct chat (no task) | Personal | Team (if team WS is set) | Personal + Team (both at host paths) |

### How File Tools Use Both

`allowedWithTeamWorkspace()` merges both paths into the allowed prefixes for every file tool call ([`internal/tools/filesystem.go`](../internal/tools/filesystem.go)):

```go
func allowedWithTeamWorkspace(ctx context.Context, base []string) []string {
    teamWs := ToolTeamWorkspaceFromCtx(ctx)
    // ... appends teamWs to allowed prefixes
}
```

This applies regardless of sandbox state. **File tools** (`read_file`, `write_file`, `edit`, `list_files`) can always access both personal and team workspace paths via this merge.

## 3.5. Group Chat vs Direct Message

These two axes are independent and both affect the resolved path:

| Axis | Config | Effect |
|---|---|---|
| Personal workspace isolation | `workspace_sharing.shared_dm`, `shared_group` | Whether different users/chats share the same personal workspace folder |
| Team workspace scope | `workspace_scope` on the team settings | Whether different users/chats share the same team workspace folder |

### Personal workspace: group chat is never shared

In `shouldShareWorkspace()` ([`internal/agent/loop_utils.go`](../internal/agent/loop_utils.go)), group-chat userIDs (`group:{channel}:{chatID}`) **always** get their own subfolder regardless of any `SharedGroup` config:

```go
if strings.HasPrefix(userID, "group:") {
    return false  // hard-coded: group chat never collapses into shared root
}
```

For DMs, `SharedDM=true` collapses all DM users into the same workspace root.

### Team workspace: `workspace_scope` controls it

For the team workspace auto-resolve path, the chat segment used is `req.ChatID` (falling back to `req.UserID`):

```go
wsChat := req.ChatID  // the actual Telegram/Zalo chatID
shared := tools.IsSharedWorkspace(team.Settings)  // workspace_scope == "shared"
wsDir = ResolveWorkspace(dataDir, TenantLayer, TeamLayer(teamID), UserChatLayer(wsChat, shared))
```

- `workspace_scope = "shared"`: `UserChatLayer` is a no-op → `{dataDir}/teams/{teamID}` for everyone
- `workspace_scope = "isolated"` (default): `UserChatLayer` appends `wsChat` → `{dataDir}/teams/{teamID}/{chatID}`

### Your concrete scenario: agent workspace shared + team workspace shared

```
Agent personal workspace_sharing:
  SharedDM    = true   → DMs collapse into base workspace root
  SharedGroup = false  → group chats each get their own subfolder

Team workspace_scope = "shared"  → everyone sees the same /teams/{teamID}
```

| Who | peerKind | Personal `ToolWorkspace` resolves to | Team `ToolTeamWorkspace` resolves to |
|---|---|---|---|
| You in group chat | group | `{ws}/{chatID}/` | `{data}/teams/{teamID}/` |
| Other user in DM | direct | `{ws}/` (shared root, no subfolder) | `{data}/teams/{teamID}/` |

Both end up with the **same team workspace path** because `workspace_scope = "shared"`. But their personal workspaces differ:
- **Your group chat**: path includes the group `chatID` subfolder (isolation enforced)
- **DM user**: path is the shared root (no per-user subfolder, `SharedDM=true`)

When the leader saves team-relevant files (using the absolute team workspace path), both of you see the same files in the team workspace. This is the intended behavior.

## 4. Sandbox Mount Model

Sandbox backends (`bwrap` and Docker) mount workspaces at their **host absolute paths** inside the container. This means:

- `/home/user/.goclaw/workspace/agents/leader/` on the host → same path inside the sandbox
- `/data/teams/{teamID}/` on the host → same path inside the sandbox
- Working directory (`cwd`) inside the sandbox = the primary workspace host path

```
SandboxHostMountRoot(ctx, registryWorkspace)
  → ToolWorkspaceFromCtx(ctx)    ← primary workspace
  → mounted at the same host path inside sandbox

ExtraMounts (team workspace)
  → ToolTeamWorkspaceFromCtx(ctx)  ← secondary workspace
  → mounted at the same host path inside sandbox
```

This is resolved in [`internal/tools/sandbox_utils.go`](../internal/tools/sandbox_utils.go) and [`internal/sandbox/bwrap.go`](../internal/sandbox/bwrap.go) / [`internal/sandbox/docker.go`](../internal/sandbox/docker.go).

### Why host-path mounting?

With the old `/workspace` remapping model, agents that learned paths from file tool outputs (e.g., `/data/teams/abc123/file.md`) couldn't use those same paths inside `exec` commands when sandboxed, because inside the sandbox the file was at `/workspace/teams/abc123/file.md`. This broke the agent's ability to consistently reference files across tool calls.

Host-path mounting eliminates this discrepancy: the same absolute path works everywhere.

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
- Paths already under a mounted workspace bind are skipped.
- Mirrors are always read-only.

## 5. Privacy Boundaries

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

## 6. Reading Debug Logs Correctly

If you see:

- `--bind /host/some/team/path /host/some/team/path`
- `resolved_workspace=/host/some/team/path`

that means the run is using the team workspace as its primary root (Scenario A — dispatched task).

If you see the personal workspace path in those fields but also `extra_mounts` in the log, the agent is in Scenario B (direct chat with leader) and the team workspace is mounted as a secondary bind.

The canonical in-sandbox workspace root is now the **host absolute path**, not `/workspace`.

## 7. Practical Guidance

- **Absolute paths are now safe** inside sandboxed `exec` — they resolve to the same path as on the host. This is the primary benefit of host-path mounting.
- **Relative paths** still resolve from the primary workspace (personal for leaders, team for dispatched tasks).
- **Team workspace** is automatically mounted for leaders in direct chat when sandbox is enabled. No configuration change needed.
- **File tools and exec tools are now consistent** — both see the same absolute paths.
- If you need `exec` commands to access paths outside both workspaces, add them to `sandbox.read_only_host_paths` in `config.json`.
