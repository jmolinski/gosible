package inventory

import (
	"errors"
	"github.com/scylladb/gosible/utils/types"
	"gopkg.in/yaml.v2"
	"os"
)

type orderedRawData = yaml.MapSlice

type typedRawGroup struct {
	Hosts    map[string]types.Vars    `yaml:"hosts"`
	Children map[string]typedRawGroup `yaml:"children"`
	Vars     types.Vars               `yaml:"vars"`
}

type typedRawData = map[string]typedRawGroup

func parseYAML(filename string) (*Data, error) {
	dat, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var typedRaw typedRawData
	err = yaml.Unmarshal(dat, &typedRaw)
	if err != nil {
		return nil, err
	}

	var orderedRaw orderedRawData
	err = yaml.Unmarshal(dat, &orderedRaw)
	if err != nil {
		return nil, err
	}

	return getData(typedRaw, orderedRaw)
}

// getData merges data in unordered and typed format and in ordered and untyped format to produce final data.
// This is important as order in which vars are declared is important if Group / Host is redeclared.
func getData(typedRaw typedRawData, orderedRaw yaml.MapSlice) (*Data, error) {
	var data = newData()
	allGroup := data.groupByName("all")
	allGroup.initialized = true
	for _, el := range orderedRaw {
		key, ok := el.Key.(string)
		if !ok {
			return nil, errors.New("unexpected key type")
		}
		groupData := typedRaw[key]
		_, err := data.addGroup(&groupData, &el)
		if err != nil {
			return nil, err
		}
	}

	err := data.formatAndValidate()
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (d *Data) addGroup(raw *typedRawGroup, el *yaml.MapItem) (*Group, error) {
	groupName, okKey := el.Key.(string)
	if !okKey {
		return nil, errors.New("unexpected type")
	}

	group := d.groupByName(groupName)
	group.initialized = true

	groupData, okValue := el.Value.(orderedRawData)
	if !okValue {
		if el.Value == nil {
			return group, nil
		}
		return nil, errors.New("unexpected type")
	}

	for _, child := range groupData {
		err := d.processGroupElement(&child, group, raw)
		if err != nil {
			return nil, err
		}
	}
	return group, nil
}

func (d *Data) processGroupElement(el *yaml.MapItem, group *Group, raw *typedRawGroup) error {
	value, okValue := el.Value.(orderedRawData)
	key, okKey := el.Key.(string)
	if !okValue || !okKey {
		return errors.New("unexpected type")
	}
	switch key {
	case "hosts":
		hosts := d.addHosts(raw.Hosts)
		group.Hosts = append(group.Hosts, hosts...)
		for _, host := range hosts {
			host.Groups = append(host.Groups, group)
		}
	case "vars":
		for k, v := range raw.Vars {
			group.Vars[k] = v
		}
	case "children":
		groups, err := d.addGroups(raw.Children, value)
		if err != nil {
			return err
		}
		group.Children = append(group.Children, groups...)
		for _, g := range groups {
			g.Parents = append(g.Parents, group)
		}
	}
	return nil
}

func (d *Data) addHosts(hosts map[string]types.Vars) []*Host {
	var hostPointers []*Host
	for name, data := range hosts {
		host := d.addVarsToHost(name, data)
		hostPointers = append(hostPointers, host)
	}
	return hostPointers
}

func (d *Data) addGroups(groups map[string]typedRawGroup, order orderedRawData) (groupPointers []*Group, err error) {
	for _, el := range order {
		key, okKey := el.Key.(string)
		if !okKey {
			return nil, errors.New("unexpected type")
		}
		var group *Group
		switch el.Value.(type) {
		case nil:
			group = d.groupByName(key)
			group.initialized = true
		case yaml.MapItem:
			groupData := groups[key]
			value := el.Value.(yaml.MapItem)
			group, err = d.addGroup(&groupData, &value)
			if err != nil {
				return
			}
		}
		groupPointers = append(groupPointers, group)
	}
	return
}
