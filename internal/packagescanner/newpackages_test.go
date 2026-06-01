package packagescanner

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewPackagesDB_IsNewlyReleased(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/npm.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"package_name": "newpkg", "version": "1.0.0", "released_on": 9999999999}]`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	// 10000000000 is far in the future
	db := NewNewPackagesDB(server.URL, 48)
	db.EnsureFetched(EcosystemNPM)

	if !db.IsNewlyReleased(EcosystemNPM, "newpkg", "1.0.0") {
		t.Errorf("expected newpkg 1.0.0 to be newly released")
	}
}
