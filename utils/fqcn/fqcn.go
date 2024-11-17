package fqcn

import "strings"

// ToInternalFcqns given a action/module name, returns a list of this name
// with the same names with the prefixes `ansible.builtin.` and `ansible.legacy.`
// added for all names that are not already FQCNs.
func ToInternalFcqns(name string) []string {
	if strings.Contains(name, ".") {
		return []string{name}
	}
	return []string{name, "ansible.builtin." + name, "ansible.legacy." + name}
}
