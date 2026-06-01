package packagescanner

import (
	"testing"
)

func TestDetectEcosystem(t *testing.T) {
	tests := []struct {
		host string
		want Ecosystem
	}{
		{"registry.npmjs.org", EcosystemNPM},
		{"pypi.org", EcosystemPyPI},
		{"unknown.com", ""},
		{"sub.registry.npmjs.org", EcosystemNPM},
	}

	for _, tt := range tests {
		got := DetectEcosystem(tt.host)
		if got != tt.want {
			t.Errorf("DetectEcosystem(%s) = %s, want %s", tt.host, got, tt.want)
		}
	}
}

func TestIsRegistryHost(t *testing.T) {
	if !IsRegistryHost("registry.npmjs.org") {
		t.Errorf("IsRegistryHost(registry.npmjs.org) should be true")
	}
	if IsRegistryHost("example.com") {
		t.Errorf("IsRegistryHost(example.com) should be false")
	}
}
