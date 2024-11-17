package modules

import (
	"github.com/scylladb/gosible/remote/proto"
	"github.com/scylladb/gosible/utils/types"
)

// Return is struct that has to be returned by each module.
// https://docs.ansible.com/ansible/latest/reference_appendices/common_return_values.html#msg
type Return struct {
	BackupFile string        // For those modules that implement backup=no|yes when manipulating files, a path to the backup file created.
	Changed    bool          // A boolean indicating if the task had to make changes to the target or delegated host.
	Diff       *Diff         // Information on differences between the previous and current state. Often a dictionary with entries before and after, which will then be formatted by the callback plugin to a diff view.
	Failed     bool          // A boolean that indicates if the task was failed or not.
	Invocation interface{}   // Information on how the module was invoked.
	Msg        string        // A string with a generic message relayed to the user.
	Rc         int           // Some modules execute command line utilities or are geared for executing commands directly (raw, shell, command, and so on), this field contains ‘return code’ of these utilities.
	Results    []interface{} // If this key exists, it indicates that a loop was present for the task and that it contains a list of the normal module ‘result’ per item.
	Skipped    bool          // A boolean that indicates if the task was skipped or not
	Stderr     []byte        // Some modules execute command line utilities or are geared for executing commands directly (raw, shell, command, and so on), this field contains the error output of these utilities.
	Stdout     []byte        // Some modules execute command line utilities or are geared for executing commands directly (raw, shell, command, and so on). This field contains the normal output of these utilities.

	ModuleSpecificReturn interface{} // Each module can set here its additional variables.

	*InternalReturn
}

type Diff struct {
	Before any
	After  any
}

type InternalReturn struct {
	AnsibleFacts       types.Facts   // This key should contain a dictionary which will be appended to the facts assigned to the host. These will be directly accessible and don’t require using a registered variable.
	FactBucket         string        // Some modules need to assign the facts as non-persistent or as host vars instead. In these cases, this property can be set to specify the bucket into which the facts will be assigned.
	Exception          string        // This key can contain traceback information caused by an exception in a module. It will only be displayed on high verbosity (-vvv).
	Warnings           []string      // This key contains a list of strings that will be presented to the user.
	Debug              []string      // This key contains a list of debug strings.
	Deprecations       []Deprecation // This key contains a list of deprecations that will be presented to the user.
	NeedsPythonRuntime bool          `json:"NeedsPythonRuntime,omitempty"` // If this is set to true, the Python runtime should be uploaded and the task should be retried.
}

const (
	BucketAnsibleFacts              = "ansible_facts"
	BucketNonPersistentAnsibleFacts = "np_ansible_facts"
	BucketHostVars                  = "host_vars"
)

type Deprecation struct {
	Msg            string
	Version        string
	Date           string
	CollectionName string
}

type Module interface {
	Name() string
	Run(context *RunContext, vars types.Vars) *Return
}

type RunContext struct {
	MetaArgs *proto.MetaArgs // List of meta arguments
}
