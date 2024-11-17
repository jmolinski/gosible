package vars

import (
	"fmt"
	"github.com/scylladb/gosible/constants"
	"github.com/scylladb/gosible/modules"
	playbookTypes "github.com/scylladb/gosible/playbook/types"
	"github.com/scylladb/gosible/utils/types"
	"sort"
	"sync"

	"github.com/scylladb/gosible/config"
	"github.com/scylladb/gosible/inventory"
	"github.com/scylladb/gosible/parsing"
	"github.com/scylladb/gosible/utils/display"
)

type Manager struct {
	extraVars              types.Vars
	inventory              *inventory.Data
	hostVars               map[*inventory.Host]types.Vars
	hostFacts              map[*inventory.Host]types.Vars
	hostNonPersistentFacts map[*inventory.Host]types.Vars
	hostLoopVars           map[*inventory.Host]types.Vars
	lock                   sync.RWMutex
}

var once sync.Once
var managerSingleton *Manager = nil

func MakeManager(inv *inventory.Data) *Manager {
	once.Do(func() {
		managerSingleton = &Manager{
			inventory:    inv,
			extraVars:    make(types.Vars),
			hostLoopVars: make(map[*inventory.Host]types.Vars),
		}
	})

	return managerSingleton
}

// SetExtraVars accepts list of string which can take following forms:
// - "foo=bar a=b" - list of key value pairs,
// - "{"foo": "bar", "a": "b"}" - yaml or json format,
// - "@./extra.yml" - path to yaml or json file with '@' prepended.
func (m *Manager) SetExtraVars(allExtraVarsRaw []string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, ev := range allExtraVarsRaw {
		if ev == "" {
			continue
		}
		if ev[0] == '@' {
			// TODO support file loading and parsing
			display.Warning(display.WarnOptions{}, "extra vars from files not supported yet")
		} else if ev[0] == '.' || ev[0] == '/' {
			return fmt.Errorf("please prepend extra_vars filename '%s' with '@'", ev)
		} else if ev[0] == '[' || ev[0] == '{' {
			// TODO parse yaml
			display.Warning(display.WarnOptions{}, "extra vars as yaml/json not supported yet")
		} else {
			extraVars := parsing.ParseKeyValuePairsString(ev, false)
			m.extraVars = combineVars(m.extraVars, MapStringStringToVars(extraVars))
		}
	}

	return nil
}

func (m *Manager) DeleteHostFacts(host *inventory.Host) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.hostFacts, host)
}

func (m *Manager) SetHostFacts(host *inventory.Host, facts types.Vars) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.hostFacts[host] = combineVars(m.hostFacts[host], facts)
}

func (m *Manager) SetHostNonPersistentFacts(host *inventory.Host, facts types.Vars) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.hostNonPersistentFacts[host] = combineVars(m.hostNonPersistentFacts[host], facts)
}

func (m *Manager) SetHostVars(host *inventory.Host, facts types.Vars) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.hostVars[host] = combineVars(m.hostVars[host], facts)
}

type varsCombiner = func(new types.Vars, source string)

// GetVars Returns the variables, with optional "context" given via the parameters
// for the play, host, and task (which could possibly result in different
// sets of variables being returned due to the additional context).
func (m *Manager) GetVars(play *playbookTypes.Play, host *inventory.Host, task *playbookTypes.Task) (types.Vars, error) {
	// TODO: handle include_hostvars, include_delegate_to, use_cache params from original implementation
	m.lock.RLock()
	defer m.lock.RUnlock()

	debug := config.Manager().Settings.DEFAULT_DEBUG

	var hostname *string
	if host != nil {
		hostname = &host.Name
	}

	display.Debug(hostname, "in variable.Manager GetVars()")

	allVars := make(types.Vars)
	varsSources := make(map[string]string)

	combine := func(new types.Vars, source string) {
		if debug {
			// Populate var sources map
			for k := range new {
				varsSources[k] = source
			}
		}
		allVars = combineVars(allVars, new)
	}

	// The order of function calls is determined by ansible variable precedence.
	// https://docs.ansible.com/ansible/latest/user_guide/playbooks_variables.html#understanding-variable-precedence
	// https://docs.ansible.com/ansible/latest/reference_appendices/general_precedence.html#configuration-settings
	// TODO add config settings
	// TODO add command-line options
	roleDefaults(play, combine)
	setBasedirs(task)
	m.groupVars(host, combine)
	hostVars(host, combine)
	hostFactVars(m, host, combine)
	// extraVars may get overwritten by other vars sources, we need to combine them once again at the end.
	// We combine them here to provide taskVars with an environment,
	// so that its vars can be rendered (if they are templates).
	// TODO keep track of the environment in which the vars from subsequent sources should be rendered.
	extraVars(m.extraVars, combine)
	if err := playVars(play, allVars, combine); err != nil {
		return nil, err
	}
	extraVars(m.extraVars, combine)
	if err := taskVars(task, allVars, combine); err != nil {
		return nil, err
	}
	includeVars(m, host, combine)
	roleVars(task, combine)
	extraVars(m.extraVars, combine)
	combine(SetMagicVars(allVars), "magic vars") // TODO: handle corner cases with magic variables (e.g. 'hostvars')
	loopVars(host, m.hostLoopVars, combine)

	return allVars, nil
}

// 2. role defaults
// first we compile any vars specified in defaults/main.yml
// for all roles within the specified play
func roleDefaults(play *playbookTypes.Play, combine varsCombiner) {
	if play == nil {
		return
	}
	// TODO: handle it when the roles are implemented
}

// https://github.com/ansible/ansible/blob/devel/lib/ansible/vars/manager.py#L205-L219
func setBasedirs(task *playbookTypes.Task) {
	if task == nil {
		return
	}
	// TODO: handle it when the variables from files are implemented
}

// Combines values from host group, order can be configured with VARIABLE_PRECEDENCE
// 4. inventory group_vars/all
// 5. playbook group_vars/all
// 6. inventory group_vars/*
// 7. playbook group_vars/*
func (m *Manager) groupVars(host *inventory.Host, combine varsCombiner) {
	if host == nil {
		return
	}

	var allGroup *inventory.Group
	for _, group := range m.inventory.Groups {
		if group.Name == "all" {
			allGroup = group
		}
	}

	// TODO group should be sorted here (sort_groups function), but we dont support all group parameters needed atm.
	hostGroups := host.Groups
	sort.Slice(hostGroups, func(i, j int) bool {
		return hostGroups[i].Name < hostGroups[j].Name
	})

	variables := make(map[string]func() types.Vars)

	variables["all_inventory"] = func() types.Vars {
		return allGroup.Vars
	}

	// TODO implement these functions when we will support functionalities needed.
	variables["all_plugins_inventory"] = func() types.Vars {
		return nil
	}
	variables["all_plugins_play"] = func() types.Vars {
		return nil
	}
	variables["groups_inventory"] = func() types.Vars {
		vars := make(types.Vars)
		// TODO: actually its logic of get_group_vars function which in original ansible is located in inventory package, might be smart to refactor this later

		// TODO: should also be sorted by sort_groups
		for _, group := range host.Groups {
			vars = combineVars(vars, group.Vars)
		}
		return vars
	}
	variables["groups_plugins_inventory"] = func() types.Vars {
		return nil
	}
	variables["groups_plugins_play"] = func() types.Vars {
		return nil
	}

	variables["plugins_by_groups"] = func() types.Vars {
		// Merges all plugin sources by group,
		// This should be used instead, NOT in combination with the other groups_plugins* functions
		return nil
	}

	for _, entry := range config.Manager().Settings.VARIABLE_PRECEDENCE {
		// Merge group as per precedence config
		if vars, ok := variables[entry]; ok {
			combine(vars(), fmt.Sprintf("group vars, precedence entry %s", entry))
		} else {
			display.Warning(display.WarnOptions{}, "Ignoring unknown variable precedence entry: %s", entry)
		}
	}

}

// 8. inventory file or script host vars
// 9. inventory host_vars
// 10. playbook host_vars
func hostVars(host *inventory.Host, combine varsCombiner) {
	if host == nil {
		return
	}
	combine(host.Vars, fmt.Sprintf("host vars for '%s'", host.Name))
	// TODO add host vars from inventory plugins and playbook host vars
}

// 11. host facts / cached set_facts
func hostFactVars(m *Manager, host *inventory.Host, combine varsCombiner) {
	if host == nil {
		return
	}
	if facts, ok := m.hostFacts[host]; ok {
		cfg := config.Manager().Settings
		if cfg.INJECT_FACTS_AS_VARS {
			combine(facts, "facts")
		} else {
			var ansibleLocal interface{}
			if ansibleLocal, ok = facts["ansible_local"]; !ok {
				ansibleLocal = types.Vars{}
			}
			combine(types.Vars{
				"ansible_local": ansibleLocal,
			}, "facts")
		}
	}
}

// 12. play vars
// 13. play vars_prompt
// 14. play vars_files
func playVars(play *playbookTypes.Play, allVars types.Vars, combine varsCombiner) error {
	if play == nil {
		return nil
	}

	renderedPlayVars, err := TemplateVarsTemplates(play.VarsTemplates, allVars)
	if err != nil {
		display.Warning(display.WarnOptions{}, "Failed to render task vars: %s", err)
		return err
	}

	combine(renderedPlayVars, fmt.Sprintf("play vars %s", play.Name))
	return nil
}

// 15. role vars (defined in role/vars/main.yml)
// 16. block vars (only for tasks in block)
// 17. task vars (only for the task)
func taskVars(task *playbookTypes.Task, allVars types.Vars, combine varsCombiner) error {
	if task == nil {
		return nil
	}
	// TODO: add role vars after roles are implemented
	// TODO: add block vars after block are implemented

	renderedTaskVars, err := TemplateVarsTemplates(task.VarsTemplates, allVars)
	if err != nil {
		display.Warning(display.WarnOptions{}, "Failed to render task vars: %s", err)
		return err
	}

	combine(renderedTaskVars, fmt.Sprintf("task vars %s", task.Name))
	return nil
}

// 18. include_vars
// 19. set_facts / registered vars
func includeVars(m *Manager, host *inventory.Host, combine varsCombiner) {
	if host == nil {
		return
	}
	if facts, ok := m.hostVars[host]; ok {
		combine(facts, "include_vars")
	}
	if facts, ok := m.hostNonPersistentFacts[host]; ok {
		combine(facts, "set_fact")
	}
}

// 20. role (and include_role) params
// 21. include params
func roleVars(task *playbookTypes.Task, combine varsCombiner) {
	if task == nil {
		return
	}
	// TODO: implement later when roles are added
}

// 22. extra vars
func extraVars(extra types.Vars, combine varsCombiner) {
	combine(extra, "extra vars")
}

// 23. loop vars (in Ansible they are injected after resolving other vars)
func loopVars(host *inventory.Host, hostLoopVars map[*inventory.Host]types.Vars, combine varsCombiner) {
	if host == nil {
		return
	}
	combine(hostLoopVars[host], "loop vars")
}

// SetMagicVars maps some names of the variables to canonical form.
// variables: variables from inventory, play, task, etc.
func SetMagicVars(src types.Vars) types.Vars {
	res := make(types.Vars)
	for attr, variableNames := range constants.MagicVariableMapping {
		for _, varName := range variableNames {
			if val, ok := src[varName]; ok {
				res[attr] = val
			}
		}
	}
	return res
}

func (m *Manager) SaveFacts(factBucket string, facts types.Facts, host *inventory.Host) {
	if facts == nil {
		return
	}
	if factBucket == "" || factBucket == modules.BucketAnsibleFacts {
		m.SetHostFacts(host, facts)
	} else if factBucket == modules.BucketNonPersistentAnsibleFacts {
		m.SetHostNonPersistentFacts(host, facts)
	} else if factBucket == modules.BucketHostVars {
		m.SetHostVars(host, facts)
	}
}

func (m *Manager) ResetLoopContext(host *inventory.Host) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.hostLoopVars[host] = make(types.Vars)
}

func (m *Manager) SetLoopItem(loopItem interface{}, host *inventory.Host) {
	m.lock.Lock()
	defer m.lock.Unlock()
	loopVar := "item" // TODO should be configurable with loop_control
	m.hostLoopVars[host][loopVar] = loopItem
}
