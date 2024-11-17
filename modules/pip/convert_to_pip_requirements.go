package pip

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/scylladb/gosible/module_utils/gosibleModule"
	"strings"
)

const pythonCodeTemplate = `from __future__ import print_function
import json
import sys
import re

try:
    from pkg_resources import Requirement
except ImportError:
    print(traceback.format_exc(), file=sys.stderr)
    exit(%d)

# From ansible for max compat.
class Package:
    """Python distribution package metadata wrapper.

    A wrapper class for Requirement, which provides
    API to parse package name, version specifier,
    test whether a package is already satisfied.
    """

    _CANONICALIZE_RE = re.compile(r"[-_.]+")

    def __init__(self, name_string, version_string=None):
        self._plain_package = False
        self.package_name = name_string
        self._requirement = None

        if version_string:
            version_string = version_string.lstrip()
            separator = "==" if version_string[0].isdigit() else " "
            name_string = separator.join((name_string, version_string))
        try:
            self._requirement = Requirement.parse(name_string)
            # old pkg_resource will replace 'setuptools' with 'distribute' when it's already installed
            if (
                self._requirement.project_name == "distribute"
                and "setuptools" in name_string
            ):
                self.package_name = "setuptools"
                self._requirement.project_name = "setuptools"
            else:
                self.package_name = Package.canonicalize_name(
                    self._requirement.project_name
                )
            self._plain_package = True
        except ValueError as e:
            pass

    @property
    def has_version_specifier(self):
        if self._plain_package:
            return bool(self._requirement.specs)
        return False

    @staticmethod
    def canonicalize_name(name):
        # This is taken from PEP 503.
        return Package._CANONICALIZE_RE.sub("-", name).lower()

    def __str__(self):
        if self._plain_package:
            return str(self._requirement)
        return self.package_name

data = [%s] # Here data mus be provided from go.
packages = []
for name, version in data:
    p = Package(name, version)
    packages.append({"HasVersionSpecifier": p.has_version_specifier, "Str": str(p)})

print(json.dumps(packages))
`

const noSetupToolsRc = 42

func getPythonCode(requirements []requirementData) string {
	strReq := make([]string, 0, len(requirements))
	for _, req := range requirements {
		strReq = append(strReq, fmt.Sprintf("('%s', '%s')", req.name, req.version))
	}
	joinedReq := strings.Join(strReq, ", ")

	return fmt.Sprintf(pythonCodeTemplate, noSetupToolsRc, joinedReq)
}

// convertToPipRequirements executes python code to get data about pip package.
func (m *Module) convertToPipRequirements(pythonPath string, requirements []requirementData) ([]requirement, error) {
	// TODO write this function in pure golang without using python calls.
	kwargs := gosibleModule.RunCommandDefaultKwargs()
	kwargs.Data = []byte(getPythonCode(requirements))
	res, err := m.RunCommand([]string{pythonPath, "--"}, kwargs)
	if err != nil {
		return nil, err
	}
	if res.Rc == noSetupToolsRc {
		return nil, errors.New(string(res.Stderr))
	}
	reqs := make([]requirement, 0, len(requirements))
	err = json.Unmarshal(res.Stdout, &reqs)
	return reqs, err
}
