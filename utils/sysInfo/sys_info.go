package sysInfo

import (
	"github.com/scylladb/gosible/module_utils/gosibleModule"
	"github.com/scylladb/gosible/utils/distro"
	pathUtils "github.com/scylladb/gosible/utils/path"
	"runtime"
	"strings"
)

func capitalize(s string) string {
	return strings.ToUpper(s[:1]) + s[1:]
}

func Platform() string {
	// capitalize to match ansible
	return capitalize(runtime.GOOS)
}

func DistributionVersion() string {
	versionBest := distro.Version(true)
	version := distro.Version(false)
	distroName := distro.Id()

	if version != "" {
		if distroName == "centos" {
			version = strings.Join(strings.Split(versionBest, ".")[:2], ".")
		}
		if distroName == "debian" {
			version = versionBest
		}
	}

	return version
}

func Distribution() string {
	id := distro.Id()

	if Platform() == "Linux" {
		switch id {
		case "Amzn":
			return "Amazon"
		case "Rhel":
			return "Redhat"
		case "":
			return "OtherLinux"
		}
	}
	return id
}

// TODO move to or use in ServiceMgrFactCollector once we implent facts.
func IsSystemdManaged[P gosibleModule.Validatable](module *gosibleModule.GosibleModule[P]) (bool, error) {
	_, err := module.GetBinPath("systemctl", nil, true)
	if err != nil {
		return false, err
	}
	for _, canary := range []string{"/run/systemd/system/", "/dev/.run/systemd/", "/dev/.systemd/"} {
		exists, err := pathUtils.Exists(canary)
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

// GetPlatformSpecificValue retrieves value based on current system from map of type platform:distribution:value.
// A Generic platform is a fallback platform.
// An empty string is a default distribution fallback for given platform.
func GetPlatformSpecificValue[V any](systemMap map[string]map[string]V) V {
	pMap, ok := systemMap[Platform()]
	if ok {
		dist, ok := pMap[Distribution()]
		if ok {
			return dist
		}
		return pMap[""]
	}
	pMap, ok = systemMap["Generic"]
	if ok {
		return pMap[""]
	}
	var zero V
	return zero
}
