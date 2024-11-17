package setFact

import (
	"context"
	"errors"
	"github.com/scylladb/gosible/modules"
	"github.com/scylladb/gosible/plugins"
	"github.com/scylladb/gosible/plugins/action"
	"github.com/scylladb/gosible/template"
	"github.com/scylladb/gosible/utils/types"
)

const Name = "set_fact"

func New() *Action {
	return &Action{action.New(&action.DefaultParams{})}
}

type Action struct {
	*action.Base[*action.DefaultParams]
}

func (a *Action) Run(_ context.Context, actionCtx *plugins.ActionContext) *plugins.Return {
	if len(actionCtx.Args) == 0 {
		return a.Error("No key/value pairs provided, at least one is required for this action to succeed")
	}

	facts := make(types.Facts)
	factsBucket := modules.BucketAnsibleFacts
	if v, ok := actionCtx.Args["cacheable"].(bool); ok && v {
		factsBucket = modules.BucketNonPersistentAnsibleFacts
	}

	templar := template.New(actionCtx.VarsEnv)
	templateOptions := template.NewOptions()

	for k, v := range actionCtx.Args {
		templatedK, err := templar.Template(k, templateOptions)
		if err != nil {
			a.Warn(err.Error())
			return a.MarkReturnFailed(err)
		}

		if _, ok := templatedK.(string); ok {
			if !isValidAnsibleFactName(k) {
				errMsg := "The variable name '" + k + "' is not valid. Variables must start with a letter or underscore character, " +
					"and contain only letters, numbers and underscores."
				return a.Error(errMsg)
			}

			// Possibly FIXME - ansible does something weird with booleans here, and it does it because of Jinja2's handling of these.
			// I believe that it works differently in Gonja (gonja does not support Jinja2 Native mode). I'm leaving this part off.

			facts[k] = v
		} else {
			return a.Error("Invalid key")

		}
	}

	return a.UpdateReturn(&plugins.Return{InternalReturn: &plugins.InternalReturn{
		AnsibleFacts: facts,
		FactBucket:   factsBucket,
	}})
}

func (a *Action) Error(test string) *plugins.Return {
	a.Warn(test)
	return a.MarkReturnFailed(errors.New(test))
}

func isValidAnsibleFactName(_ string) bool {
	// TODO reimplement python's str.isidentifier
	return true
}
