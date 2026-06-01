package packagescanner

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Scanner struct {
	mu         sync.RWMutex
	malwareDB  *MalwareDB
	newPkgDB   *NewPackagesDB
	enabled    bool
	minAge     time.Duration
	baseURL    string
	logger     *slog.Logger

	customNpmRegistries  []string
	customPyPIRegistries []string
}

type ScannerConfig struct {
	MalwareDBEnabled     bool
	SuppressEnabled      bool
	MalwareAPIBaseURL    string
	MinimumPackageAge    time.Duration
	Logger               *slog.Logger
	CustomNPMRegistries  []string
	CustomPyPIRegistries []string
}

var DefaultMalwareAPIBaseURL = "https://malware-list.aikido.dev"
var DefaultMinimumPackageAge = 48 * time.Hour

func NewScanner(cfg ScannerConfig) *Scanner {
	baseURL := cfg.MalwareAPIBaseURL
	if baseURL == "" {
		baseURL = DefaultMalwareAPIBaseURL
	}
	minAge := cfg.MinimumPackageAge
	if minAge <= 0 {
		minAge = DefaultMinimumPackageAge
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	s := &Scanner{
		enabled:              cfg.MalwareDBEnabled || cfg.SuppressEnabled,
		minAge:               minAge,
		baseURL:              strings.TrimRight(baseURL, "/"),
		logger:               logger,
		customNpmRegistries:  cfg.CustomNPMRegistries,
		customPyPIRegistries: cfg.CustomPyPIRegistries,
	}
	if cfg.MalwareDBEnabled {
		s.malwareDB = NewMalwareDB(baseURL)
	}
	if cfg.SuppressEnabled {
		s.newPkgDB = NewNewPackagesDB(baseURL, int(minAge.Hours()))
	}
	return s
}

func (s *Scanner) Enabled() bool {
	return s.enabled
}

func (s *Scanner) SetEnabled(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = v
}

func (s *Scanner) DetectEcosystem(host string) Ecosystem {
	eco := DetectEcosystem(host)
	if eco != "" {
		return eco
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.customNpmRegistries {
		if strings.EqualFold(host, r) || strings.HasSuffix(strings.ToLower(host), "."+strings.ToLower(r)) {
			return EcosystemNPM
		}
	}
	for _, r := range s.customPyPIRegistries {
		if strings.EqualFold(host, r) || strings.HasSuffix(strings.ToLower(host), "."+strings.ToLower(r)) {
			return EcosystemPyPI
		}
	}
	return ""
}

func (s *Scanner) ensureDBs(eco Ecosystem) {
	if s.malwareDB != nil {
		if err := s.malwareDB.EnsureFetched(eco); err != nil {
			s.logger.Warn("packagescanner malware fetch failed", "ecosystem", eco, "error", err)
		} else {
			s.logger.Debug("packagescanner malware DB ready", "ecosystem", eco)
		}
	}
	if s.minAge > 0 && s.newPkgDB != nil {
		if err := s.newPkgDB.EnsureFetched(eco); err != nil {
			s.logger.Warn("packagescanner release-age fetch failed", "ecosystem", eco, "error", err)
		} else {
			s.logger.Debug("packagescanner release-age DB ready", "ecosystem", eco)
		}
	}
}

func (s *Scanner) CheckDownload(host string, rawURL string) *BlockResult {
	if !s.enabled {
		s.logger.Debug("packagescanner skipped (disabled)", "url", rawURL)
		return nil
	}
	eco := s.DetectEcosystem(host)
	if eco == "" {
		s.logger.Debug("packagescanner skipped (unknown ecosystem)", "host", host, "url", rawURL)
		return nil
	}

	pkg, ok := ParseDownloadURL(eco, rawURL)
	if !ok {
		s.logger.Debug("packagescanner skipped (not a download URL)", "ecosystem", eco, "host", host, "url", rawURL)
		return nil
	}

	s.ensureDBs(eco)

	if s.malwareDB != nil && s.malwareDB.IsMalware(eco, pkg.Name, pkg.Version) {
		s.logger.Warn("blocked malware package",
			"ecosystem", eco,
			"package", pkg.Name,
			"version", pkg.Version,
			"url", rawURL,
		)
		return &BlockResult{
			Blocked:  true,
			Reason:   "malware",
			Eco:      eco,
			Package:  pkg.Name,
			Version:  pkg.Version,
			HTTPCode: http.StatusForbidden,
			Message:  "Malicious package blocked",
		}
	}

	if s.minAge > 0 && s.newPkgDB != nil && s.newPkgDB.IsNewlyReleased(eco, pkg.Name, pkg.Version) {
		s.logger.Warn("blocked young package",
			"ecosystem", eco,
			"package", pkg.Name,
			"version", pkg.Version,
			"url", rawURL,
		)
		return &BlockResult{
			Blocked:  true,
			Reason:   "minimum_age",
			Eco:      eco,
			Package:  pkg.Name,
			Version:  pkg.Version,
			HTTPCode: http.StatusForbidden,
			Message:  "Package is too new (minimum age: " + s.minAge.String() + ")",
		}
	}

	s.logger.Debug("packagescanner passed", "ecosystem", eco, "package", pkg.Name, "version", pkg.Version, "url", rawURL)
	return nil
}

func (s *Scanner) ShouldModifyMetadata(host, rawURL string) (Ecosystem, bool) {
	if !s.enabled || s.minAge <= 0 || s.newPkgDB == nil {
		return "", false
	}
	eco := s.DetectEcosystem(host)
	if eco == "" {
		return "", false
	}
	if !IsMetadataURL(eco, rawURL) {
		return "", false
	}
	return eco, true
}

func (s *Scanner) ModifyRequestHeaders(eco Ecosystem, rawURL string, headers http.Header) {
	if !s.enabled {
		return
	}
	ModifyMetadataRequestHeaders(eco, headers)
}

func (s *Scanner) ModifyResponse(eco Ecosystem, rawURL string, body []byte, headers http.Header) []byte {
	if !s.enabled || len(body) == 0 || s.newPkgDB == nil {
		return body
	}

	if s.minAge <= 0 {
		return body
	}

	pkgName := ExtractPackageNameFromMetadata(body, headers)
	if pkgName == "" && eco == EcosystemPyPI {
		pkgName = extractPyPIPackageNameFromURL(rawURL)
	}

	return ModifyMetadataResponse(eco, body, headers, s.minAge, func(name, version string) bool {
		n := name
		if n == "" {
			n = pkgName
		}
		if n == "" {
			return false
		}
		return s.newPkgDB.IsNewlyReleased(eco, n, version)
	}, pkgName)
}

type BlockResult struct {
	Blocked  bool
	Reason   string
	Eco      Ecosystem
	Package  string
	Version  string
	HTTPCode int
	Message  string
}

var (
	scannerInstance *Scanner
	scannerMu       sync.Mutex
)

func InitGlobal(cfg ScannerConfig) *Scanner {
	scannerMu.Lock()
	defer scannerMu.Unlock()
	scannerInstance = NewScanner(cfg)
	if l := scannerInstance.logger; l != nil {
		l.Debug("packagescanner initialised",
			"malware_disabled", !cfg.MalwareDBEnabled,
			"suppress_disabled", !cfg.SuppressEnabled,
			"min_age", cfg.MinimumPackageAge.String(),
			"base_url", cfg.MalwareAPIBaseURL,
		)
	}
	return scannerInstance
}

func Global() *Scanner {
	scannerMu.Lock()
	defer scannerMu.Unlock()
	return scannerInstance
}

func SetGlobal(s *Scanner) {
	scannerMu.Lock()
	defer scannerMu.Unlock()
	scannerInstance = s
}