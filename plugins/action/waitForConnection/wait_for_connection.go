package waitForConnection

import (
	"context"
	"errors"
	"fmt"
	"github.com/scylladb/gosible/executor/moduleExecutor"
	"github.com/scylladb/gosible/modules"
	"github.com/scylladb/gosible/modules/ping"
	playbookTypes "github.com/scylladb/gosible/playbook/types"
	"github.com/scylladb/gosible/plugins"
	"github.com/scylladb/gosible/plugins/action"
	"github.com/scylladb/gosible/utils/display"
	"time"
)

const Name = "wait_for_connection"

func New() *Action {
	return &Action{action.New(&Params{
		ConnectTimeout: 5,
		Delay:          0,
		Sleep:          1,
		Timeout:        600,
	})}
}

type Action struct {
	*action.Base[*Params]
}

func (a *Action) Run(_ context.Context, actionCtx *plugins.ActionContext) *plugins.Return {
	if err := a.ParseParams(actionCtx.Args); err != nil {
		return a.MarkReturnFailed(err)
	}
	start := time.Now()
	if a.Params.Delay != 0 {
		time.Sleep(time.Duration(a.Params.Delay) * time.Second)
	}
	end := start.Add(time.Duration(a.Params.Timeout) * time.Second)
	succ := false
	var err error
	var modRet *modules.Return
	for time.Now().Before(end) {
		modRet, err = pingModuleTest(actionCtx)
		if err == nil {
			if m, ok := modRet.ModuleSpecificReturn.(map[string]interface{}); ok {
				if m["Ping"] != ping.DefaultData {
					err = errors.New("ping test failed")
				} else {
					display.Debug(nil, "wait_for_connection: ping module test success")
					succ = true
					break
				}
			} else {
				err = errors.New("expected module specific return to be map")
			}
		}
		display.Debug(nil, fmt.Sprintf("wait_for_connection: ping module test fail (expected), retrying in %d seconds...", a.Params.Sleep))
		time.Sleep(time.Duration(a.Params.Sleep) * time.Second)
	}
	if !succ {
		return a.MarkReturnFailed(fmt.Errorf("timed out waiting for ping module test: %v", err))
	}

	elapsed := time.Now().Sub(start)
	return a.UpdateReturn(&plugins.Return{
		ModuleSpecificReturn: &Return{
			Elapsed: int(elapsed.Seconds()),
		},
	})
}

type Return struct {
	Elapsed int
}

type Params struct {
	ConnectTimeout int `mapstructure:"connect_timeout"`
	Delay          int `mapstructure:"delay"`
	Sleep          int `mapstructure:"sleep"`
	Timeout        int `mapstructure:"timeout"`
}

func (p Params) Validate() error {
	return nil
}

func pingModuleTest(actionCtx *plugins.ActionContext) (*modules.Return, error) {
	return moduleExecutor.ExecuteRemoteModuleTask(&playbookTypes.Task{
		Action: &playbookTypes.Action{
			Name: "ping",
		},
	}, nil, actionCtx.Connection, nil)
}
