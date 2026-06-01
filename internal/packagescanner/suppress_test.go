package packagescanner

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestModifyNPMJSON(t *testing.T) {
	body := []byte(`{
		"time": {"1.0.0": "2020-01-01T00:00:00Z", "2.0.0": "2026-06-01T00:00:00Z"},
		"versions": {"1.0.0": {}, "2.0.0": {}},
		"dist-tags": {"latest": "2.0.0"}
	}`)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	
	// Min age 48h, 2.0.0 is brand new
	modified := modifyNPMJSON(body, headers, 48*time.Hour)
	
	var data map[string]any
	json.Unmarshal(modified, &data)
	
	versions := data["versions"].(map[string]any)
	if _, ok := versions["2.0.0"]; ok {
		t.Errorf("expected 2.0.0 to be removed")
	}
	if _, ok := versions["1.0.0"]; !ok {
		t.Errorf("expected 1.0.0 to be kept")
	}
}
