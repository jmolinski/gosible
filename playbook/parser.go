package playbook

import (
	"fmt"
	"github.com/scylladb/gosible/config"
	"github.com/scylladb/gosible/modules"
	playbookTypes "github.com/scylladb/gosible/playbook/types"
	"github.com/scylladb/gosible/utils/types"
	"gopkg.in/yaml.v2"
	"os"
	"strings"
)

// TODO move common elements from playbook and inventory parsers to parsing package

func Parse(filename string, modules *modules.ModuleRegistry) (data *playbookTypes.Playbook, err error) {
	// We could use separate parsers for JSON and YAML, but since YAML is a superset
	// of JSON, we can just use the same parser for both - it's a bit slower
	// for parsing JSONs, but lowers the complexity.
	parser := parser{
		modules: modules,
	}
	return parser.parseYAML(filename)
}

type parser struct {
	modules *modules.ModuleRegistry
}

type rawPlay struct {
	Name     string
	Hosts    string
	Strategy string
	Tasks    []yaml.MapSlice
	Vars     yaml.MapSlice
}

func (p *parser) parseYAML(filename string) (*playbookTypes.Playbook, error) {
	dat, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var plays []*rawPlay
	err = yaml.Unmarshal(dat, &plays)
	if err != nil {
		return nil, err
	}

	return p.parseRawPlays(plays)
}

func (p *parser) parseRawPlays(rawPlays []*rawPlay) (*playbookTypes.Playbook, error) {
	var playbook playbookTypes.Playbook
	for _, rawPlay := range rawPlays {
		tasks, err := p.parseRawTasks(rawPlay.Tasks)
		varsTemplates, ok := parseRawArgs(rawPlay.Vars) // Actually vars are in the same format as args.
		if !ok {
			return nil, fmt.Errorf("play vars is not a list of key-value pairs")
		}
		if err != nil {
			return nil, err
		}

		rawPlay.Strategy = strings.TrimSpace(rawPlay.Strategy)
		if rawPlay.Strategy == "" {
			rawPlay.Strategy = config.Manager().Settings.DEFAULT_STRATEGY
		}
		if rawPlay.Strategy != "linear" && rawPlay.Strategy != "free" {
			return nil, fmt.Errorf("strategy must be 'linear' or 'free'")
		}
		playbook.Plays = append(playbook.Plays, &playbookTypes.Play{
			Name:          strings.TrimSpace(rawPlay.Name),
			HostsPattern:  rawPlay.Hosts,
			Tasks:         tasks,
			VarsTemplates: varsTemplates,
			StrategyKey:   rawPlay.Strategy,
		})
	}

	return &playbook, nil
}

func (p *parser) parseRawTasks(rawTasks []yaml.MapSlice) ([]*playbookTypes.Task, error) {
	var tasks []*playbookTypes.Task
	for _, rawTask := range rawTasks {
		task, err := p.parseRawTask(rawTask)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (p *parser) parseRawTask(rawTask yaml.MapSlice) (*playbookTypes.Task, error) {
	var task playbookTypes.Task

	for _, rawTaskItem := range rawTask {
		keyword, ok := rawTaskItem.Key.(string)
		if !ok {
			return nil, fmt.Errorf("keyword is not a string: %s", rawTaskItem.Key)
		}
		err := parseTaskKeyword(&task, keyword, rawTaskItem.Value)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse task keyword value: %s", err)
		}
	}

	action, args, delegateTo, err := ResolveModuleArgs(&task, p.modules)
	if err != nil {
		return nil, err
	}

	task.Action = &playbookTypes.Action{
		Name:       action,
		Args:       args,
		DelegateTo: delegateTo,
	}

	return &task, nil
}

func parseTaskKeyword(task *playbookTypes.Task, originalKeyword string, value interface{}) error {
	var ok bool

	keyword := originalKeyword
	if strings.HasPrefix(keyword, "with_") {
		keyword = "with_"
	}

	switch keyword {
	case "name":
		task.Name, ok = value.(string)
		if !ok {
			return fmt.Errorf("task name is not a string")
		}
	case "args":
		task.Args, ok = parseRawArgs(value)
		if !ok {
			return fmt.Errorf("task args is not a list of key-value pairs")
		}
	case "vars":
		task.VarsTemplates, ok = parseRawArgs(value) // Actually vars are in the same format as args.
		if !ok {
			return fmt.Errorf("task vars is not a list of key-value pairs")
		}
	case "loop":
		task.Loop = &playbookTypes.Loop{}

		switch v := value.(type) {
		case string:
			task.Loop.Template = strings.TrimSpace(v)
		case []interface{}:
			task.Loop.Items = v
		default:
			return fmt.Errorf("loop is not template or list of values")
		}
	case "when":
		switch v := value.(type) {
		case string:
			task.WhenConditions = append(task.WhenConditions, strings.TrimSpace(v))
		case []interface{}:
			for _, item := range v {
				if condition, ok := item.(string); ok {
					task.WhenConditions = append(task.WhenConditions, strings.TrimSpace(condition))
				} else {
					// TODO does it actually have to be a string?
					// With our current template engine I guess it has to.
					return fmt.Errorf("when condition is not a string")
				}
			}
		default:
			return fmt.Errorf("loop is not template or list of strings")
		}
	case "with_":
		task.With = &playbookTypes.With{
			LookupPluginName: strings.TrimPrefix(originalKeyword, "with_"),
			Loop:             &playbookTypes.Loop{},
		}
		switch v := value.(type) {
		case string:
			task.With.Template = strings.TrimSpace(v)
		case []interface{}:
			task.With.Items = v
		default:
			return fmt.Errorf("loop is not template or list of values")
		}

	// TODO
	// Below is a template for parsing (some of the) other task keywords.
	// For now, only the most basic keyword are supported.
	// Some keywords are common among tasks, blocks, roles, and plays.
	// The code for parsing ang validating the keywords should thus be shared for all these ansible
	// objects.

	/*
		case "hosts":
			task.Hosts = parseHostsList(value.(string))
		case "become":
			task.Become = value.(bool)
		case "become_method":
			task.BecomeMethod = value.(string)
		case "become_user":
			task.BecomeUser = value.(string)
		case "become_flags":
			task.BecomeFlags = value.(string)
		case "async":
			task.Async = value.(bool)
		case "local_action":
			task.LocalAction = value.(string)
		case "tags":
			task.Tags = value.(string)
		case "delegate_to":
			task.DelegateTo = value.(string)
		case "local_action":
			task.LocalAction = rawTaskItem
	*/
	default:
		if task.Keywords == nil {
			task.Keywords = map[string]interface{}{}
		}
		task.Keywords[keyword] = value
	}

	return nil
}

func parseRawArgs(value interface{}) (types.Vars, bool) {
	rawArgs, ok := value.(yaml.MapSlice)
	if !ok {
		return nil, false
	}

	args := types.Vars{}
	for _, rawArg := range rawArgs {
		key, ok := rawArg.Key.(string)
		if !ok {
			return nil, false
		}
		args[key] = rawArg.Value
	}

	return args, true
}
