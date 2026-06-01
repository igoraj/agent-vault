package packagescanner

import (
	"log/slog"
	"testing"
)

func TestScanner_CheckDownload(t *testing.T) {
	s := NewScanner(ScannerConfig{
		MalwareDBEnabled: true,
		SuppressEnabled:  true,
		Logger:           slog.Default(),
	})

	// Test disabling
	s.SetEnabled(false)
	res := s.CheckDownload("registry.npmjs.org", "https://registry.npmjs.org/pkg/-/pkg-1.0.0.tgz")
	if res != nil {
		t.Errorf("expected nil for disabled scanner")
	}

	s.SetEnabled(true)
}
