package packagescanner

import "strings"

type Ecosystem string

const (
	EcosystemNPM       Ecosystem = "npm"
	EcosystemPyPI      Ecosystem = "pypi"
	EcosystemPackagist Ecosystem = "packagist"
	EcosystemNuGet     Ecosystem = "nuget"
	EcosystemMaven     Ecosystem = "maven"
)

func (e Ecosystem) MalwareDBPath() string {
	switch e {
	case EcosystemNPM:
		return "malware_predictions.json"
	case EcosystemPyPI:
		return "malware_pypi.json"
	case EcosystemPackagist:
		return "malware_packagist.json"
	case EcosystemNuGet:
		return "malware_nuget.json"
	case EcosystemMaven:
		return "malware_maven.json"
	}
	return ""
}

func (e Ecosystem) ReleasesPath() string {
	switch e {
	case EcosystemNPM:
		return "releases/npm.json"
	case EcosystemPyPI:
		return "releases/pypi.json"
	case EcosystemPackagist:
		return "releases/packagist.json"
	case EcosystemNuGet:
		return "releases/nuget.json"
	case EcosystemMaven:
		return "releases/maven.json"
	}
	return ""
}

type registryEntry struct {
	ecosystem Ecosystem
	hosts     []string
}

var registries = []registryEntry{
	{
		ecosystem: EcosystemNPM,
		hosts: []string{
			"registry.npmjs.org",
			"registry.yarnpkg.com",
			"registry.npmjs.com",
		},
	},
	{
		ecosystem: EcosystemPyPI,
		hosts: []string{
			"files.pythonhosted.org",
			"pypi.org",
			"pypi.python.org",
			"pythonhosted.org",
		},
	},
	{
		ecosystem: EcosystemPackagist,
		hosts: []string{
			"repo.packagist.org",
			"packagist.org",
		},
	},
	{
		ecosystem: EcosystemNuGet,
		hosts: []string{
			"api.nuget.org",
			"www.nuget.org",
		},
	},
	{
		ecosystem: EcosystemMaven,
		hosts: []string{
			"repo1.maven.org",
			"repo.maven.apache.org",
			"maven.google.com",
			"dl.google.com",
			"jcenter.bintray.com",
		},
	},
}

func DetectEcosystem(host string) Ecosystem {
	h := strings.ToLower(host)
	for _, r := range registries {
		for _, hostPattern := range r.hosts {
			if h == hostPattern || strings.HasSuffix(h, "."+hostPattern) {
				return r.ecosystem
			}
		}
	}
	return ""
}

func IsRegistryHost(host string) bool {
	return DetectEcosystem(host) != ""
}