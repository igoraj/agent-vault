package packagescanner

import (
	"net/url"
	"path"
	"strings"
)

type PackageRef struct {
	Name    string
	Version string
}

func IsDownloadURL(eco Ecosystem, rawURL string) bool {
	_, ok := ParseDownloadURL(eco, rawURL)
	return ok
}

func IsMetadataURL(eco Ecosystem, rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	p := u.Path
	switch eco {
	case EcosystemNPM:
		if strings.HasSuffix(p, ".tgz") {
			return false
		}
		if strings.Contains(p, "/-/") {
			return false
		}
		return true
	case EcosystemPyPI:
		return isPyPIMetadataURL(p)
	default:
		return false
	}
}

func ParseDownloadURL(eco Ecosystem, rawURL string) (PackageRef, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return PackageRef{}, false
	}
	p := u.Path

	switch eco {
	case EcosystemNPM:
		return parseNPMDownload(p)
	case EcosystemPyPI:
		return parsePyPIDownload(p)
	case EcosystemPackagist:
		return parsePackagistDownload(p)
	case EcosystemNuGet:
		return parseNuGetDownload(p)
	case EcosystemMaven:
		return parseMavenDownload(p)
	}
	return PackageRef{}, false
}

func parseNPMDownload(p string) (PackageRef, bool) {
	if !strings.HasSuffix(p, ".tgz") {
		return PackageRef{}, false
	}
	sep := strings.Index(p, "/-/")
	if sep < 0 {
		return PackageRef{}, false
	}
	pkgName := strings.TrimPrefix(p[:sep], "/")
	fileName := p[sep+3:]
	fileName = strings.TrimSuffix(fileName, ".tgz")

	var version string
	if strings.HasPrefix(pkgName, "@") {
		lastSlash := strings.LastIndex(pkgName, "/")
		if lastSlash < 0 {
			return PackageRef{}, false
		}
		shortName := pkgName[lastSlash+1:]
		if strings.HasPrefix(fileName, shortName+"-") {
			version = fileName[len(shortName)+1:]
		}
	} else {
		if strings.HasPrefix(fileName, pkgName+"-") {
			version = fileName[len(pkgName)+1:]
		}
	}
	if version == "" {
		return PackageRef{}, false
	}
	return PackageRef{Name: pkgName, Version: version}, true
}

func parsePyPIDownload(p string) (PackageRef, bool) {
	fileName := path.Base(p)
	if fileName == "" || fileName == "/" {
		return PackageRef{}, false
	}
	fileName = decodeURIComponent(fileName)

	if strings.HasSuffix(fileName, ".whl") || strings.HasSuffix(fileName, ".whl.metadata") {
		base := strings.TrimSuffix(strings.TrimSuffix(fileName, ".metadata"), ".whl")
		firstDash := strings.Index(base, "-")
		if firstDash <= 0 {
			return PackageRef{}, false
		}
		pkgName := base[:firstDash]
		rest := base[firstDash+1:]
		secondDash := strings.Index(rest, "-")
		var version string
		if secondDash >= 0 {
			version = rest[:secondDash]
		} else {
			version = rest
		}
		if version == "" || version == "latest" || pkgName == "" {
			return PackageRef{}, false
		}
		return PackageRef{Name: pkgName, Version: version}, true
	}

	sdistPatterns := []string{".tar.gz", ".tar.bz2", ".tar.xz", ".zip"}
	for _, suffix := range sdistPatterns {
		if strings.HasSuffix(fileName, suffix) || strings.HasSuffix(fileName, suffix+".metadata") {
			base := strings.TrimSuffix(strings.TrimSuffix(fileName, ".metadata"), suffix)
			lastDash := strings.LastIndex(base, "-")
			if lastDash <= 0 {
				return PackageRef{}, false
			}
			pkgName := base[:lastDash]
			version := base[lastDash+1:]
			if version == "" || version == "latest" || pkgName == "" {
				return PackageRef{}, false
			}
			return PackageRef{Name: pkgName, Version: version}, true
		}
	}
	return PackageRef{}, false
}

func parsePackagistDownload(p string) (PackageRef, bool) {
	p = strings.TrimPrefix(p, "/")
	parts := strings.SplitN(p, "/", 3)
	if len(parts) < 3 {
		return PackageRef{}, false
	}
	vendor := parts[0]
	packageName := parts[1]
	rest := parts[2]

	dashIdx := strings.LastIndex(rest, "-")
	if dashIdx < 0 {
		return PackageRef{}, false
	}
	version := rest[:dashIdx]
	if version == "" {
		return PackageRef{}, false
	}
	fullName := vendor + "/" + packageName
	return PackageRef{Name: fullName, Version: version}, true
}

func parseNuGetDownload(p string) (PackageRef, bool) {
	p = strings.TrimPrefix(p, "/")
	parts := strings.SplitN(p, "/", 4)
	if len(parts) < 4 {
		return PackageRef{}, false
	}
	pkgName := parts[1]
	version := parts[2]
	if pkgName == "" || version == "" {
		return PackageRef{}, false
	}
	return PackageRef{Name: pkgName, Version: version}, true
}

func parseMavenDownload(p string) (PackageRef, bool) {
	p = strings.TrimPrefix(p, "/")
	parts := strings.Split(p, "/")
	if len(parts) < 4 {
		return PackageRef{}, false
	}
	fileName := parts[len(parts)-1]
	version := parts[len(parts)-2]
	artifactId := parts[len(parts)-3]
	groupParts := parts[:len(parts)-3]
	group := strings.Join(groupParts, ".")
	fullName := group + ":" + artifactId
	if strings.HasPrefix(fileName, artifactId+"-") {
		return PackageRef{Name: fullName, Version: version}, true
	}
	return PackageRef{}, false
}

func isPyPIMetadataURL(p string) bool {
	return strings.HasPrefix(p, "/simple/") || strings.HasPrefix(p, "/pypi/")
}

func decodeURIComponent(s string) string {
	decoded, err := url.QueryUnescape(s)
	if err != nil {
		return s
	}
	return decoded
}