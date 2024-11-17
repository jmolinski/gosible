package playbook

import (
	"fmt"
	"github.com/scylladb/gosible/modules"
	"github.com/scylladb/gosible/parsing"
	playbookTypes "github.com/scylladb/gosible/playbook/types"
	"github.com/scylladb/gosible/plugins"
	"github.com/scylladb/gosible/utils/types"
	"gopkg.in/errgo.v2/fmt/errors"
	"gopkg.in/yaml.v2"
	"reflect"
	"strings"
)

// TODO FREEFORM_ACTIONS = moduleRequireArgs alias

// This is a VERY simplified version of ModuleArgsParser from Ansible
// TODO expand to cover all ways of expressing the action.

func ResolveModuleArgs(task *playbookTypes.Task, modules *modules.ModuleRegistry) (action string, args types.Vars, delegateTo string, err error) {
	// TODO should also return a meaningful value for delegate_to
	// Takes a task as an argument and returns its action (module), arguments and delegate_to
	// values.

	additionalArgs := task.Args

	// TODO parse old-style declarations: when one of action, local_action is specifie
	// TODO - action
	// TODO - local_action

	// New-style
	// module: <stuff> invocation

	// filter out task attributes so that we're only querying unrecognized keys as actions/modules
	actionCandidates := map[string]interface{}{}
	for k, v := range task.Keywords {
		if isTaskKeyword(k) {
			continue
		}
		actionCandidates[k] = v
	}

	for k, v := range actionCandidates {
		isActionCandidate := false

		// TODO if in BUILTIN_TASKS then is a candidate
		// else try resolve plugin:
		_, pluginActionResolved := plugins.FindAction(k)
		if !pluginActionResolved {
			_, pluginActionResolved = modules.FindModule(k)
		}
		isActionCandidate = pluginActionResolved

		if isActionCandidate {
			if action != "" {
				return "", nil, "", errors.New("multiple actions specified")
			}

			// TODO save plugin context := pluginContext.resolved_fqcn

			action, args, err = normalizeParameters(k, v, additionalArgs)
			if err != nil {
				return "", nil, "", err
			}
		}

	}

	if action == "" {
		// TODO check if actionCandidates is nonempty - if so, warn that the action may be misspelled
		return "", nil, "", errors.New("no action specified")
	}

	return action, args, delegateTo, nil
}

func isTaskKeyword(key string) bool {
	if strings.HasPrefix(key, "with_") {
		return true
	}
	for _, k := range taskKeywords {
		if k == key {
			return true
		}
	}

	return false
}

func normalizeParameters(action string, value interface{}, additionalArgs map[string]interface{}) (string, types.Vars, error) {
	finalArgs := types.Vars{}

	for k, v := range additionalArgs {
		var ok bool
		finalArgs[k], ok = v.(string)
		// TODO support Templar template string substitution
		if !ok {
			return "", nil, fmt.Errorf("%s is not a string", k)
		}
	}

	args, err := normalizeNewStyleArgs(action, value)
	if err != nil {
		return "", nil, err
	}
	for k, v := range args {
		finalArgs[k] = v
	}

	return action, finalArgs, nil
}

func normalizeNewStyleArgs(action string, value interface{}) (types.Vars, error) {
	args := types.Vars{}

	// Comment form Ansible:
	// possible example inputs:
	//      'echo hi', 'shell'
	//      {'region': 'xyz'}, 'ec2'
	// standardized outputs like:
	//      { _raw_params: 'echo hi', _uses_shell: True }

	switch v := value.(type) {
	case yaml.MapSlice:
		// # form is like: { xyz: { x: 2, y: 3 } }
		// args = thing
		for _, kv := range v {
			key, ok := kv.Key.(string)
			if !ok {
				return nil, fmt.Errorf("%#v is not a string", kv.Key)
			}
			if key == "args" {
				// TODO support args
				continue
			}
			args[key] = kv.Value
		}
	case string:
		// form is like: copy: src=a dest=b
		for k, v := range parsing.ParseKeyValuePairsString(v, isFreeformAction(action)) {
			args[k] = v
		}
	case nil:
	default:
		return nil, errors.New(fmt.Sprintf("unexpected parameter type in action: %s", reflect.TypeOf(value)))
	}

	return args, nil
}

func isFreeformAction(action string) bool {
	for _, a := range moduleRequireArgs {
		if a == action {
			return true
		}
	}
	return false
}
