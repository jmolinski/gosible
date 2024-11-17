package template

import (
	"errors"
	"fmt"
	"github.com/hhkbp2/go-strftime"
	"github.com/scylladb/gosible/config"
	"github.com/scylladb/gosible/utils/types"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func GenerateAnsibleTemplateVars(p string, fullPath, destPath *string) (types.Vars, error) {
	var chosenPath string
	if fullPath == nil {
		chosenPath = p
	} else {
		chosenPath = *fullPath
	}

	templateUid, err := getTemplateUid(chosenPath)
	if err != nil {
		return nil, err
	}

	mtime, err := getModificationTime(chosenPath)
	if err != nil {
		return nil, err
	}
	tempVars := types.Vars{
		"template_path":     p,
		"template_mtime":    mtime,
		"template_uid":      fmt.Sprint(templateUid),
		"template_run_date": time.Now(),
	}
	tempVars["template_host"], err = getHost()
	if err != nil {
		return nil, err
	}

	if destPath != nil {
		tempVars["template_destpath"] = *destPath
	}
	if fullPath != nil {
		tempVars["template_fullpath"], err = filepath.Abs(*fullPath)
		if err != nil {
			return nil, err
		}
	}
	managedStr := formatString(config.Manager().Settings.DEFAULT_MANAGED_STR, map[string]interface{}{
		"host": tempVars["template_host"],
		"uid":  tempVars["template_uid"],
		"file": tempVars["template_path"],
	})

	tempVars["ansible_managed"] = strftime.Format(managedStr, mtime.Local())

	return tempVars, nil
}

func formatString(template string, replacements map[string]interface{}) string {
	args := make([]string, len(replacements)*2)
	for k, v := range replacements {
		args = append(args, fmt.Sprintf("{%s}", k))
		args = append(args, fmt.Sprint(v))
	}
	r := strings.NewReplacer(args...)

	return r.Replace(template)
}

func getModificationTime(path string) (time.Time, error) {
	data, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return data.ModTime(), nil
}

func getTemplateUid(path string) (interface{}, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, errors.New("conversion failure while getting file owner data")
	}
	stUid := stat.Uid

	if systemUser, err := user.LookupId(strconv.FormatUint(uint64(stUid), 10)); err != nil {
		return systemUser.Name, nil
	}

	return stUid, nil
}

func getHost() (string, error) {
	return os.Hostname()
}
