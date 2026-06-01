package packagescanner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// releaseSourceForEcosystem maps an Ecosystem to the expected "source" field
// value in the releases API response. safe-chain uses "npm" / "pypi" etc.
func releaseSourceForEcosystem(eco Ecosystem) string {
	switch eco {
	case EcosystemNPM:
		return "npm"
	case EcosystemPyPI:
		return "pypi"
	case EcosystemPackagist:
		return "packagist"
	case EcosystemNuGet:
		return "nuget"
	case EcosystemMaven:
		return "maven"
	}
	return string(eco)
}

type newPackageEntry struct {
	PackageName string `json:"package_name"`
	Version     string `json:"version"`
	ReleasedOn  int64  `json:"released_on"`
	Source      string `json:"source,omitempty"`
}

type NewPackagesDB struct {
	mu       sync.RWMutex
	entries  []newPackageEntry
	etag     string
	fetched  bool
	fetchErr error
	baseURL  string
	client   *http.Client
	minAge   time.Duration
}

func NewNewPackagesDB(baseURL string, minAgeHours int) *NewPackagesDB {
	if minAgeHours <= 0 {
		minAgeHours = 48
	}
	return &NewPackagesDB{
		baseURL: strings.TrimRight(baseURL, "/"),
		minAge:  time.Duration(minAgeHours) * time.Hour,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (db *NewPackagesDB) EnsureFetched(eco Ecosystem) error {
	db.mu.RLock()
	if db.fetched {
		db.mu.RUnlock()
		return db.fetchErr
	}
	db.mu.RUnlock()

	return db.fetch(eco)
}

func (db *NewPackagesDB) fetch(eco Ecosystem) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	path := eco.ReleasesPath()
	if path == "" {
		err := fmt.Errorf("unsupported ecosystem: %s", eco)
		db.fetched = true
		db.fetchErr = err
		return err
	}
	url := db.baseURL + "/" + path
	resp, err := db.client.Get(url)
	if err != nil {
		db.fetched = true
		db.fetchErr = err
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		db.fetched = true
		db.fetchErr = err
		return err
	}
	var entries []newPackageEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		db.fetched = true
		db.fetchErr = err
		return err
	}
	db.entries = entries
	db.etag = resp.Header.Get("Etag")
	db.fetched = true
	db.fetchErr = nil
	return nil
}

func (db *NewPackagesDB) IsNewlyReleased(eco Ecosystem, name, version string) bool {
	db.mu.RLock()
	entries := db.entries
	db.mu.RUnlock()

	if len(entries) == 0 || name == "" || version == "" {
		return false
	}

	cutOff := time.Now().Add(-db.minAge)
	normName := normalizeName(eco, name)
	candidates := equivalentNames(eco, normName)

	expectedSource := releaseSourceForEcosystem(eco)

	for _, e := range entries {
		if e.Source != "" && strings.ToLower(e.Source) != expectedSource {
			continue
		}
		entryName := normalizeName(eco, e.PackageName)
		entryName = strings.ToLower(entryName)
		matched := false
		for _, c := range candidates {
			if entryName == c {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if e.Version != version {
			continue
		}
		released := time.Unix(e.ReleasedOn, 0)
		return released.After(cutOff)
	}
	return false
}

func equivalentNames(eco Ecosystem, name string) []string {
	switch eco {
	case EcosystemPyPI:
		sep := strings.NewReplacer("_", "-", ".", "-")
		hyphen := sep.Replace(name)
		underscore := strings.ReplaceAll(strings.ReplaceAll(name, "-", "_"), ".", "_")
		dot := strings.ReplaceAll(strings.ReplaceAll(name, "-", "."), "_", ".")
		seen := map[string]bool{}
		var result []string
		for _, n := range []string{name, hyphen, underscore, dot} {
			if !seen[n] {
				seen[n] = true
				result = append(result, n)
			}
		}
		return result
	default:
		return []string{name}
	}
}