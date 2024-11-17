package config

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/scylladb/gosible/utils/path"
	"golang.org/x/sys/unix"
	"gopkg.in/ini.v1"
	"os"
	"path/filepath"
	"strings"
)

func findCwdAnsibleConfig() (cwdConfigPath string, warnCwdPublic bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	perms, err := os.Stat(cwd)
	if err != nil {
		return "", false
	}

	cwdConfigPath = filepath.Join(cwd, "ansible.cfg")

	if uint32(perms.Mode())&unix.S_IWOTH != 0 {
		// Working directory is world writable so we'll skip it.
		// Still have to look for a file here, though, so that we know if we have to warn
		if exists, _ := pathUtils.Exists(cwdConfigPath); exists {
			warnCwdPublic = true
		}
		cwdConfigPath = ""
	}
	return
}

// Determine INI Config File location
// order (first found is used): ENV, CWD, HOME, /etc/ansible
func findIniConfigFile() string {
	var potentialPaths []string

	pathFromEnv, ok := os.LookupEnv("ANSIBLE_CONFIG")
	if ok {
		pathFromEnv = pathUtils.UnfrackPath(pathFromEnv, pathUtils.UnfrackOptions{NoFollow: true})
		if isDir, err := pathUtils.IsDir(pathFromEnv); err == nil && isDir {
			pathFromEnv = filepath.Join(pathFromEnv, "ansible.cfg")
		}
		potentialPaths = append(potentialPaths, pathFromEnv)
	}

	cwdConfigPath, warnCwdPublic := findCwdAnsibleConfig()
	if cwdConfigPath != "" {
		potentialPaths = append(potentialPaths, cwdConfigPath)
	}

	perUserLocation := pathUtils.UnfrackPath("~/.ansible.cfg", pathUtils.UnfrackOptions{NoFollow: true})
	potentialPaths = append(potentialPaths, perUserLocation)

	// System location
	potentialPaths = append(potentialPaths, "/etc/ansible/ansible.cfg")

	var path string
	for _, candidate := range potentialPaths {
		if exists, _ := pathUtils.Exists(candidate); exists {
			if canRead, _ := pathUtils.HasReadPermission(candidate); canRead {
				path = candidate
				break
			}
		}
	}

	// Emit a warning if all the following are true:
	// * We did not use a config from ANSIBLE_CONFIG
	// * There's an ansible.cfg in the current working directory that we skipped
	if pathFromEnv != path && warnCwdPublic {
		// TODO emit warning -- display may not exist at this point yet; how to do it properly?
		cwd, _ := os.Getwd()
		docsUrl := "https://docs.ansible.com/ansible/devel/reference_appendices/config.html#cfg-in-world-writable-dir"
		fmt.Fprintf(os.Stderr, "Ansible is being run in a world writable directory (%s), ignoring it as an ansible.cfg source. ", cwd)
		fmt.Fprintf(os.Stderr, "For more information see %s\n", docsUrl)
	}
	return path
}

func getConfigType(filename string) (string, error) {
	ext := filepath.Ext(filename)
	if ext == ".ini" || ext == ".cfg" {
		return "ini", nil
	} else {
		return "", errors.New(fmt.Sprintf("unsupported configuration file extension for %s: %s", filename, ext))
	}
}

func parseConfigFile(path string, data *ConfigData) error {
	ftype, err := getConfigType(path)

	if err != nil {
		return err
	}
	if ftype == "ini" {
		return parseIniConfigFile(path, data)
	} else {
		return errors.New(fmt.Sprintf("unsupported configuration file type %s", ftype))
	}
}

// Strips comments and inline comments. Only permits ; as inline comment indicator, as Ansible does.
func stripCommentAndSpaces(line string) string {
	line = strings.TrimSpace(line)
	if len(line) == 0 {
		return line
	}
	if line[0] == ';' || line[0] == '#' {
		return ""
	}
	if idx := strings.Index(line, " ;"); idx > -1 {
		return strings.TrimSpace(line[:idx])
	}
	return line
}

func parseIniConfigFile(path string, data *ConfigData) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	var text []string
	for scanner.Scan() {
		text = append(text, stripCommentAndSpaces(scanner.Text()))
	}

	buf := bytes.NewBufferString(strings.Join(text, "\n"))
	cfg, err := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, buf)
	if err != nil {
		return err
	}

	for _, section := range cfg.Sections() {
		for _, key := range section.Keys() {
			if err = data.updateFieldFromIni(section.Name(), key); err != nil {
				return err
			}
		}
	}

	return nil
}
