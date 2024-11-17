package inventory

import (
	"github.com/scylladb/gosible/utils/types"
	"gopkg.in/errgo.v2/fmt/errors"
	"strings"
)

func newData() *Data {
	return &Data{
		Groups: make(map[string]*Group),
		Hosts:  make(map[string]*Host),
	}
}

func newHost(name string) *Host {
	return &Host{
		Name: name,
		Vars: make(types.Vars),
	}
}

func newGroup(name string) *Group {
	return &Group{
		Name: name,
		Vars: make(types.Vars),
	}
}

func (d *Data) formatAndValidate() error {
	d.addUngroupedGroup()
	d.addToAllGroup()
	d.simplify()
	d.removeAllGroupFromHosts()
	return d.validate()
}

func (d *Data) simplify() {
	for _, host := range d.Hosts {
		host.deduplicateGroups()
	}
	for _, group := range d.Groups {
		group.deduplicateParents()
	}
}

func (d *Data) addToAllGroup() {
	allGroup := d.groupByName("all")
	for name, group := range d.Groups {
		if len(group.Parents) == 0 && name != "all" {
			allGroup.Children = append(allGroup.Children, group)
			group.Parents = append(group.Parents, allGroup)
		}
	}
}

func (d *Data) addUngroupedGroup() {
	ungrouped := d.groupByName("ungrouped")
	ungrouped.initialized = true

	all := d.groupByName("all")

	ungrouped.Hosts = append(ungrouped.Hosts, all.Hosts...)
	all.Hosts = make([]*Host, 0)
}

func (d *Data) removeAllGroupFromHosts() {
	all := d.groupByName("all")

	for _, host := range d.Hosts {
		for i, group := range host.Groups {
			if group == all {
				host.Groups = append(host.Groups[:i], host.Groups[i+1:]...)
				break
			}
		}
	}
}

func (d *Data) validate() error {
	type validator = func(*Data) error

	var validators = [...]validator{validateGroupInitialization, validateNoCircularReferences}
	for _, v := range validators {
		err := v(d)
		if err != nil {
			return err
		}
	}
	return nil
}

func validateGroupInitialization(d *Data) error {
	for name, group := range d.Groups {
		if !group.initialized {
			return errors.Newf("group: `%s` was not initialized", name)
		}
	}
	return nil
}

func validateNoCircularReferences(d *Data) error {
	for _, group := range d.Groups {
		if circularReference(group.Parents, group, len(d.Groups)) {
			return errors.New("circular reference detected")
		}
	}
	return nil
}

func circularReference(parents []*Group, group *Group, maxDepth int) bool {
	if maxDepth == 0 {
		return true
	}
	for _, parent := range parents {
		if parent == group || circularReference(parent.Parents, group, maxDepth-1) {
			return true
		}
	}
	return false
}

func (d *Data) groupByName(name string) *Group {
	group, ok := d.Groups[name]
	if !ok {
		group = newGroup(name)
		d.Groups[name] = group
	}
	return group
}

func (d *Data) addVarsToHost(name string, vars types.Vars) *Host {
	host := d.hostByName(name)
	for k, v := range vars {
		host.Vars[k] = v
	}
	return host
}

func (d *Data) hostByName(name string) *Host {
	host, ok := d.Hosts[name]
	if !ok {
		host = newHost(name)
		d.Hosts[name] = host
	}
	return host
}

func (h *Host) deduplicateGroups() {
	h.Groups = deduplicate(h.Groups)
}

func (g *Group) deduplicateParents() {
	g.Parents = deduplicate(g.Parents)
}

func deduplicate(array []*Group) []*Group {
	groupSet := make(map[*Group]bool)
	for _, group := range array {
		groupSet[group] = true
	}
	ret := make([]*Group, 0, len(groupSet))
	for group := range groupSet {
		ret = append(ret, group)
	}

	return ret
}

// determineHostsFromGroupsPattern takes a multi groups pattern describing a hosts list
// and returns a list of hosts that match the pattern.
// example pattern: "webservers:dbservers:&staging:!phoenix"
func (d *Data) determineHostsFromGroupsPattern(groupsPattern string) (map[string]*Host, error) {
	hosts := make(map[string]*Host)

	// Multiple groups
	// webservers:dbservers
	// all hosts in webservers plus all hosts in dbservers
	//
	// Excluding groups
	// webservers:!atlanta
	// all hosts in webservers except those in atlanta
	//
	// Intersection of groups
	// webservers:&staging
	// any hosts in webservers that are also in staging

	maybeGroupsList := strings.Split(groupsPattern, ":")
	for _, groupName := range maybeGroupsList {
		excludeGroup, intersectWithGroup := false, false
		if groupName == "" {
			return nil, errors.Newf("group `%s` does not exist", groupName)
		} else if groupName[0] == '!' {
			excludeGroup = true
			groupName = groupName[1:]
		} else if groupName[0] == '&' {
			intersectWithGroup = true
			groupName = groupName[1:]
		}

		if group, ok := d.Groups[groupName]; ok {
			groupHosts := group.GetHosts()

			if excludeGroup {
				for hostName := range groupHosts {
					delete(hosts, hostName)
				}
			} else if intersectWithGroup {
				hostsToDelete := make([]string, 0)
				for hostName, _ := range hosts {
					if _, ok := groupHosts[hostName]; !ok {
						hostsToDelete = append(hostsToDelete, hostName)
					}
				}
				for _, hostName := range hostsToDelete {
					delete(hosts, hostName)
				}
			} else {
				for hostName, host := range groupHosts {
					hosts[hostName] = host
				}
			}
		} else {
			return nil, errors.Newf("group `%s` does not exist", groupName)
		}
	}

	return hosts, nil
}

// DetermineHosts takes a pattern describing a hosts list (value of hosts property in playbook)
// and returns a list of hosts that match the pattern.
func (d *Data) DetermineHosts(pattern string) (map[string]*Host, error) {
	// Simplified version of https://docs.ansible.com/ansible/latest/user_guide/intro_patterns.html#common-patterns

	pattern = strings.TrimSpace(pattern)

	hosts := make(map[string]*Host)

	if pattern == "" {
		return hosts, nil
	}
	if pattern == "all" {
		return d.Hosts, nil
	}

	hasCommas := strings.Contains(pattern, ",")
	hasColons := strings.Contains(pattern, ":")

	// FIXME Doesn't work for literal ipv6 addresses

	if !hasCommas && !hasColons {
		// No commas or colons, so this is a single host or group
		if d.Hosts[pattern] != nil {
			hosts[pattern] = d.Hosts[pattern]
		} else if d.Groups[pattern] != nil {
			hosts = d.Groups[pattern].GetHosts()
		} else {
			return nil, errors.Newf("host or group `%s` does not exist", pattern)
		}
	} else if hasCommas {
		maybeHostsList := strings.Split(pattern, ",")
		for _, hostName := range maybeHostsList {
			if host, ok := d.Hosts[hostName]; ok {
				hosts[hostName] = host
			} else {
				return nil, errors.Newf("host `%s` does not exist", hostName)
			}
		}
	} else {
		// FIXME this parser (for simplicity) only allows ',' as hosts separator;
		// FIXME should be able to use ':' as host separator too
		return d.determineHostsFromGroupsPattern(pattern)
	}

	return hosts, nil
}
