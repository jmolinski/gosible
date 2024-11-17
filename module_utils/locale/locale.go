package locale

import (
	"errors"
	"fmt"
	"github.com/scylladb/gosible/module_utils/gosibleModule"
	"strings"
)

const newline = "\n"

func GetBestParsableLocale[P gosibleModule.Validatable](module *gosibleModule.GosibleModule[P], preferences []string, errorOnLocale bool) (string, error) {
	found := "C" // default posix, its ascii but always there
	locale, err := module.GetBinPath("locale", nil, true)
	if err != nil {
		return "", nil
	}
	if locale == "" && errorOnLocale {
		return "", errors.New("could not find 'locale' tool")
	}
	if len(preferences) == 0 {
		// new POSIX standard or English cause those are messages core team expects
		// yes, the last 2 are the same but some systems are weird
		preferences = []string{"C.utf8", "en_US.utf8", "C", "POSIX"}
	}

	var available []string
	res, err := module.RunCommand([]string{locale, "-a"}, gosibleModule.RunCommandDefaultKwargs())
	if res.Rc == 0 {
		if len(res.Stdout) != 0 {
			available = strings.Split(strings.TrimSpace(string(res.Stdout)), newline)
		} else if errorOnLocale {
			return "", fmt.Errorf("no output from locale, rc=%d: %s", res.Rc, string(res.Stderr))
		}
	} else if errorOnLocale {
		return "", fmt.Errorf("unable to get locale information, rc=%d: %s", res.Rc, string(res.Stderr))
	}

	for _, pref := range preferences {
		for _, a := range available {
			if a == pref {
				return pref, nil
			}
		}
	}

	return found, nil
}
