package packagescanner

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func ModifyMetadataRequestHeaders(eco Ecosystem, headers http.Header) {
	switch eco {
	case EcosystemNPM:
		accept := headers.Get("Accept")
		if strings.Contains(accept, "application/vnd.npm.install-v1+json") {
			headers.Set("Accept", "application/json")
		}
	case EcosystemPyPI:
		headers.Del("If-None-Match")
		headers.Del("If-Modified-Since")
	}
}

func ModifyMetadataResponse(eco Ecosystem, body []byte, headers http.Header, minAge time.Duration, isNewlyReleased func(name, version string) bool, pkgName string) []byte {
	if len(body) == 0 {
		return body
	}

	switch eco {
	case EcosystemNPM:
		return modifyNPMJSON(body, headers, minAge)
	case EcosystemPyPI:
		return modifyPyPIResponse(body, headers, isNewlyReleased, pkgName)
	case EcosystemPackagist, EcosystemNuGet, EcosystemMaven:
		// Not implemented
		return body
	}
	return body
}

func ClearCachingHeaders(headers http.Header) {
	headers.Del("Etag")
	headers.Del("Last-Modified")
	headers.Del("Cache-Control")
	headers.Del("Content-Length")
}

func ClearContentEncoding(headers http.Header) {
	headers.Del("Content-Encoding")
}

var htmlAnchorRE = regexp.MustCompile(`(?i)<a\b[^>]*href\s*=\s*["']([^"']+)["'][^>]*>[\s\S]*?</a>`)

func modifyPyPIResponse(body []byte, headers http.Header, isNewlyReleased func(name, version string) bool, pkgName string) []byte {
	ct := headers.Get("Content-Type")
	ct = strings.ToLower(ct)

	if strings.Contains(ct, "html") || strings.Contains(ct, "application/vnd.pypi.simple.v1+html") {
		return modifyPyPIHTML(body, isNewlyReleased, pkgName)
	}
	if strings.Contains(ct, "json") || strings.Contains(ct, "application/vnd.pypi.simple.v1+json") {
		return modifyPyPIJSON(body, isNewlyReleased, pkgName)
	}
	return body
}

func modifyPyPIHTML(body []byte, isNewlyReleased func(name, version string) bool, pkgName string) []byte {
	html := string(body)
	modified := false

	result := htmlAnchorRE.ReplaceAllStringFunc(html, func(anchor string) string {
		match := htmlAnchorRE.FindStringSubmatch(anchor)
		if len(match) < 2 {
			return anchor
		}
		href := match[1]

		resolved := resolvePyPIHref(href)

		pkg, ok := parsePyPIDownload(resolved)
		if !ok {
			pkg, ok = parsePyPIDownload(href)
			if !ok {
				return anchor
			}
		}
		if isNewlyReleased(pkgName, pkg.Version) {
			modified = true
			return ""
		}
		return anchor
	})

	if !modified {
		return body
	}
	return []byte(result)
}

func resolvePyPIHref(href string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "/") {
		return "https://files.pythonhosted.org" + href
	}
	parts := strings.Split(strings.Trim(href, "/"), "/")
	if len(parts) >= 3 {
		return "https://files.pythonhosted.org/packages/" + strings.Join(parts, "/")
	}
	return href
}

func modifyPyPIJSON(body []byte, isNewlyReleased func(name, version string) bool, pkgName string) []byte {
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return body
	}

	verifiedPkgName := pkgName
	if verifiedPkgName == "" {
		if info, ok := data["info"].(map[string]any); ok {
			if name, ok := info["name"].(string); ok {
				verifiedPkgName = name
			}
		}
	}

	modified := false

	if files, ok := data["files"].([]any); ok {
		var kept []any
		for _, f := range files {
			file, ok := f.(map[string]any)
			if !ok {
				kept = append(kept, f)
				continue
			}
			version := getVersionFromMetadataFile(file)
			if version != "" && isNewlyReleased(verifiedPkgName, version) {
				modified = true
				continue
			}
			kept = append(kept, f)
		}
		data["files"] = kept
	}

	if releases, ok := data["releases"].(map[string]any); ok {
		for ver := range releases {
			if isNewlyReleased(verifiedPkgName, ver) {
				delete(releases, ver)
				modified = true
			}
		}
	}

	if urls, ok := data["urls"].([]any); ok {
		var kept []any
		for _, u := range urls {
			entry, ok := u.(map[string]any)
			if !ok {
				kept = append(kept, u)
				continue
			}
			version := getVersionFromMetadataFile(entry)
			if version != "" && isNewlyReleased(verifiedPkgName, version) {
				modified = true
				continue
			}
			kept = append(kept, u)
		}
		data["urls"] = kept
	}

	if !modified {
		return body
	}

	updatePyPIInfoVersion(data)

	result, err := json.Marshal(data)
	if err != nil {
		return body
	}
	return result
}

func updatePyPIInfoVersion(data map[string]any) {
	info, ok := data["info"].(map[string]any)
	if !ok {
		return
	}
	oldVersion, ok := info["version"].(string)
	if !ok || oldVersion == "" {
		return
	}
	if _, exists := data["releases"].(map[string]any)[oldVersion]; exists {
		return
	}

	var available []string
	for ver := range data["releases"].(map[string]any) {
		available = append(available, ver)
	}
	if len(available) == 0 {
		if files, ok := data["files"].([]any); ok {
			seen := map[string]bool{}
			for _, f := range files {
				if file, ok := f.(map[string]any); ok {
					ver := getVersionFromMetadataFile(file)
					if ver != "" && !seen[ver] {
						seen[ver] = true
						available = append(available, ver)
					}
				}
			}
		}
	}
	if len(available) == 0 {
		if urls, ok := data["urls"].([]any); ok {
			seen := map[string]bool{}
			for _, u := range urls {
				if entry, ok := u.(map[string]any); ok {
					ver := getVersionFromMetadataFile(entry)
					if ver != "" && !seen[ver] {
						seen[ver] = true
						available = append(available, ver)
					}
				}
			}
		}
	}
	if len(available) == 0 {
		return
	}

	latest := selectLatestPyPIVersion(available)
	if latest != "" && latest != oldVersion {
		info["version"] = latest
	}
}

func selectLatestPyPIVersion(versions []string) string {
	if len(versions) == 0 {
		return ""
	}
	candidate := versions[0]
	for _, v := range versions[1:] {
		if comparePyPIVersions(v, candidate) > 0 {
			candidate = v
		}
	}
	return candidate
}



func comparePyPIVersions(a, b string) int {
	partsA := strings.FieldsFunc(a, func(r rune) bool {
		return r == '.' || r == '-' || r == '_'
	})
	partsB := strings.FieldsFunc(b, func(r rune) bool {
		return r == '.' || r == '-' || r == '_'
	})
	for i := 0; i < len(partsA) && i < len(partsB); i++ {
		var numA, numB int
		_, errA := fmt.Sscanf(partsA[i], "%d", &numA)
		_, errB := fmt.Sscanf(partsB[i], "%d", &numB)
		if errA == nil && errB == nil {
			if numA != numB {
				if numA > numB {
					return 1
				}
				return -1
			}
		} else {
			if partsA[i] != partsB[i] {
				if partsA[i] > partsB[i] {
					return 1
				}
				return -1
			}
		}
	}
	if len(partsA) > len(partsB) {
		return 1
	}
	if len(partsA) < len(partsB) {
		return -1
	}
	return 0
}

func getVersionFromMetadataFile(file map[string]any) string {
	if u, ok := file["url"].(string); ok && u != "" {
		if pkg, ok := parsePyPIDownload(u); ok {
			return pkg.Version
		}
	}
	if filename, ok := file["filename"].(string); ok && filename != "" {
		if pkg, ok := parsePyPIDownload(filename); ok {
			return pkg.Version
		}
	}
	return ""
}

func modifyNPMJSON(body []byte, headers http.Header, minAge time.Duration) []byte {
	ct := headers.Get("Content-Type")
	if !strings.Contains(strings.ToLower(ct), "application/json") {
		return body
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return body
	}

	timeMap, hasTime := data["time"].(map[string]any)
	versions, hasVersions := data["versions"].(map[string]any)
	distTags, hasDistTags := data["dist-tags"].(map[string]any)

	if !hasTime || !hasDistTags || !hasVersions {
		return body
	}

	cutOff := time.Now().Add(-minAge)
	hasLatest := false
	if latest, ok := distTags["latest"]; ok && latest != nil {
		hasLatest = true
	}

	modified := false
	for ver, ts := range timeMap {
		if ver == "created" || ver == "modified" {
			continue
		}
		tsStr, ok := ts.(string)
		if !ok {
			continue
		}
		t, err := time.Parse(time.RFC3339, tsStr)
		if err != nil {
			t, err = time.Parse(time.RFC3339Nano, tsStr)
			if err != nil {
				continue
			}
		}
		if t.After(cutOff) {
			delete(timeMap, ver)
			delete(versions, ver)
			for tag, dv := range distTags {
				if dv == ver {
					delete(distTags, tag)
				}
			}
			modified = true
		}
	}

	if modified && hasLatest {
		if _, ok := distTags["latest"]; !ok {
			if newLatest := calculateLatestTag(timeMap); newLatest != "" {
				distTags["latest"] = newLatest
			}
		}
	}

	if !modified {
		return body
	}

	result, err := json.Marshal(data)
	if err != nil {
		return body
	}
	return result
}

func calculateLatestTag(timeMap map[string]any) string {
	var latestVer string
	var latestTS time.Time
	for ver, ts := range timeMap {
		if ver == "created" || ver == "modified" {
			continue
		}
		tsStr, ok := ts.(string)
		if !ok {
			continue
		}
		t, err := time.Parse(time.RFC3339, tsStr)
		if err != nil {
			if t, err = time.Parse(time.RFC3339Nano, tsStr); err != nil {
				continue
			}
		}
		if latestTS.IsZero() || t.After(latestTS) {
			latestTS = t
			latestVer = ver
		}
	}
	return latestVer
}

func ExtractPackageNameFromMetadata(body []byte, headers http.Header) string {
	ct := headers.Get("Content-Type")
	if !strings.Contains(strings.ToLower(ct), "application/json") {
		return ""
	}
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return ""
	}
	if name, ok := data["name"].(string); ok {
		return name
	}
	return ""
}

func BlockResponseBody(msg string) (int, string) {
	return http.StatusForbidden, fmt.Sprintf("Blocked by package scanner: %s", msg)
}

func extractPyPIPackageNameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, p := range parts {
		if (p == "simple" || p == "pypi") && i+1 < len(parts) {
			name := parts[i+1]
			if decoded, err := url.QueryUnescape(name); err == nil {
				return decoded
			}
			return name
		}
	}
	return ""
}