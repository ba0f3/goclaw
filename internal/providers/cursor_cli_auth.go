package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// CursorAuthStatus holds the parsed result of `agent whoami --format json`.
type CursorAuthStatus struct {
	Status          string `json:"status"`
	IsAuthenticated bool   `json:"isAuthenticated"`
	HasAccessToken  bool   `json:"hasAccessToken"`
	HasRefreshToken bool   `json:"hasRefreshToken"`
	UserInfo        struct {
		Email string `json:"email,omitempty"`
	} `json:"userInfo"`
}

// CheckCursorAuthStatus runs `agent whoami --format json` using the given CLI
// path and returns the parsed authentication status.
func CheckCursorAuthStatus(ctx context.Context, cliPath string) (*CursorAuthStatus, error) {
	if cliPath == "" {
		cliPath = "agent"
	}

	resolvedPath, err := exec.LookPath(cliPath)
	if err != nil {
		return nil, fmt.Errorf("cursor agent binary not found at %q: %w", cliPath, err)
	}

	cmd := exec.CommandContext(ctx, resolvedPath, "whoami", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("cursor whoami failed: %w", err)
	}

	var status CursorAuthStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return nil, fmt.Errorf("failed to parse cursor auth status: %w", err)
	}
	return &status, nil
}
