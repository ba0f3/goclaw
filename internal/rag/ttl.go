package rag

import (
	"regexp"
	"time"
)

var (
	newsPattern   = regexp.MustCompile(`(?i)(^|\.)(news|blog)\.|/(article|news|post)/|/20[0-9]{2}/[0-9]{2}/[0-9]{2}/`)
	docsPattern   = regexp.MustCompile(`(?i)(^|\.)(docs)\.|/(documentation|docs|wiki)/`)
	socialPattern = regexp.MustCompile(`(?i)(twitter\.com|x\.com|reddit\.com|facebook\.com|instagram\.com)`)
	repoPattern   = regexp.MustCompile(`(?i)(github\.com/.+/blob/|raw\.githubusercontent\.com)`)
)

type TTLClassifier struct {
	defaultWebTTL time.Duration
}

func NewTTLClassifier(defaultWebTTL time.Duration) *TTLClassifier {
	if defaultWebTTL <= 0 {
		defaultWebTTL = 48 * time.Hour
	}
	return &TTLClassifier{defaultWebTTL: defaultWebTTL}
}

// ClassifyTTL is the package-level helper used by tools.
// Fallback TTL is 48h (matches config default).
func ClassifyTTL(rawURL string) time.Duration {
	return NewTTLClassifier(48 * time.Hour).ClassifyTTL(rawURL)
}

func (c *TTLClassifier) ClassifyTTL(rawURL string) time.Duration {
	if newsPattern.MatchString(rawURL) {
		return 6 * time.Hour
	}
	if docsPattern.MatchString(rawURL) {
		return 7 * 24 * time.Hour
	}
	if socialPattern.MatchString(rawURL) {
		return time.Hour
	}
	if repoPattern.MatchString(rawURL) {
		return 24 * time.Hour
	}
	return c.defaultWebTTL
}
