package inventory

import "github.com/scylladb/gosible/utils/types"

type Host struct {
	Name   string
	Vars   types.Vars
	Groups []*Group
}

type Group struct {
	Name        string
	Children    []*Group
	Hosts       []*Host
	Vars        types.Vars
	Parents     []*Group
	initialized bool
}

type Data struct {
	Hosts  map[string]*Host
	Groups map[string]*Group
}

// AllVars returns all variables that host have either from itself or its groups.
func (h *Host) AllVars() types.Vars {
	var vars = make(types.Vars)
	for _, group := range h.Groups {
		vars = mergeVars(vars, group.AllVars())
	}
	return mergeVars(vars, h.Vars)
}

// AllVars returns all variables that group have either from itself or its parents.
func (g *Group) AllVars() types.Vars {
	var vars = make(types.Vars)
	for _, group := range g.Parents {
		vars = mergeVars(vars, group.AllVars())
	}
	return mergeVars(vars, g.Vars)
}

// GetHosts returns all host that are in the group or its children.
func (g *Group) GetHosts() map[string]*Host {
	hosts := make(map[string]*Host)

	for _, host := range g.Hosts {
		hosts[host.Name] = host
	}

	for _, group := range g.Children {
		for _, host := range group.GetHosts() {
			hosts[host.Name] = host
		}
	}

	return hosts
}

func mergeVars(base types.Vars, new types.Vars) types.Vars {
	for k, v := range new {
		base[k] = v
	}
	return base
}
