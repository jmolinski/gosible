package meta

import (
	"fmt"
	"github.com/scylladb/gosible/executor/conn"
	"github.com/scylladb/gosible/inventory"
	playbookTypes "github.com/scylladb/gosible/playbook/types"
	"github.com/scylladb/gosible/utils/fqcn"
	"github.com/scylladb/gosible/utils/maps"
	"github.com/scylladb/gosible/utils/parallel"
	varsPkg "github.com/scylladb/gosible/vars"
)

const name = "meta"

type task func(hosts map[string]*inventory.Host, task *playbookTypes.Task, play *playbookTypes.Play, connMgrs map[string]*conn.Manager, varsManager *varsPkg.Manager) error

var tasks = map[string]task{
	"clear_facts":       clearFacts,
	"clear_host_errors": clearHostErrors,
	"end_host":          endHost,
	"end_play":          endPlay,
	"flush_handlers":    flushHandlers,
	"noop":              noop,
	"refresh_inventory": refreshInventory,
	"reset_connection":  resetConnection,
	"end_batch":         endBatch,
}

func endBatch(hosts map[string]*inventory.Host, t *playbookTypes.Task, play *playbookTypes.Play, mgrs map[string]*conn.Manager, manager *varsPkg.Manager) error {
	// TODO Gosible doesn't support serial or batches.
	return nil
}

func resetConnection(_ map[string]*inventory.Host, _ *playbookTypes.Task, _ *playbookTypes.Play, mgrs map[string]*conn.Manager, _ *varsPkg.Manager) error {
	for _, c := range mgrs {
		if err := c.Close(); err != nil {
			return err
		}
	}
	return nil
}

func refreshInventory(hosts map[string]*inventory.Host, t *playbookTypes.Task, play *playbookTypes.Play, mgrs map[string]*conn.Manager, manager *varsPkg.Manager) error {
	// TODO implement when ansible implements dynamic inventories
	return nil
}

func noop(_ map[string]*inventory.Host, _ *playbookTypes.Task, _ *playbookTypes.Play, _ map[string]*conn.Manager, _ *varsPkg.Manager) error {
	return nil
}

func flushHandlers(hosts map[string]*inventory.Host, t *playbookTypes.Task, play *playbookTypes.Play, mgrs map[string]*conn.Manager, manager *varsPkg.Manager) error {
	// Gosible doesn't have handlers as of now
	return nil
}

func removeHosts(hosts map[string]*inventory.Host, hostsToRemove []*inventory.Host, mgrs map[string]*conn.Manager) error {
	f := func(host *inventory.Host) error { return mgrs[host.Name].Close() }

	if errors := parallel.ForAll(hostsToRemove, f); errors.IsError() {
		return fmt.Errorf("while removing hosts, %w", errors.Combine())
	}

	for _, h := range hostsToRemove {
		delete(hosts, h.Name)
		delete(mgrs, h.Name)
	}
	return nil
}

func endPlay(hosts map[string]*inventory.Host, _ *playbookTypes.Task, _ *playbookTypes.Play, mgrs map[string]*conn.Manager, _ *varsPkg.Manager) error {
	return removeHosts(hosts, maps.Values(hosts), mgrs)
}

func endHost(hosts map[string]*inventory.Host, task *playbookTypes.Task, play *playbookTypes.Play, mgrs map[string]*conn.Manager, manager *varsPkg.Manager) error {
	var hostsToRemove []*inventory.Host

	for _, host := range hosts {
		varsEnv, err := manager.GetVars(play, host, task)
		if err != nil {
			return err
		}
		if whenSatisfied, err := task.WhenConditionsSatisfied(varsEnv); err == nil {
			if whenSatisfied {
				hostsToRemove = append(hostsToRemove, host)
			}
		} else {
			return fmt.Errorf("on host %s, failed to check when conditions: %w", host.Name, err)
		}
	}

	return removeHosts(hosts, hostsToRemove, mgrs)
}

func clearHostErrors(_ map[string]*inventory.Host, _ *playbookTypes.Task, _ *playbookTypes.Play, _ map[string]*conn.Manager, _ *varsPkg.Manager) error {
	// Gosible doesn't support host errors.
	return nil
}

func clearFacts(hosts map[string]*inventory.Host, _ *playbookTypes.Task, _ *playbookTypes.Play, _ map[string]*conn.Manager, manager *varsPkg.Manager) error {
	for _, h := range hosts {
		manager.DeleteHostFacts(h)
	}
	return nil
}

var metaNames = fqcn.ToInternalFcqns(name)

func IsMetaTask(task *playbookTypes.Task) bool {
	for _, n := range metaNames {
		if task.Action.Name == n {
			return true
		}
	}
	return false
}

func Execute(hosts map[string]*inventory.Host, task *playbookTypes.Task, play *playbookTypes.Play, connMgrs map[string]*conn.Manager, varsManager *varsPkg.Manager) error {
	taskName := task.Action.Name
	if a, ok := tasks[taskName]; ok {
		return a(hosts, task, play, connMgrs, varsManager)
	}
	return fmt.Errorf("unknown meta task: %s", taskName)
}
