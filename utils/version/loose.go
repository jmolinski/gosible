package version

import (
	"strconv"
	"strings"
)

// Loose is a very simplified adaptation of a class ansible.module_utils.compat.version.Loose.
type Loose []int64

func NewLoose(s string) (l Loose, err error) {
	parts := strings.Split(s, ".")
	l = make([]int64, 0, len(parts))
	for i, p := range parts {
		l[i], err = strconv.ParseInt(p, 10, 64)
		if err != nil {
			return
		}
	}
	return
}

func (l Loose) Compare(l2 Loose) int {
	for i, p := range l {
		if i >= len(l2) {
			return 1
		}
		if p > l2[i] {
			return 1
		} else if p < l2[i] {
			return -1
		}
	}
	if len(l2) > len(l) {
		return -1
	}
	return 0
}
