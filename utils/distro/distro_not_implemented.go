//go:build !linux

package distro

func Id() string {
	return "not implemented"
}

func Version(_ bool) string {
	return "not implemented"
}
