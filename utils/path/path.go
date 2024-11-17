package pathUtils

import (
	"errors"
	"fmt"
	"github.com/ory/dockertest/v3/docker/pkg/mount"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type UnfrackOptions struct {
	Basedir  string
	NoFollow bool
}

func ExpandUser(x string) string {
	sep := string(os.PathSeparator)
	tildesep := "~" + sep

	home, err := os.UserHomeDir()
	if err != nil {
		// Using display.Fatal would cause an import cycle.
		// err != nil iff $HOME is not set in the env.
		log.Fatalln(err)
	}
	if x == "~" {
		return home
	} else if strings.HasPrefix(x, tildesep) {
		return home + sep + x[len(tildesep):]
	}
	return x
}

func ExpandUserAndEnv(x string) string {
	return ExpandUser(os.ExpandEnv(x))
}

func UserAndGroup(path string, expand bool) (int, int, error) {
	if expand {
		path = ExpandUserAndEnv(path)
	}
	stat, err := os.Lstat(path)
	if err != nil {
		return 0, 0, err
	}
	if sysStat, ok := stat.Sys().(*syscall.Stat_t); ok {
		return int(sysStat.Uid), int(sysStat.Gid), nil
	}
	return 0, 0, errors.New("couldn't retrieve file owner")
}

// FindMountPoint returns path's mount point
func FindMountPoint(path string) (string, error) {
	path = UnfrackPath(path, UnfrackOptions{})
	for {
		mounted, err := mount.Mounted(path)
		if err != nil {
			return "", err
		}
		if mounted {
			return path, nil
		}
	}
}

// LChmod is a chmod that doesn't follow symlinks.
func LChmod(path string, mode os.FileMode) error {
	// There's lchmod implementation on some unix system but not linux.
	// https://linux.die.net/man/2/fchmodat

	// Attempt to set the perms of the symlink but be careful not to change the perms of the underlying file while trying
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}
	if err = os.Chmod(path, mode); err != nil {
		return err
	}
	newStat, err := os.Stat(path)
	if err != nil {
		return err
	}
	if stat.Mode() != newStat.Mode() {
		return os.Chmod(path, stat.Mode())
	}
	return nil
}

// UnfrackPath returns a path that is free of symlinks (if options.follow=True), environment variables,
// relative path traversals and symbols (~)
// example: '$HOME/../../var/mail' becomes '/var/spool/mail'
func UnfrackPath(path string, options UnfrackOptions) string {
	// TODO verify if it is functionally equivalent to ansible's version
	// https://github.com/ansible/ansible/blob/2cbfd1e350cbe1ca195d33306b5a9628667ddda8/lib/ansible/utils/path.py#L31

	if options.Basedir == "" {
		options.Basedir, _ = os.Getwd()
	} else if isRegular, err := IsRegular(options.Basedir); err == nil && isRegular {
		options.Basedir = filepath.Dir(options.Basedir)
	}

	path = os.ExpandEnv(path)
	path = ExpandUser(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(options.Basedir, path)
	}

	if !options.NoFollow {
		if expandedPath, err := filepath.EvalSymlinks(path); err == nil {
			return expandedPath
		}
	}

	return filepath.Clean(path)
}

func IsDir(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fi.IsDir(), nil
}

func IsRegular(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fi.Mode().IsRegular(), nil
}

func Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func HasReadPermission(path string) (bool, error) {
	// TODO probably better to use "golang.org/x/sys/unix"
	file, err := os.OpenFile(path, os.O_RDONLY, 0666)
	if err != nil {
		if os.IsPermission(err) {
			return false, nil
		}
		return false, err
	} else {
		_ = file.Close()
		return true, nil
	}
}

func GetBinPath(arg string, optDirs []string, errOnNotFound bool) (string, error) {
	paths, err := getPaths(optDirs)
	if err != nil {
		return "", err
	}
	for _, dir := range paths {
		path := filepath.Join(dir, arg)
		isExec, err := isExecutable(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", err
		}
		if isExec {
			return path, nil
		}
	}
	if errOnNotFound {
		return "", fmt.Errorf("failed to find required executable \"%s\"", arg)
	}
	return "", nil
}

func IsExecutable(path string) (bool, error) {
	const execAnyMask = 0111
	stat, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	isExec := stat.Mode()&execAnyMask == execAnyMask
	return isExec, nil
}

func IsSymLink(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return stat.Mode()&os.ModeSymlink != 0, nil
}

func Access(path string, mode uint32) (bool, error) {
	err := syscall.Access(path, mode)
	switch err {
	case nil:
		return true, nil
	case syscall.EACCES:
		return false, nil
	default:
		return false, err
	}
}

// Copy copies files content from src to dest.
func Copy(src, dest string) error {
	dstFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	return CopyToFile(src, dstFile)
}

// CopyToFile copies files content from src to dest.
func CopyToFile(src string, dstFile *os.File) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	_, err = io.Copy(srcFile, dstFile)
	if err != nil {
		return err
	}
	return nil
}

// CopyStat copies permissions owner modification time from src to dest.
func CopyStat(src, dest string) error {
	stat, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err = os.Chmod(dest, stat.Mode()); err != nil {
		return err
	}
	if err = os.Chtimes(dest, time.Now(), stat.ModTime()); err != nil {
		return err
	}
	if sysStat, ok := stat.Sys().(*syscall.Stat_t); ok {
		err = os.Chown(dest, int(sysStat.Uid), int(sysStat.Gid))
		if err != nil && !os.IsPermission(err) {
			return err
		}
	} else {
		return errors.New("couldn't retrieve file owner")
	}
	return nil
}

// Vars used for unit testing.
var pathListSep = os.PathListSeparator
var getPathEnv = func() string { return os.Getenv("PATH") }
var isExecutable = IsExecutable
var exist = Exists

func getPaths(optDirs []string) ([]string, error) {
	sbinPaths := []string{"/sbin", "/usr/sbin", "/usr/local/sbin"}
	allPaths := sbinPaths
	allPaths = append(allPaths, optDirs...)

	paths := strings.Split(getPathEnv(), string(pathListSep))
	for _, dir := range allPaths {
		exists, err := exist(dir)
		if err != nil {
			return nil, err
		}
		if exists {
			paths = append(paths, dir)
		}
	}

	return paths, nil
}
