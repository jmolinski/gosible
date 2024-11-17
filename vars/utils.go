package vars

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/scylladb/gosible/template"
	"github.com/scylladb/gosible/utils/display"
	"github.com/scylladb/gosible/utils/types"
)

/*
Return a copy of dictionaries of variables based on configured hash behavior.
Does not modify a, b Vars.
TODO: implement fancy combine_vars and merge_hash from original ansible.
*/
func combineVars(a, b types.Vars) types.Vars {
	result := make(types.Vars)

	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}

	return result
}

func MapStringStringToVars(m map[string]string) types.Vars {
	result := make(types.Vars)
	for k, v := range m {
		result[k] = v
	}
	return result
}

// TemplateVarsTemplates takes vars templates (map of strings to interface{}) and if the value is a string,
// it tries to template it in the outer vars environment. If a value is not a template string, it is copied as-is.
func TemplateVarsTemplates(varsTemplates, env types.Vars) (types.Vars, error) {
	templar := template.New(env)
	templateOptions := template.NewOptions()

	templated := make(types.Vars)
	for key, valueInterface := range varsTemplates {
		valueString, ok := valueInterface.(string)
		if ok && templar.IsTemplate(valueString) {
			t, err := templar.Template(valueString, templateOptions)
			if err != nil {
				return nil, err
			}
			templated[key] = t
		} else {
			templated[key] = valueInterface
		}
	}

	return templated, nil
}

func TemplateActionArgs(args, vars types.Vars) (types.Vars, error) {
	display.Debug(nil, "Before templating args: %s", spew.Sdump(args))

	templatedArgs, err := TemplateVarsTemplates(args, vars)
	if err == nil {
		display.Debug(nil, "After templating args: %s", spew.Sdump(templatedArgs))
	} else {
		display.Debug(nil, "Templating args failed: %s", err)
	}

	return templatedArgs, err
}
