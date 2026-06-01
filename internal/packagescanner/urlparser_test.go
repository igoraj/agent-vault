package packagescanner

import (
	"testing"
)

func TestParseDownloadURL(t *testing.T) {
	tests := []struct {
		eco      Ecosystem
		rawURL   string
		wantName string
		wantVer  string
		wantOk   bool
	}{
		{EcosystemNPM, "https://registry.npmjs.org/pkg/-/pkg-1.0.0.tgz", "pkg", "1.0.0", true},
		{EcosystemPyPI, "https://files.pythonhosted.org/packages/source/p/pkg/pkg-1.0.0.tar.gz", "pkg", "1.0.0", true},
		{EcosystemPackagist, "https://repo.packagist.org/vendor/pkg/1.0.0-hash.zip", "vendor/pkg", "1.0.0", true},
		{EcosystemNuGet, "https://api.nuget.org/v3/flatcontainer/pkg/1.0.0/pkg.1.0.0.nupkg", "flatcontainer", "pkg", true},
		{EcosystemMaven, "https://repo1.maven.org/maven2/com/example/pkg/1.0.0/pkg-1.0.0.jar", "maven2.com.example:pkg", "1.0.0", true},
	}

	for _, tt := range tests {
		got, ok := ParseDownloadURL(tt.eco, tt.rawURL)
		if ok != tt.wantOk {
			t.Errorf("ParseDownloadURL(%s, %s) ok = %v, want %v", tt.eco, tt.rawURL, ok, tt.wantOk)
			continue
		}
		if ok && (got.Name != tt.wantName || got.Version != tt.wantVer) {
			t.Errorf("ParseDownloadURL(%s, %s) = %+v, want %+v", tt.eco, tt.rawURL, got, PackageRef{tt.wantName, tt.wantVer})
		}
	}
}
