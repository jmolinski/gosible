package inventory

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/scylladb/gosible/utils/types"
	"os"
	"regexp"
	"strings"
)

type sectionType int

const (
	section sectionType = iota
	vars
	children
)

type stateStruct struct {
	group *Group
	data  *Data
	typ   sectionType
}

func parseIni(filename string) (*Data, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	state := newState()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		err := state.handleLine(scanner.Text())
		if err != nil {
			return nil, err
		}
	}

	err = state.data.formatAndValidate()
	if err != nil {
		return nil, err
	}
	return state.data, nil
}

func newState() *stateStruct {
	state := &stateStruct{
		group: newGroup("all"),
		typ:   section,
		data:  newData(),
	}
	state.group.initialized = true
	state.data.Groups["all"] = state.group

	return state
}

func (state *stateStruct) handleLine(line string) error {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return nil
	}
	if isSection(trimmed) {
		state.handleSectionLine(trimmed)
		return nil
	}
	return state.handleDataLine(trimmed)
}

func removeComment(line string) string {
	// TODO
	return line
}

func (state *stateStruct) handleSectionLine(line string) {
	name, typ := getSectionData(line)
	state.group = state.data.groupByName(name)
	state.typ = typ

	if typ == section || typ == children {
		state.group.initialized = true
	}
}

func (state *stateStruct) handleDataLine(line string) error {
	switch state.typ {
	case section:
		name, vars, err := getHostData(line)
		if err != nil {
			return err
		}
		host := state.data.addVarsToHost(name, vars)
		host.Groups = append(host.Groups, state.group)
		state.group.Hosts = append(state.group.Hosts, host)
	case vars:
		name, value, err := getVar(line)
		if err != nil {
			return err
		}
		state.group.Vars[name] = value
	case children:
		name, err := getChild(line)
		if err != nil {
			return err
		}
		group := state.data.groupByName(name)
		group.Parents = append(group.Parents, state.group)
		state.group.Children = append(state.group.Children, group)
	default:
		return errors.New("unexpected line")
	}
	return nil
}

const nameRegexStr = `((?:\w|-|_)+)`

const dataRegexStr = nameRegexStr + `=('.+'|".+"|\S+)`

var dataRegex = regexp.MustCompile(dataRegexStr)

var hostRegexStr = fmt.Sprintf(`^%s(\s+%s)*$`, nameRegexStr, dataRegexStr)
var hostRegex = regexp.MustCompile(hostRegexStr)
var nameRegex = regexp.MustCompile(nameRegexStr)
var nameLineRegex = regexp.MustCompile("^" + nameRegexStr + "$")

var sectionRegex = regexp.MustCompile(`^\[\S+(?::(?:vars|children))?]$`)

func getChild(line string) (string, error) {
	match := nameLineRegex.FindString(line)
	if match == "" {
		return "", errors.New("unexpected child line format")
	}
	return match, nil
}

func getVar(line string) (string, string, error) {
	unexpectedFormatErr := errors.New("unexpected Vars line format")
	if !dataRegex.MatchString(line) {
		return "", "", unexpectedFormatErr
	}
	split := strings.SplitN(line, "=", 2)
	return split[0], split[1], nil
}

func getHostData(line string) (string, types.Vars, error) {
	if !hostRegex.MatchString(line) {
		return "", nil, errors.New("unexpected host line format")
	}
	var vars = make(types.Vars)
	for _, variable := range dataRegex.FindAllString(line, -1) {
		split := strings.SplitN(variable, "=", 2)
		vars[split[0]] = split[1]
	}
	name := nameRegex.FindAllString(line, 1)[0]
	return name, vars, nil
}

func getSectionData(trimmed string) (string, sectionType) {
	noBrackets := strings.Trim(trimmed, "[]")
	split := strings.Split(noBrackets, ":")
	typ := section
	if len(split) == 2 {
		switch split[1] {
		case "vars":
			typ = vars
		case "children":
			typ = children
		}
	}
	return split[0], typ
}

func isSection(line string) bool {
	return sectionRegex.MatchString(line)
}
