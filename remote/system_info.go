package remote

import (
	"bufio"
	"errors"
	"github.com/scylladb/gosible/connection"
	"github.com/scylladb/gosible/utils/display"
	"github.com/scylladb/gosible/utils/types"
	"strings"
)

type systemInfo struct {
	gosibleDir        string
	gosibleBinExists  bool
	gosibleBinPath    string
	gosibleTmpBinPath string
	uname             string
}

func gatherSystemInfo(path string, conn connection.CommandExecutor) (si systemInfo, err error) {
	sha256, err := getHashStr(path)
	if err != nil {
		return
	}
	dirPath := "$HOME/.cache/gosible_client/" + sha256
	runnerPath := "$DIR/" + ClientFileName

	cmd := "DIR=\"" + dirPath + "\"; echo \"Dir=$DIR\"; echo \"Sys=$(uname -a)\"; ([ -e \"" + runnerPath + "\" ] && echo HasRunner=1); mkdir -p \"$DIR\"; chmod 1775 \"$DIR\""
	display.Debug(nil, "Gather system information command: "+cmd)

	stdout, _, err := conn.ExecCommand(cmd, nil, false, &types.BecomeArgs{})
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(strings.NewReader(stdout.String()))
	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := line[:idx]
		value := line[idx+1:]

		switch key {
		case "Sys":
			si.uname = value
		case "Dir":
			si.gosibleDir = value
		case "HasRunner":
			si.gosibleBinExists = true
		}
	}

	if si.gosibleDir == "" {
		err = errors.New("failed to resolve remote gosible directory")
		return
	}
	si.gosibleBinPath = si.gosibleDir + "/" + ClientFileName
	si.gosibleTmpBinPath = si.gosibleDir + "/" + ClientTmpFileName
	return
}
