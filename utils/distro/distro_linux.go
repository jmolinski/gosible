package distro

import (
	"github.com/zcalusic/sysinfo"
	"strings"
)

var normalizedDistroId = map[string]string{
	"redhat": "rhel",
}

var si sysinfo.SysInfo

func init() {
	si.GetSysInfo()
}

func normalizeDistroId(id string) string {
	norm := strings.ReplaceAll(strings.ToLower(id), " ", "_")
	if v, ok := normalizedDistroId[norm]; ok {
		return v
	}
	return norm
}

func Id() string {
	// TODO check if name is the same for each distro.
	return normalizeDistroId(si.OS.Vendor)
}

func Version(best bool) string {
	if best {
		return si.OS.Release
	}
	return si.OS.Version
}
