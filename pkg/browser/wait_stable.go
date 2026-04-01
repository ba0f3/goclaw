package browser

import (
	"strings"
	"time"

	"github.com/go-rod/rod"
)

// handleWaitStableError returns true when the error is safely ignorable.
func (m *Manager) handleWaitStableError(page *rod.Page, err error, action string) bool {
	if !isIgnorableWaitStableError(err) {
		return false
	}
	m.logger.Warn("wait stable unsupported, continuing",
		"action", action,
		"error", err.Error(),
	)
	// Fallback delay to give the page a brief chance to settle.
	time.Sleep(250 * time.Millisecond)
	return true
}

func isIgnorableWaitStableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UnknownDomain")
}
