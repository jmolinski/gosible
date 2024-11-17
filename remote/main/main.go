package main

import (
	"github.com/scylladb/gosible/modules"
	defaultModules "github.com/scylladb/gosible/modules/default"
	"github.com/scylladb/gosible/remote"
	"github.com/scylladb/gosible/utils/osUtils"
	"os"
	"path"
)

func renameSelf() error {
	binPath, err := osUtils.GetBinaryPath()
	if err != nil {
		return err
	}
	if path.Base(binPath) == remote.ClientTmpFileName {
		newBinPath := path.Join(path.Dir(binPath), remote.ClientFileName)
		return os.Rename(binPath, newBinPath)
	}
	return nil
}

func main() {
	_ = renameSelf()

	mods := modules.NewRegistry()
	defaultModules.Register(mods)
	defaultModules.RegisterPython(mods)
	setupRpcServer(mods)
}
