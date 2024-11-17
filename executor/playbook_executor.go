package executor

import (
	"context"
	"fmt"
	"github.com/scylladb/gosible/executor/conn"
	"github.com/scylladb/gosible/executor/moduleExecutor"
	"github.com/scylladb/gosible/inventory"
	playbookTypes "github.com/scylladb/gosible/playbook/types"
	"github.com/scylladb/gosible/plugins"
	"github.com/scylladb/gosible/plugins/meta"
	"github.com/scylladb/gosible/template"
	"github.com/scylladb/gosible/utils/display"
	"github.com/scylladb/gosible/utils/maps"
	"github.com/scylladb/gosible/utils/parallel"
	"github.com/scylladb/gosible/utils/types"
	varsPkg "github.com/scylladb/gosible/vars"
)

// TimeoutLocalTaskExecution is the timeout for task executed on the local machine.
const TimeoutLocalTaskExecution = moduleExecutor.TimeoutRemoteTaskExecution

type playExecutor struct {
	play               *playbookTypes.Play
	inv                *inventory.Data
	varsManager        *varsPkg.Manager
	hosts              map[string]*inventory.Host
	connectionManagers map[string]*conn.Manager
}

type tasksExecutor struct {
	*playExecutor
	tasks []*playbookTypes.Task
}

type taskOnHostExecutor struct {
	*tasksExecutor
	task              *playbookTypes.Task
	host              *inventory.Host
	connectionManager *conn.Manager
	connection        *plugins.ConnectionContext
}

func (ex *taskOnHostExecutor) GetVars() (types.Vars, error) {
	if v, err := ex.varsManager.GetVars(ex.play, ex.host, ex.task); err != nil {
		return nil, fmt.Errorf("on host %s, failed to collect vars: %w", ex.host.Name, err)
	} else {
		return v, nil
	}
}

func ExecutePlaybook(pbook *playbookTypes.Playbook, inventory *inventory.Data, varsManager *varsPkg.Manager, passwords types.Passwords) error {
	// TODO support list/check only mode: listhosts, listtasks, listtags, syntax
	// TODO support loop_control
	display.Display(display.Options{}, "Executing %d plays from the specified playbook", len(pbook.Plays))

	for _, play := range pbook.Plays {
		playExecutor := &playExecutor{
			play:        play,
			inv:         inventory,
			varsManager: varsManager,
		}

		if err := playExecutor.execute(passwords); err != nil {
			return err
		}
	}
	return nil
}

func (ex *playExecutor) execute(passwords types.Passwords) error {
	if err := ex.showPlayNameBanner(); err != nil {
		return err
	}

	if err := ex.determineHosts(); err != nil {
		return err
	}

	err := ex.setupConnectionManagers(passwords)

	defer func() {
		if errClose := conn.CloseConnMgrs(ex.connectionManagers); errClose != nil {
			if err == nil {
				err = fmt.Errorf("while closing connections: %w", errClose)
			}
			err = fmt.Errorf("while closing connections: %w, %s", errClose, err)
		}
	}()

	if err != nil {
		return err
	}

	return ex.executeStrategy()
}

func (ex *playExecutor) executeStrategy() error {
	if ex.play.StrategyKey == "linear" {
		// Schedule tasks one by one for execution on each host, sync after each task.
		for _, t := range ex.play.Tasks {
			tasksExecutor := &tasksExecutor{
				playExecutor: ex,
				tasks:        []*playbookTypes.Task{t},
			}
			if err := tasksExecutor.execute(); err != nil {
				return err
			}
		}
	} else if ex.play.StrategyKey == "free" {
		// Schedule all tasks for parallel execution on each host. Sync after all hosts are done with all tasks.
		tasksExecutor := &tasksExecutor{
			playExecutor: ex,
			tasks:        ex.play.Tasks,
		}
		if err := tasksExecutor.execute(); err != nil {
			return err
		}
	} else {
		// Should never happen.
		return fmt.Errorf("unknown strategy key: %s", ex.play.StrategyKey)
	}

	return nil
}

func (ex *playExecutor) showPlayNameBanner() (err error) {
	vars, err := ex.varsManager.GetVars(ex.play, nil, nil)
	if err != nil {
		return err
	}
	if ex.play.Name != "" {
		if templatedName, err := template.TemplateToString(ex.play.Name, vars, nil); err != nil {
			return fmt.Errorf("failed to template play name: %w", err)
		} else {
			display.Display(display.Options{}, "Executing play '%s'", templatedName)
		}
	} else {
		display.Display(display.Options{}, "Executing an unnamed play (consider naming it!)")
	}
	return nil
}

func (ex *playExecutor) determineHosts() (err error) {
	ex.hosts, err = ex.inv.DetermineHosts(ex.play.HostsPattern)
	if err != nil {
		return fmt.Errorf("failed to determine hosts for play: %w", err)
	}
	return nil
}

func (ex *playExecutor) setupConnectionManagers(passwords types.Passwords) error {
	ex.connectionManagers = make(map[string]*conn.Manager)
	for hostName, host := range ex.hosts {
		opts, err := ex.varsManager.GetVars(ex.play, host, nil)
		if err != nil {
			return err
		}
		ex.connectionManagers[hostName] = conn.NewManager(host, opts, passwords)
	}

	return nil
}

func (ex *tasksExecutor) execute() error {
	// Execute meta tasks (in strategy linear, only one task is in the slice; in strategy free,
	// all meta tasks are executed beforehand all regular tasks TODO verify if this is what ansible does).
	for _, t := range ex.tasks {
		if meta.IsMetaTask(t) {
			if err := meta.Execute(ex.hosts, t, ex.play, ex.connectionManagers, ex.varsManager); err != nil {
				return fmt.Errorf("on task %s, %w", t.Name, err)
			}
		}
	}

	if errors := parallel.ForAll(maps.Values(ex.hosts), ex.executeTasksOnHost); errors.IsError() {
		return errors.Combine()
	}
	return nil
}

func (ex *tasksExecutor) executeTasksOnHost(host *inventory.Host) error {
	// Execute all tasks from the list on the given host.
	// If a task fails, cease execution on the host.
	for _, t := range ex.tasks {
		if meta.IsMetaTask(t) {
			// Skip meta tasks. TODO fix meta handling
			continue
		}
		taskInstance := &taskOnHostExecutor{
			tasksExecutor:     ex,
			host:              host,
			task:              t,
			connectionManager: ex.connectionManagers[host.Name],
		}
		if err := taskInstance.execute(); err != nil {
			return fmt.Errorf("on task %s, %w", t.Name, err)
		}
	}
	return nil
}

func (ex *taskOnHostExecutor) execute() error {
	if err := ex.showTaskNameBanner(); err != nil {
		return err
	}

	if err := ex.setupConnection(); err != nil {
		return fmt.Errorf("on host %s, %w", ex.host.Name, err)
	}

	if ex.task.HasLoop() {
		if err := ex.runLoop(); err != nil {
			return fmt.Errorf("on host %s, %w", ex.host.Name, err)
		}
		return nil
	}

	if err := ex.executeActionIfWhenSatisfied(); err != nil {
		return fmt.Errorf("on host %s, %w", ex.host.Name, err)
	}
	return nil
}

func (ex *taskOnHostExecutor) showTaskNameBanner() (err error) {
	vars, err := ex.varsManager.GetVars(ex.play, nil, ex.task)
	if err != nil {
		return err
	}
	if templatedName, err := template.TemplateToString(ex.task.Name, vars, nil); err != nil {
		return fmt.Errorf("failed to template task name: %w", err)
	} else {
		display.Display(display.Options{}, "Executing task '%s' on host '%s'", templatedName, ex.host.Name)
	}
	return nil
}

func (ex *taskOnHostExecutor) setupConnection() error {
	display.Debug(nil, "Collecting variables for task %s on host %s", ex.task.Name, ex.host.Name)
	opts, err := ex.GetVars()
	if err != nil {
		return err
	}

	ex.connectionManager.UpdateOpts(opts)
	ex.connection, err = ex.connectionManager.GetConnForTask(ex.task)
	return err
}

func (ex *taskOnHostExecutor) runLoop() error {
	ex.varsManager.ResetLoopContext(ex.host)
	defer ex.varsManager.ResetLoopContext(ex.host)

	vars, err := ex.GetVars()
	if err != nil {
		return err
	}
	loopItems, err := ex.task.GetLoopItems(vars)
	if err != nil {
		return fmt.Errorf("on host %s, failed to get loop items: %w", ex.host.Name, err)
	}

	for _, loopItem := range loopItems {
		ex.varsManager.SetLoopItem(loopItem, ex.host)
		if err = ex.executeActionIfWhenSatisfied(); err != nil {
			return err
		}
	}

	return nil
}

func (ex *taskOnHostExecutor) executeActionIfWhenSatisfied() error {
	varsEnv, err := ex.varsManager.GetVars(ex.play, ex.host, ex.task)
	if err != nil {
		return fmt.Errorf("on host %s, failed to collect vars: %w", ex.host.Name, err)
	}

	if whenSatisfied, err := ex.task.WhenConditionsSatisfied(varsEnv); err == nil {
		if whenSatisfied {
			return ex.executeAction(varsEnv)
		}
		display.Debug(&ex.host.Name, "Skipping task '%s' because when conditions are not satisfied", ex.task.Name)
		return nil
	} else {
		return fmt.Errorf("failed to check when conditions: %w", err)
	}
}

func (ex *taskOnHostExecutor) executeAction(varsEnv types.Vars) error {
	if action, ok := plugins.FindAction(ex.task.Action.Name); ok {
		// Execute plugin if one exists for this action.
		templatedArgs, err := varsPkg.TemplateActionArgs(ex.task.Action.Args, varsEnv)
		if err != nil {
			return err
		}
		ctx := plugins.CreateActionContext(ex.connection, templatedArgs, varsEnv)
		res, err := executePluginAction(action, &ctx)
		if err == nil && res.InternalReturn != nil {
			ex.varsManager.SaveFacts(res.FactBucket, res.AnsibleFacts, ex.host)
		}
	} else {
		// Otherwise, try executing the action as a module.
		res, err := moduleExecutor.ExecuteRemoteModuleTask(ex.task, ex.play, ex.connection, varsEnv)
		if err == nil && res.InternalReturn != nil {
			ex.varsManager.SaveFacts(res.FactBucket, res.AnsibleFacts, ex.host)
		}
	}
	// TODO do something meaningful with the execution result (in particular, support register)

	return nil
}

func executePluginAction(action plugins.Action, actionCtx *plugins.ActionContext) (*plugins.Return, error) {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutLocalTaskExecution)
	defer cancel()
	rsp := action.Run(ctx, actionCtx)

	display.Display(display.Options{}, "Plugin execution result msg: %s", rsp.Msg)

	return rsp, nil
}
