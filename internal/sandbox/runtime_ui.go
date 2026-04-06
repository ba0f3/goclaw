package sandbox

import "context"

// RuntimeUISnapshot returns server-side flags for the settings UI (config.get).
// When mgr is nil (sandbox disabled), bwrap_cgroup_limits_active is still probed so the UI can
// preview whether limits would apply after enabling bubblewrap.
func RuntimeUISnapshot(mgr Manager) map[string]any {
	ctx := context.Background()
	bwrapBinOK := CheckBwrapAvailable(ctx) == nil
	snap := map[string]any{
		"bwrap_binary_ok": bwrapBinOK,
	}
	cgroupActive := false
	if mgr != nil {
		st := mgr.Stats()
		if b, ok := st["bwrap"].(map[string]any); ok {
			if v, ok := b["cgroup_limits_via_systemd"].(bool); ok {
				cgroupActive = v
			}
		}
	} else if bwrapBinOK {
		cgroupActive = probeSystemdRunScope(ctx)
	}
	snap["bwrap_cgroup_limits_active"] = cgroupActive
	return snap
}
