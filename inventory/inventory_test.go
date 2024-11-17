package inventory

import (
	"github.com/scylladb/gosible/utils/maps"
	"github.com/scylladb/gosible/utils/types"
	"reflect"
	"sort"
	"testing"
)

// HELPERS

// dataEqual is a helper function to check if two datas are the same
func dataEqual(d1 *Data, d2 *Data) bool {
	return groupMapEqual(d1.Groups, d2.Groups) && hostMapEqual(d1.Hosts, d2.Hosts)
}

func groupMapEqual(g1 map[string]*Group, g2 map[string]*Group) bool {
	if len(g1) != len(g2) {
		return false
	}
	for k, v := range g1 {
		v2, ok := g2[k]
		if !ok || !groupEqual(v, v2) {
			return false
		}
	}
	return true
}

func groupEqual(g1 *Group, g2 *Group) bool {
	return g1.Name == g2.Name &&
		reflect.DeepEqual(g1.Vars, g2.Vars) &&
		nameListEqual(groupNames(g1.Parents), groupNames(g2.Parents)) &&
		nameListEqual(groupNames(g1.Children), groupNames(g2.Children)) &&
		nameListEqual(hostNames(g1.Hosts), hostNames(g2.Hosts))
}

// nameListEqual compares two string slices. Slices are sorted in function.
func nameListEqual(g1 []string, g2 []string) bool {
	sort.Strings(g1)
	sort.Strings(g2)
	return reflect.DeepEqual(g1, g2)
}

func hostMapEqual(h1 map[string]*Host, h2 map[string]*Host) bool {
	if len(h1) != len(h2) {
		return false
	}
	for k, v := range h1 {
		v2, ok := h1[k]
		if !ok || !hostEqual(v, v2) {
			return false
		}
	}
	return true
}

func hostNames(hosts []*Host) []string {
	names := make([]string, len(hosts))
	for _, host := range hosts {
		names = append(names, host.Name)
	}
	return names
}

func groupNames(hosts []*Group) []string {
	names := make([]string, len(hosts))
	for _, host := range hosts {
		names = append(names, host.Name)
	}
	return names
}

func hostEqual(h1 *Host, h2 *Host) bool {
	return h1.Name == h2.Name &&
		reflect.DeepEqual(h1.Vars, h2.Vars) &&
		nameListEqual(groupNames(h1.Groups), groupNames(h2.Groups))
}

func getBasicData() *Data {
	allGroup, ungroupedGroup := newGroup("all"), newGroup("ungrouped")
	host1, host2, host3 := newHost("scylla-cloud-grafproxy-0"), newHost("scylla-cloud-grafproxy-1"), newHost("scylla-cloud-grafproxy-2")

	host1.Vars = types.Vars{
		"ansible_host":           "54.86.186.22",
		"ansible_port":           "22",
		"ansible_user":           "centos",
		"ansible_ssh_extra_args": "-o StrictHostKeyChecking=no",
	}
	host2.Vars = types.Vars{
		"ansible_host":           "54.86.186.23",
		"ansible_port":           "22",
		"ansible_user":           "centos",
		"ansible_ssh_extra_args": "-o StrictHostKeyChecking=no",
	}
	host3.Vars = types.Vars{
		"ansible_host":           "54.86.186.24",
		"ansible_port":           "22",
		"ansible_user":           "centos",
		"ansible_ssh_extra_args": "-o StrictHostKeyChecking=no",
	}

	host1.Groups = append(host1.Groups, ungroupedGroup)
	host2.Groups = append(host2.Groups, ungroupedGroup)
	host3.Groups = append(host3.Groups, ungroupedGroup)

	allGroup.Children = append(allGroup.Children, ungroupedGroup)
	ungroupedGroup.Parents = append(ungroupedGroup.Parents, allGroup)

	ungroupedGroup.Hosts = append(ungroupedGroup.Hosts, host1, host2, host3)

	d := newData()
	d.Groups[allGroup.Name] = allGroup
	d.Groups[ungroupedGroup.Name] = ungroupedGroup

	d.Hosts[host1.Name] = host1
	d.Hosts[host2.Name] = host2
	d.Hosts[host3.Name] = host3

	return d
}

func getGroupsData() *Data {
	allGroup, ungroupedGroup, groupGroup := newGroup("all"), newGroup("ungrouped"), newGroup("group")
	host := newHost("host")

	host.Vars = types.Vars{
		"foo": "42",
	}
	groupGroup.Vars = types.Vars{
		"foo": "6",
		"bar": "9",
	}

	host.Groups = append(host.Groups, groupGroup)

	allGroup.Children = append(allGroup.Children, ungroupedGroup, groupGroup)
	ungroupedGroup.Parents = append(ungroupedGroup.Parents, allGroup)
	groupGroup.Parents = append(groupGroup.Parents, allGroup)

	groupGroup.Hosts = append(groupGroup.Hosts, host)

	d := newData()
	d.Groups[allGroup.Name] = allGroup
	d.Groups[ungroupedGroup.Name] = ungroupedGroup
	d.Groups[groupGroup.Name] = groupGroup

	d.Hosts[host.Name] = host

	return d
}

func getNotInAll() *Data {
	all, ungrouped, group, group2 := newGroup("all"), newGroup("ungrouped"), newGroup("group"), newGroup("group2")

	all.Children = append(all.Children, ungrouped, group)
	group.Children = append(group.Children, group2)

	ungrouped.Parents = append(ungrouped.Parents, all)
	group.Parents = append(group.Parents, all)
	group2.Parents = append(group2.Parents, group)

	d := newData()
	d.Groups[all.Name] = all
	d.Groups[ungrouped.Name] = ungrouped
	d.Groups[group.Name] = group
	d.Groups[group2.Name] = group2

	return d
}

func determineHostNamesByPattern(d *Data, pattern string, t *testing.T) []string {
	// This expects that the pattern is valid!

	hosts, err := d.DetermineHosts(pattern)
	if err != nil {
		t.Fatal("Could not determine hosts", err)
	}

	return maps.Keys(hosts)
}

// TESTS

func TestReadIni(t *testing.T) {
	data, err := Parse("tests/assets/basicRead.ini")
	if err != nil {
		t.Fatal("Error was not expected", err)
	}
	d := getBasicData()
	if !dataEqual(d, data) {
		t.Error("datas are different")
	}
}

func TestReadYaml(t *testing.T) {
	data, err := Parse("tests/assets/basicRead.yaml")
	if err != nil {
		t.Fatal("Error was not expected", err)
	}
	d := getBasicData()
	if !dataEqual(d, data) {
		t.Error("datas are different")
	}
}

func TestCircularIni(t *testing.T) {
	_, err := Parse("tests/assets/circular.ini")
	if err == nil {
		t.Fatal("Error expected")
	}
}

func TestCircularYaml(t *testing.T) {
	_, err := Parse("tests/assets/circular.yaml")
	if err == nil {
		t.Fatal("Error expected")
	}
}

func TestGroupsIni(t *testing.T) {
	data, err := Parse("tests/assets/groups.ini")
	if err != nil {
		t.Fatal("Error was not expected", err)
	}
	d := getGroupsData()
	if !dataEqual(d, data) {
		t.Error("datas are different")
	}
}

func TestGroupsYaml(t *testing.T) {
	data, err := Parse("tests/assets/groups.yaml")
	if err != nil {
		t.Fatal("Error was not expected", err)
	}
	d := getGroupsData()
	if !dataEqual(d, data) {
		t.Error("datas are different")
	}
}

func TestNotInAllIni(t *testing.T) {
	data, err := Parse("tests/assets/notInAll.ini")
	if err != nil {
		t.Fatal("Error was not expected", err)
	}
	d := getNotInAll()
	if !dataEqual(d, data) {
		t.Error("datas are different")
	}
}

func TestNotInAllYaml(t *testing.T) {
	data, err := Parse("tests/assets/notInAll.yaml")
	if err != nil {
		t.Fatal("Error was not expected", err)
	}
	d := getNotInAll()
	if !dataEqual(d, data) {
		t.Error("datas are different")
	}
}

func TestGetVars(t *testing.T) {
	d := getGroupsData()
	allVars := make(types.Vars)
	ungroupedVars := make(types.Vars)
	groupVars := types.Vars{
		"foo": "6",
		"bar": "9",
	}
	hostVars := types.Vars{
		"foo": "42",
		"bar": "9",
	}

	if !reflect.DeepEqual(allVars, d.Groups["all"].AllVars()) {
		t.Error("`all` group vars are different")
	}
	if !reflect.DeepEqual(ungroupedVars, d.Groups["ungrouped"].AllVars()) {
		t.Error("`ungrouped` group vars are different")
	}
	if !reflect.DeepEqual(groupVars, d.Groups["group"].AllVars()) {
		t.Error("`group` group vars are different")
	}
	if !reflect.DeepEqual(hostVars, d.Hosts["host"].AllVars()) {
		allHostVars := d.Hosts["host"].AllVars()
		_ = allHostVars
		t.Error("`host` host vars are different")
	}
}

func TestDetermineHosts(t *testing.T) {
	data, err := Parse("tests/assets/simpleManyGroups.ini")
	if err != nil {
		t.Fatal("Error was not expected", err)
	}

	determineHosts := func(pattern string) []string {
		return determineHostNamesByPattern(data, pattern, t)
	}
	assertEqual := func(pattern string, expected []string) {
		actual := determineHosts(pattern)
		if !nameListEqual(expected, actual) {
			t.Error("Hosts are different", pattern, expected, actual)
		}
	}

	// Host patterns
	assertEqual("h1", []string{"h1"})
	assertEqual("h1,h4", []string{"h1", "h4"})

	// Single group pattern
	assertEqual("g5", []string{"h4", "h5"})

	// Special group patterns
	assertEqual("all", []string{"h1", "h2", "h3", "h4", "h5"})
	assertEqual("ungrouped", []string{"h1"})

	// Multiple group patterns
	assertEqual("g1:g2:g3", []string{"h2", "h3", "h4"})
	assertEqual("all:!g1", []string{"h1", "h3", "h4", "h5"})
	assertEqual("all:&g5:ungrouped", []string{"h1", "h4", "h5"})
}
