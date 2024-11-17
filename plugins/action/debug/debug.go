package debug

import (
	"context"
	"fmt"
	"github.com/scylladb/gosible/plugins"
	"github.com/scylladb/gosible/template"
	"github.com/scylladb/gosible/utils/display"
)

const Name = "debug"

func New() *Action {
	return &Action{}
}

type Action struct{}

func (a *Action) Run(_ context.Context, actionCtx *plugins.ActionContext) *plugins.Return {
	if actionCtx.Args["msg"] != nil && actionCtx.Args["var"] != nil {
		errMsg := "'msg' and 'var' are incompatible options"
		return &plugins.Return{Failed: true, Msg: errMsg}
	}

	result := plugins.Return{Failed: false}

	verbosity := actionCtx.Args.GetOrDefault("verbosity", 0).(int)
	if verbosity > display.Instance().GetVerbosity() {
		result.Skipped = true
		return &result
	}

	if actionCtx.Args["msg"] != nil {
		result.Msg = actionCtx.Args["msg"].(string)
	} else if actionCtx.Args["var"] != nil {
		varName := actionCtx.Args["var"].(string) // TODO Ansible supports list and dict too.
		templar := template.New(actionCtx.VarsEnv)
		templateOptions := template.NewOptions().SetConvertBare(true).SetFailOnUndefined(true)

		results, err := templar.Template(varName, templateOptions)
		if err == nil && results == varName {
			results, err = templar.Template("{{"+varName+"}}", templateOptions)
		}
		if err != nil {
			result.Msg = fmt.Sprintf("VARIABLE %s IS NOT DEFINED OR CAN'T BE TEMPLATED!: %v", varName, err)
			return &result
		} else {
			result.Msg = fmt.Sprintf("%v", results)
		}
	} else {
		result.Msg = "Hello world!"
	}

	return &result
}
