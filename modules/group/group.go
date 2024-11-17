package group

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/scylladb/gosible/module_utils/gosibleModule"
	"github.com/scylladb/gosible/modules"
	pathUtils "github.com/scylladb/gosible/utils/path"
	"github.com/scylladb/gosible/utils/types"
	"os"
	"os/user"
	"strconv"
	"strings"
)

type Module struct {
	*gosibleModule.GosibleModule[*Params]
}

var _ modules.Module = &Module{}

type Return struct {
	Name   string
	State  string
	Exists bool
	System bool
	Gid    int
}

func New() *Module {
	return &Module{GosibleModule: gosibleModule.New[*Params](&Params{
		State: "present",
	})}
}

func (m *Module) Name() string {
	return "group"
}

func (m *Module) Run(ctx *modules.RunContext, vars types.Vars) (r *modules.Return) {
	if err := m.ParseParams(ctx, vars); err != nil {
		return m.MarkReturnFailed(err)
	}
	m.UpdateReturn(&modules.Return{ModuleSpecificReturn: &Return{
		Name:  m.Params.Name,
		State: m.Params.State,
	}})
	g := m.newPlatformSpecificGroup()

	var result *RunCommandResult
	if m.Params.State == "absent" {
		exists, err := g.groupExists()
		if err != nil {
			return m.MarkReturnFailed(err)
		}
		if exists {
			result, err = g.groupDel()
			if err != nil {
				return m.MarkReturnFailed(err)
			}
			if result.Rc != 0 {
				return m.MarkReturnFailed(fmt.Errorf("failed removing group %s. command exited with error: %s", m.Params.Name, result.Stderr))
			}
		}
	} else {
		exists, err := g.groupExists()
		if err != nil {
			return m.MarkReturnFailed(err)
		}
		if exists {
			result, err = g.groupMod(m.Params.Gid)
		} else {
			result, err = g.groupAdd(m.Params.Gid, m.Params.System)
		}

		if err != nil {
			return m.MarkReturnFailed(err)
		}
		if result != nil && result.Rc != 0 {
			return m.MarkReturnFailed(fmt.Errorf("failed removing group %s. command exited with error: %s", m.Params.Name, result.Stderr))
		}
	}

	if result != nil {
		m.UpdateReturn(&modules.Return{Changed: true, Stderr: result.Stderr, Stdout: result.Stdout})
	}

	info, err := g.groupInfo()
	if err != nil {
		return m.MarkReturnFailed(err)
	}
	if info != nil {
		gid, err := strconv.Atoi(info.Gid)
		if err != nil {
			return m.MarkReturnFailed(err)
		}
		m.UpdateReturn(&modules.Return{ModuleSpecificReturn: &Return{
			Exists: true,
			System: m.Params.System,
			Gid:    gid,
		}})
	}

	if err = m.Close(); err != nil {
		return m.MarkReturnFailed(err)
	}
	return m.GetReturn()
}

func (p *Params) Validate() error {
	if p.State != "present" && p.State != "absent" {
		return errors.New("state can only be present or absent")
	}
	if p.Name == "" {
		return errors.New("name parameter is required")
	}
	if p.NonUnique && p.Gid == nil {
		return errors.New("when non_unique is selected gid must be provided")
	}

	return nil
}

type RunCommandResult = gosibleModule.RunCommandResult

type group interface {
	executeCommand(interface{}) (*RunCommandResult, error)
	groupDel() (*RunCommandResult, error)
	groupAdd(gid *int, system bool) (*RunCommandResult, error)
	groupMod(gid *int) (*RunCommandResult, error)
	groupExists() (bool, error)
	groupInfo() (*user.Group, error)
}

type baseGroup struct {
	module    *gosibleModule.GosibleModule[*Params]
	groupFile string
}

func newBaseGroup(module *gosibleModule.GosibleModule[*Params]) *baseGroup {
	return &baseGroup{
		module:    module,
		groupFile: "/etc/group",
	}
}

type Params struct {
	State     string `mapstructure:"state"`
	Name      string `mapstructure:"name"`
	Gid       *int   `mapstructure:"gid"`
	System    bool   `mapstructure:"system"`
	Local     bool   `mapstructure:"local"`
	NonUnique bool   `mapstructure:"non_unique"`
}

func (g *baseGroup) executeCommand(args interface{}) (*RunCommandResult, error) {
	return g.module.RunCommand(args, gosibleModule.RunCommandDefaultKwargs())
}

func (g *baseGroup) getLocalOrSystemCommandPath(systemName string, localName string) (string, error) {
	var command string
	if g.module.Params.Local {
		command = localName
		err := g.localCheckGidExists()
		if err != nil {
			return "", err
		}
	} else {
		command = systemName
	}

	return g.module.GetBinPath(command, []string{}, true)
}

func (g *baseGroup) groupDel() (res *RunCommandResult, err error) {
	path, err := g.getLocalOrSystemCommandPath("groupdel", "lgroupdel")
	if err != nil {
		return
	}
	cmd := []string{path, g.module.Params.Name}
	return g.executeCommand(cmd)
}

func (g *baseGroup) groupAdd(gid *int, system bool) (res *RunCommandResult, err error) {
	path, err := g.getLocalOrSystemCommandPath("groupadd", "lgroupadd")
	if err != nil {
		return
	}
	cmd, err := g.getCmdGroupAddModCmd(path, gid)
	if err != nil {
		return
	}
	if system {
		cmd = append(cmd, "-r")
	}
	cmd = append(cmd, g.module.Params.Name)

	return g.executeCommand(cmd)
}

func (g *baseGroup) groupMod(gid *int) (res *RunCommandResult, err error) {
	path, err := g.getLocalOrSystemCommandPath("groupmod", "lgroupmod")
	if err != nil {
		return
	}
	cmd, err := g.getCmdGroupAddModCmd(path, gid)
	if err != nil {
		return
	}
	if len(cmd) == 1 {
		return nil, nil
	}
	cmd = append(cmd, g.module.Params.Name)

	return g.executeCommand(cmd)
}

func (g *baseGroup) getCmdGroupAddModCmd(path string, gid *int) ([]string, error) {
	cmd := []string{path}
	if gid != nil {
		info, err := g.groupInfo()
		if err != nil {
			return nil, err
		}
		if info == nil || info.Gid != strconv.Itoa(*gid) {
			cmd = append(cmd, "-g", strconv.Itoa(*gid))
			if g.module.Params.NonUnique {
				cmd = append(cmd, "-o")
			}
		}
	}
	return cmd, nil
}

func (g *baseGroup) localCheckGidExists() error {
	if g.module.Params.Gid == nil {
		return nil
	}
	grp, err := user.LookupGroupId(strconv.Itoa(*g.module.Params.Gid))
	if err != nil {
		return err
	}
	if grp.Name != g.module.Params.Name {
		return fmt.Errorf("GID '%d' already exists with group '%s'", *g.module.Params.Gid, grp.Name)
	}
	return nil
}

func (g *baseGroup) groupExists() (bool, error) {
	if g.module.Params.Local {
		return g.groupExistsLocally()
	}
	return g.groupExistsInSystem()

}

func (g *baseGroup) groupInfo() (*user.Group, error) {
	exists, err := g.groupExists()
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return user.LookupGroup(g.module.Params.Name)
}

func (g *baseGroup) groupExistsInSystem() (bool, error) {
	_, err := user.LookupGroup(g.module.Params.Name)
	if err != nil {
		if _, ok := err.(user.UnknownGroupError); ok {
			return false, nil
		}
		return false, err
	}
	return true, err
}

func (g *baseGroup) groupExistsLocally() (bool, error) {
	gfExists, err := pathUtils.Exists(g.groupFile)
	if err != nil {
		return false, err
	}
	if !gfExists {
		return false, fmt.Errorf("'local: true' specified but unable to find local group file %s to parse", g.groupFile)
	}
	nameTest := g.module.Params.Name + ":"
	f, err := os.Open(g.groupFile)
	if err != nil {
		return false, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), nameTest) {
			return true, nil
		}
	}
	return false, nil
}

func (m *Module) newPlatformSpecificGroup() group {
	// Implement choosing group based on system and group for other systems.
	return newBaseGroup(m.GosibleModule)
}
