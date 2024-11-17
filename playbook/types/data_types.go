package types

import (
	"encoding/json"
	"fmt"
	"github.com/jmolinski/gosible-templates/exec"
	"github.com/scylladb/gosible/plugins/lookup"
	"github.com/scylladb/gosible/template"
	"github.com/scylladb/gosible/utils/types"
	"strings"
)

type Playbook struct {
	Plays []*Play
}

type Play struct {
	Name          string
	HostsPattern  string
	VarsTemplates types.Vars
	Tasks         []*Task
	StrategyKey   string
	// TODO add roles support
	// TODO add blocks support
	// TODO add notify support
}

type Action struct {
	Name       string
	Args       types.Vars
	DelegateTo string
}

type Loop struct {
	Template string
	Items    []interface{}
}

type With struct {
	LookupPluginName string
	*Loop
}

type Task struct {
	Name           string
	Args           types.Vars
	VarsTemplates  types.Vars
	Keywords       map[string]interface{}
	Action         *Action
	Loop           *Loop
	With           *With
	WhenConditions []string
	// TODO add parent
}

func (p *Playbook) String() string {
	pBytes, _ := json.MarshalIndent(p, "", "    ")
	return string(pBytes)
}

func (p *Play) String() string {
	pBytes, _ := json.MarshalIndent(p, "", "    ")
	return string(pBytes)
}

func (t *Task) String() string {
	tBytes, _ := json.MarshalIndent(t, "", "    ")
	return string(tBytes)
}

func (l *Loop) IsTemplate() bool {
	return l.Items == nil
}

func (t *Task) HasLoop() bool {
	return t.Loop != nil || t.With != nil
}

func templateLoopItems(templar *template.Templar, templateOptions *template.Options, loopItems []interface{}) ([]interface{}, error) {
	items := make([]interface{}, 0, len(loopItems))

	for _, item := range loopItems {
		if v, ok := item.(string); ok {
			if templar.IsTemplate(item.(string)) {
				t, err := templar.Template(v, templateOptions)
				if err != nil {
					return nil, fmt.Errorf("error templating loop item: %s", err)
				}
				items = append(items, t)
			} else {
				items = append(items, item)
			}
		} else {
			items = append(items, item)
		}
	}

	return items, nil
}

func (t *Task) GetLoopItems(varsEnv types.Vars) ([]interface{}, error) {
	templar := template.New(varsEnv)
	templateOptions := template.NewOptions()

	if t.With != nil {
		return t.getWithLookupLoopItems(varsEnv, templar, templateOptions)
	}

	if t.Loop.IsTemplate() {
		templated, err := templar.Template(t.Loop.Template, templateOptions)
		if err != nil {
			return nil, err
		}

		if v, ok := templated.([]interface{}); ok {
			return v, nil
		} else {
			return nil, fmt.Errorf("loop template must return a list")
		}
	} else {
		return templateLoopItems(templar, templateOptions, t.Loop.Items)
	}
}

func (t *Task) getWithLookupLoopItems(varsEnv types.Vars, templar *template.Templar, templateOptions *template.Options) ([]interface{}, error) {
	va := exec.NewVarArgs(varsEnv)
	va.Args = append(va.Args, exec.AsValue(t.With.LookupPluginName))

	if t.With.IsTemplate() {
		templated, err := templar.Template(t.With.Template, templateOptions)
		if err != nil {
			return nil, err
		}

		va.Args = append(va.Args, exec.AsValue(templated))
	} else {
		templatedArgsForPlugin, err := templateLoopItems(templar, templateOptions, t.With.Items)
		if err != nil {
			return nil, err
		}
		for _, templatedArgForPlugin := range templatedArgsForPlugin {
			va.Args = append(va.Args, exec.AsValue(templatedArgForPlugin))
		}
	}

	lookupResult := lookup.Query(va)
	if lookupResult.IsError() {
		return nil, fmt.Errorf("error running lookup plugin: %s", lookupResult.Error())
	}

	items := make([]interface{}, 0, lookupResult.Len())
	for i := 0; i < lookupResult.Len(); i++ {
		items = append(items, lookupResult.Index(i))
	}

	return items, nil
}

func (t *Task) WhenConditionsSatisfied(varsEnv types.Vars) (bool, error) {
	templar := template.New(varsEnv)
	templateOptions := template.NewOptions()

	// All conditions are ANDed together.
	for _, condition := range t.WhenConditions {
		if !strings.HasPrefix(condition, "{{") {
			condition = "{{ " + condition + " }}"
		}

		if templar.IsTemplate(condition) {
			templated, err := templar.Template(condition, templateOptions)
			if err != nil {
				return false, fmt.Errorf("error templating when condition: %s", err)
			}

			switch cond := templated.(type) {
			case bool:
				if !cond {
					return false, nil
				}
			case string:
				// The results are python-like booleans: True and False (capitalized, string).
				if cond == "False" {
					return false, nil
				}
				if cond != "True" {
					return false, fmt.Errorf("unexpected condition result: %s", cond)
				}
			default:
				return false, fmt.Errorf("unexpected condition result: %v", cond)
			}
		}
	}

	return true, nil
}

// TODO implement Task's get_vars()
