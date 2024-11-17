package lookup

import (
	"fmt"
	"github.com/jmolinski/gosible-templates/exec"
	"github.com/pkg/errors"
	"github.com/scylladb/gosible/utils/display"
	"strings"
)

func Query(va *exec.VarArgs) *exec.Value {
	va.KwArgs["wantlist"] = exec.AsValue(true)
	return Lookup(va)
}

func Lookup(va *exec.VarArgs) *exec.Value {
	pluginName, err := extractName(va)
	if err != nil {
		return exec.AsValue(err)
	}

	plugin := FindLookupPlugin(pluginName)
	if plugin == nil {
		return exec.AsValue(fmt.Errorf("lookup plugin %s not found", pluginName))
	}

	pluginExecutionResult := plugin(va)

	if pluginExecutionResult.IsError() {
		display.Error(display.ErrorOptions{}, "lookup plugin %s returned an error %v", pluginName, pluginExecutionResult.Error())
		return pluginExecutionResult
	}

	if !pluginExecutionResult.IsList() {
		return exec.AsValue(fmt.Errorf("lookup plugin %s returned a non-list value", pluginName))
	}

	outputAsList, err := wantsList(va)
	if err != nil {
		return exec.AsValue(err)
	}

	if outputAsList {
		return pluginExecutionResult
	}

	items := make([]string, 0, pluginExecutionResult.Len())
	for i := 0; i < pluginExecutionResult.Len(); i++ {
		items = append(items, pluginExecutionResult.Index(i).String())
	}
	return exec.AsValue(strings.Join(items, ","))
}

func wantsList(va *exec.VarArgs) (bool, error) {
	wantlistArg := va.KwArgs["wantlist"]
	if wantlistArg != nil && !wantlistArg.IsBool() {
		return false, errors.New("wantlist argument must be a boolean")
	}

	return wantlistArg != nil && wantlistArg.Bool(), nil
}

func extractName(va *exec.VarArgs) (string, error) {
	if len(va.Args) == 0 {
		return "", errors.New("wrong signature for 'lookup'")
	}
	name := va.Args[0].String()
	va.Args = va.Args[1:]
	return name, nil
}
