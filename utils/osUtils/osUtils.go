package osUtils

import (
	"os"
	"path"
	"path/filepath"
	"syscall"
)

func GetUmask() int {
	umask := syscall.Umask(0)
	_ = syscall.Umask(umask)
	return umask
}

func GetBinaryPath() (string, error) {
	binPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(binPath)
}

func GetBinaryDir() (string, error) {
	binPath, err := GetBinaryPath()
	if err != nil {
		return "", err
	}
	return path.Dir(binPath), nil
}
