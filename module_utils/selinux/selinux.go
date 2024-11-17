package selinux

import (
	"bytes"
	"fmt"
	sl "github.com/opencontainers/selinux/go-selinux"
	"github.com/scylladb/gosible/config"
	pathUtils "github.com/scylladb/gosible/utils/path"
	"os"
	"os/exec"
	"strings"
)

type Selinux struct {
	enabled        bool
	mlsEnabled     *bool
	initialContext []string
	specialFs      []string
}

func New() *Selinux {
	return &Selinux{
		specialFs:      config.Manager().Settings.DEFAULT_SELINUX_SPECIAL_FS,
		enabled:        sl.GetEnabled(),
		initialContext: []string{"", "", ""},
	}
}

// getIsMlsEnabled load selinux mlsEnabled data.
func (se *Selinux) getIsMlsEnabled() error {
	// There must be an easier and more reliable way
	// There's native C command to get this info, but we cannot use it as we can't use CGO.
	// https://man7.org/linux/man-pages/man3/is_selinux_enabled.3.html
	path, err := exec.LookPath("sestatus")
	if err != nil {
		// sestatus not found
		se.mlsEnabled = new(bool)
		return nil
	}
	res, err := exec.Command(path).Output()
	if err != nil {
		return err
	}
	se.mlsEnabled = new(bool)
	for _, line := range bytes.Split(res, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("Policy MLS status:")) {
			*se.mlsEnabled = isEnabled(line)
		}
	}
	if *se.mlsEnabled {
		se.initialContext = append(se.initialContext, "")
	}

	return nil
}

func isEnabled(line []byte) bool {
	return bytes.HasSuffix(line, []byte("enabled"))
}

func (se *Selinux) MlsEnabled() (bool, error) {
	if !se.IsInit() {
		if err := se.getIsMlsEnabled(); err != nil {
			return false, err
		}
	}
	return *se.mlsEnabled, nil
}

func (se *Selinux) Enabled() bool {
	return se.enabled
}

func (se *Selinux) InitialContext() ([]string, error) {
	if !se.IsInit() {
		if err := se.getIsMlsEnabled(); err != nil {
			return nil, err
		}
	}

	return se.initialContext, nil
}

func (se *Selinux) IsInit() bool {
	return se.mlsEnabled != nil
}

func (se *Selinux) DefaultContext(path string) ([]string, error) {
	if !se.IsInit() {
		if err := se.getIsMlsEnabled(); err != nil {
			return nil, err
		}
	}

	ctx := se.initialContext
	if !se.enabled {
		return ctx, nil
	}
	fCtx, err := se.Context(path)
	if err != nil {
		return ctx, nil
	}
	return fCtx, nil
}

func (se *Selinux) Context(path string) ([]string, error) {
	if !se.IsInit() {
		if err := se.getIsMlsEnabled(); err != nil {
			return nil, err
		}
	}

	ctx := se.initialContext
	if !se.enabled {
		return ctx, nil
	}
	ret, err := se.LGetFileConRaw(path)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve selinux context, %v", err)
	}
	// Limit split to 4 because the selevel, the last in the list, may contain ':' characters
	return strings.SplitN(ret, ":", 4), nil
}

// IsSpecialPath Returns true, selinux_context if the given path is on a
// NFS or other 'special' fs  mount point, otherwise the return will be false, nil.
func (se *Selinux) IsSpecialPath(path string) (bool, []string, error) {
	content, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false, nil, nil
	}
	mountPoint, err := pathUtils.FindMountPoint(path)
	if err != nil {
		return false, nil, nil
	}

	mountData := strings.Split(string(content), "\n")
	for _, line := range mountData {
		lineSplit := strings.SplitN(line, " ", 5)
		if lineSplit[1] == mountPoint {
			for _, fs := range se.specialFs {
				if strings.Contains(lineSplit[2], fs) {
					spContext, err := se.Context(path)
					if err == nil {
						return true, spContext, nil
					}
					return false, nil, err
				}
			}
		}
	}
	return false, nil, nil
}

func (se *Selinux) LSetFileCon(path string, context []string) error {
	return sl.LsetFileLabel(path, strings.Join(context, ":"))
}

func (se *Selinux) LGetFileConRaw(path string) (string, error) {
	return sl.LfileLabel(path)
}
