package gosibleModule

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"fmt"
	"github.com/alessio/shellescape"
	"github.com/google/shlex"
	"github.com/hhkbp2/go-strftime"
	"github.com/mitchellh/mapstructure"
	"github.com/scylladb/gosible/module_utils/file"
	"github.com/scylladb/gosible/module_utils/selinux"
	"github.com/scylladb/gosible/modules"
	pb "github.com/scylladb/gosible/remote/proto"
	"github.com/scylladb/gosible/utils/maps"
	"github.com/scylladb/gosible/utils/osUtils"
	pathUtils "github.com/scylladb/gosible/utils/path"
	"github.com/scylladb/gosible/utils/slices"
	"github.com/scylladb/gosible/utils/types"
	"github.com/scylladb/gosible/utils/wrappers"
	"golang.org/x/crypto/md4"
	"golang.org/x/crypto/ripemd160"
	"golang.org/x/crypto/sha3"
	"hash"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Validatable interface {
	Validate() error
}

type closeFn func() error

type GosibleModule[P Validatable] struct {
	shell                   *string
	RunCommandEnvironUpdate map[string]string
	MetaArgs                *pb.MetaArgs
	*wrappers.Return
	Params          P
	se              *selinux.Selinux
	tmpDir          string
	remoteTmp       string
	keepRemoteFiles bool
	closeFns        []closeFn
}

type RunCommandKwargs struct {
	CheckRc                   bool
	Executable                string
	Data                      []byte
	PathPrefix                string
	Cwd                       string
	UseUnsafeShell            bool
	PromptRegex               string
	EnvironUpdate             map[string]string
	ExpandUserAndVars         bool
	PassFds                   []*os.File
	BeforeCommunicateCallback func(cmd *exec.Cmd, stdin io.Writer, stdout io.Reader, stderr *bytes.Buffer) error
	IgnoreInvalidCwd          bool
	HandleExceptions          bool
}

type RunCommandResult struct {
	Rc     int
	Stdout []byte
	Stderr []byte
}

// Se stands for SELinux file context.
type Se struct {
	Level   string `mapstructure:"selevel"`
	Role    string `mapstructure:"serole"`
	Type    string `mapstructure:"setype"`
	User    string `mapstructure:"seuser"`
	Context []string
}

type FileCommonParams struct {
	Attributes   string      `mapstructure:"attributes"`
	Path         string      `mapstructure:"path"`
	Dest         string      `mapstructure:"dest"`
	Group        string      `mapstructure:"group"`
	Mode         interface{} `mapstructure:"mode"`
	Owner        string      `mapstructure:"owner"`
	UnsafeWrites bool        `mapstructure:"unsafe_writes"`
	Follow       bool        `mapstructure:"follow"`
	Se           `mapstructure:",squash"`
}

func PrepareVarsForFileCommonParams(vars types.Vars) types.Vars {
	if _, ok := vars["attributes"].(string); !ok {
		vars["attributes"] = vars["attrs"]
	}
	return vars
}

func NewFileCommonParams() *FileCommonParams {
	return &FileCommonParams{}
}

func (f *FileCommonParams) Validate() error {
	return nil
}

func New[P Validatable](defaultParams P) *GosibleModule[P] {
	return &GosibleModule[P]{
		Return: wrappers.NewReturn(),
		Params: defaultParams,
		se:     selinux.New(),
	}
}

func (rcr *RunCommandResult) Validate() error {
	if rcr.Rc != 0 {
		return fmt.Errorf("command failed rc=%d, out=%s, err=%s", rcr.Rc, rcr.Stdout, rcr.Stderr)
	}
	return nil
}

func RunCommandDefaultKwargs() *RunCommandKwargs {
	return &RunCommandKwargs{
		ExpandUserAndVars: true,
		IgnoreInvalidCwd:  true,
		HandleExceptions:  true,
	}
}

func (m *GosibleModule[P]) ParseParams(ctx *modules.RunContext, vars types.Vars) error {
	m.MetaArgs = ctx.MetaArgs
	if err := mapstructure.Decode(vars, m.Params); err != nil {
		return err
	}
	return m.Params.Validate()
}

// LoadFileCommonParams as many modules deal with files, this encapsulates common
// options that the file module accepts such that it is directly
// available to all modules and they can share code.
// Allows to overwrite the path/dest module argument by providing path.
func (m *GosibleModule[P]) LoadFileCommonParams(vars types.Vars, path string) (*FileCommonParams, error) {
	if path == "" {
		if v, ok := vars["path"].(string); ok {
			path = v
		} else if v, ok = vars["dest"].(string); ok {
			path = v
		} else {
			return nil, nil
		}
	}
	p := FileCommonParams{}
	if err := mapstructure.Decode(vars, &p); err != nil {
		return nil, err
	}
	if p.Follow {
		link, err := pathUtils.IsSymLink(path)
		if err != nil {
			return nil, err
		}
		if link {
			path, err = os.Readlink(path)
			if err != nil {
				return nil, err
			}
		}
	}
	p.Se.Context = []string{p.Se.User, p.Se.Role, p.Se.Type}

	enabled, err := m.se.MlsEnabled()
	if err != nil {
		return nil, err
	}
	if enabled {
		p.Se.Context = append(p.Se.Context, p.Se.Level)
	}
	defaultSeContext, err := m.se.DefaultContext(path)
	for i, dCtx := range defaultSeContext {
		if p.Se.Context[i] == "_default" {
			p.Se.Context[i] = dCtx
		}
	}

	p.Path = path
	return &p, nil
}

// Special Rcs

const InternalErrorRc = 257

func (m *GosibleModule[P]) GetBinPath(arg string, optDirs []string, errOnNotFound bool) (string, error) {
	return pathUtils.GetBinPath(arg, optDirs, errOnNotFound)
}

func (m *GosibleModule[P]) RunCommand(rawArgs interface{}, kwargs *RunCommandKwargs) (*RunCommandResult, error) {
	promptRe, err := m.getPromptRe(kwargs)
	if err != nil {
		return nil, err
	}
	cmd, err := m.getCmd(rawArgs, kwargs)
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	var stdinPipe io.WriteCloser
	if kwargs.Data != nil || kwargs.BeforeCommunicateCallback != nil {
		if stdinPipe, err = cmd.StdinPipe(); err != nil {
			return nil, err
		}
	}

	if err = cmd.Start(); err != nil {
		return nil, err
	}
	if kwargs.BeforeCommunicateCallback != nil {
		if err = kwargs.BeforeCommunicateCallback(cmd, stdinPipe, stdoutPipe, &stderr); err != nil {
			return nil, err
		}
	}

	if kwargs.Data != nil {
		if _, err = stdinPipe.Write(kwargs.Data); err != nil {
			return nil, err
		}
	}
	if kwargs.Data != nil || kwargs.BeforeCommunicateCallback != nil {
		if err = stdinPipe.Close(); err != nil {
			return nil, err
		}
	}

	stdout, err := m.readOutput(stdoutPipe, promptRe, kwargs)
	if err != nil {
		return m.readOutputError(err, stdout, cmd)
	}

	if err = cmd.Wait(); err != nil {
		return m.waitError(err, stdout, stderr, kwargs)
	}

	return &RunCommandResult{
		Rc:     0,
		Stdout: stdout,
		Stderr: stderr.Bytes(),
	}, nil
}

var availableHashAlgorithms = map[string]func() hash.Hash{
	"md4":        md4.New,
	"md5":        md5.New,
	"sha1":       sha1.New,
	"sha224":     sha256.New224,
	"sha256":     sha256.New,
	"sha384":     sha512.New384,
	"sha512":     sha512.New,
	"sha512_224": sha512.New512_224,
	"sha512_256": sha512.New512_256,
	"ripemd160":  ripemd160.New,
	"sha3_224":   sha3.New224,
	"sha3_256":   sha3.New256,
	"sha3_384":   sha3.New384,
	"sha3_512":   sha3.New512,
}

// DigestFromFile Return hex digest of local file for a digest algorithm specified by name, or "" if file is not present.
func (m *GosibleModule[P]) DigestFromFile(filename, algorithm string) ([]byte, error) {
	exists, err := pathUtils.Exists(filename)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	isDir, err := pathUtils.IsDir(filename)
	if err != nil {
		return nil, err
	}
	if isDir {
		return nil, fmt.Errorf("attempted to take checksum of directory: %s", filename)
	}
	algFn, ok := availableHashAlgorithms[algorithm]
	if !ok {
		return nil, fmt.Errorf("could not hash file '%s' with algorithm '%s'. Available algorithms: %s", filename, algorithm, strings.Join(maps.Keys(availableHashAlgorithms), ", "))
	}
	alg := algFn()
	f, err := os.Open(filename)
	defer f.Close()
	_, err = io.Copy(alg, f)
	if err != nil {
		return nil, err
	}
	return alg.Sum(nil), nil
}

func (m *GosibleModule[P]) Md5(filename string) ([]byte, error) {
	return m.DigestFromFile(filename, "md5")
}

func (m *GosibleModule[P]) Sha1(filename string) ([]byte, error) {
	return m.DigestFromFile(filename, "sha1")
}

func (m *GosibleModule[P]) Sha256(filename string) ([]byte, error) {
	return m.DigestFromFile(filename, "sha256")
}

func (m *GosibleModule[P]) readOutput(stdoutPipe io.Reader, promptRe *regexp.Regexp, kwargs *RunCommandKwargs) ([]byte, error) {
	stdout := make([]byte, 0, 4096)
	buffer := make([]byte, 4096)
	for {
		n, err := stdoutPipe.Read(buffer)
		if n == 0 {
			return stdout, nil
		}
		if err != nil {
			return nil, err
		}
		stdout = append(stdout, buffer[:n]...)
		if promptRe != nil {
			if (*promptRe).Find(stdout) != nil && kwargs.Data == nil {
				const msg = "a prompt was encountered while running a command, but no input data was specified"
				return stdout, errors.New(msg)
			}
		}
	}
}

func (m *GosibleModule[P]) SetFsAttributesIfDifferent(params *FileCommonParams, diff *modules.Diff, expand bool) (bool, error) {
	changedContext, err := m.SetContextIfDifferent(params.Path, params.Se.Context, diff)
	if err != nil {
		return false, nil
	}
	changedOwner, err := m.SetOwnerIfDifferent(params.Path, params.Owner, diff, expand)
	if err != nil {
		return false, nil
	}
	changedGroup, err := m.SetGroupIfDifferent(params.Path, params.Group, diff, expand)
	if err != nil {
		return false, nil
	}
	changedMode, err := m.SetModeIfDifferent(params.Path, params.Mode, diff, expand)
	if err != nil {
		return false, nil
	}
	changedAttributes, err := m.SetAttributesIfDifferent(params.Path, params.Attributes, diff, expand)
	if err != nil {
		return false, nil
	}
	return changedContext || changedOwner || changedGroup || changedMode || changedAttributes, nil
}

// TmpDir return module's temporary directory path.
func (m *GosibleModule[P]) TmpDir() (string, error) {
	// if _ansible_tmpdir was not set and we have a remote_tmp,
	// the module needs to create it and clean it up once finished.
	// otherwise we create our own module tmp dir from the system defaults
	if m.tmpDir != "" {
		return m.tmpDir, nil
	}
	var baseDir string
	if m.remoteTmp != "" {
		baseDir = pathUtils.ExpandUserAndEnv(m.remoteTmp)
	}
	if baseDir != "" {
		exists, err := pathUtils.Exists(baseDir)
		if err != nil {
			return "", err
		}
		if !exists {
			if err = os.MkdirAll(baseDir, 0700); err == nil {
				m.Warn(fmt.Sprintf("Module remote_tmp %s did not exist and was created with a mode of 0700, this may cause issues when running as another user. To avoid this, create the remote_tmp dir with the correct permissions manually", baseDir))
			} else {
				m.Warn(fmt.Sprintf("Unable to use %s as temporary directory, failing back to system: %s", baseDir, err.Error()))
				baseDir = ""
			}
		}
	}

	baseFile := fmt.Sprintf("ansible-moduletmp-%d-", time.Now().Unix())
	tmpDir, err := os.MkdirTemp(baseDir, baseFile)
	if err != nil {
		return "", fmt.Errorf("failed to create remote module tmp path at dir %s with prefix %s: %v", baseDir, baseFile, err)
	}
	m.tmpDir = tmpDir
	if !m.keepRemoteFiles {
		m.registerCloseFunction(func() error { return os.RemoveAll(tmpDir) })
	}
	return tmpDir, nil
}

func (m *GosibleModule[P]) registerCloseFunction(fn closeFn) {
	m.closeFns = append(m.closeFns, fn)
}

func (m *GosibleModule[P]) Close() error {
	for _, fn := range m.closeFns {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

// BackupLocal makes a date-marked backup of the specified file.
func (m *GosibleModule[P]) BackupLocal(filename string) (backupDest string, err error) {
	exists, err := pathUtils.Exists(filename)
	if err != nil || !exists {
		return
	}
	// backups named basename.PID.YYYY-MM-DD@HH:MM:SS~
	ext := strftime.Format("%Y-%m-%d@%H:%M:%S~", time.Now().Local())
	pid := os.Getpid()
	backupDest = fmt.Sprintf("%s.%d.%s", filename, pid, ext)
	err = m.PreservedCopy(filename, backupDest)
	if err != nil {
		return "", err
	}
	return
}

// AtomicMove atomically moves src to dest, copying attributes from dest, returns true on success
// it uses os.rename to ensure this as it is an atomic operation, rest of the function is
// to work around limitations, corner cases and ensure selinux context is saved if possible
func (m *GosibleModule[P]) AtomicMove(src, dest string, unsafeWrites bool) error {
	copyStatErr := pathUtils.CopyStat(dest, src)
	ctx, err := m.GetContextBasedOnCopyStatError(dest, copyStatErr)
	if err != nil {
		return err
	}
	creating := os.IsNotExist(copyStatErr)
	// Optimistically try a rename, solves some corner cases and can avoid useless work, return error if not atomic.
	renameErr := os.Rename(src, dest)
	if renameErr != nil {
		if err = m.TryRenameWorkarounds(src, dest, renameErr, ctx, unsafeWrites); err != nil {
			return err
		}
	}
	if creating {
		// make sure the file has the correct permissions based on the current value of umask
		umask := osUtils.GetUmask()
		if err = os.Chmod(dest, file.DefaultPerm & ^os.FileMode(umask)); err != nil {
			return err
		}
		// We're okay with trying our best here.  If the user is not root (or old Unices) they won't be able to chown.
		_ = os.Chown(dest, os.Geteuid(), os.Getegid())
	}
	if m.se.Enabled() {
		// rename might not preserve context
		_, err = m.SetContextIfDifferent(dest, ctx, nil)
		return err
	}
	return nil
}

func (m *GosibleModule[P]) TryRenameWorkarounds(src, dest string, renameError error, context []string, unsafeWrites bool) error {
	if !os.IsPermission(renameError) {
		// TODO Ansible test for more errors but don't know how to test for them in golang.
		return fmt.Errorf("could not replace file: %s to %s: %v", src, dest, renameError)
	}
	destDir := filepath.Dir(dest)
	destSuffix := filepath.Base(dest)
	temp, err := os.CreateTemp(destDir, ".ansible_tmp-*-"+destSuffix)
	if err != nil {
		if !unsafeWrites {
			return fmt.Errorf("the destination directory (%s) is not writable by the current user. Error was: %v", destDir, err)
		}
		// sadly there are some situations where we cannot ensure atomicity, but only if the user insists and we get the appropriate error we update the file unsafely.
		if err = pathUtils.Copy(src, dest); err != nil {
			return fmt.Errorf("could not write data to file (%s) from (%s): %v", dest, src, err)
		}
	}
	if temp == nil {
		return nil
	}
	if err = m.tryRenameWithTemp(temp, src, dest, context); err != nil {
		if unsafeWrites {
			return pathUtils.Copy(src, dest)
		}
		return fmt.Errorf("failed to replace file: %s to %s: %v", src, dest, err)
	}

	return nil
}

func (m *GosibleModule[P]) tryRenameWithTemp(temp *os.File, src, dest string, context []string) error {
	tempName := temp.Name()
	defer m.Cleanup(tempName)
	if err := temp.Close(); err != nil {
		return err
	}
	if os.Rename(src, tempName) != nil {
		if err := pathUtils.Copy(src, dest); err != nil {
			return err
		}
		if err := pathUtils.CopyStat(src, dest); err != nil {
			return err
		}
	}
	if m.se.Enabled() {
		if _, err := m.SetContextIfDifferent(tempName, context, nil); err != nil {
			return err
		}
	}
	uid, gid, err := pathUtils.UserAndGroup(tempName, false)
	if err == nil {
		err = os.Chown(tempName, uid, gid)
	} else if !os.IsPermission(err) {
		return err
	}
	if err = os.Rename(src, dest); err != nil {
		// TODO Ansible test for EBUSY and tries to copy file then but I don't know how to test for this err in golang.
		return fmt.Errorf("unable to make %s into to %s, failed final rename from %s: %v", src, dest, tempName, err)
	}
	return nil
}

func (m *GosibleModule[P]) GetContextBasedOnCopyStatError(path string, err error) ([]string, error) {
	if err == nil {
		if m.se.Enabled() {
			return m.se.Context(path)
		}
	} else {
		if !os.IsNotExist(err) && !os.IsPermission(err) {
			return nil, err
		}
		if m.se.Enabled() {
			return m.se.DefaultContext(path)
		}
	}
	return nil, nil
}

func (m *GosibleModule[P]) SetContextIfDifferent(path string, context []string, diff *modules.Diff) (bool, error) {
	enabled := m.se.Enabled()
	if !enabled {
		return false, nil
	}
	curContext, newContext, err := m.getCurrentAndNewContext(path, context)
	if err != nil {
		return false, err
	}
	if slices.Equal(curContext, newContext) {
		return false, nil
	}
	if diff != nil {
		if _, ok := diff.Before.(map[string]any); !ok {
			diff.Before = map[string]any{}
		}
		if _, ok := diff.After.(map[string]any); !ok {
			diff.After = map[string]any{}
		}
		diff.Before.(map[string]any)["secontext"] = curContext
		diff.After.(map[string]any)["secontext"] = newContext
	}
	if err = m.se.LSetFileCon(path, newContext); err != nil {
		return false, fmt.Errorf("set selinux context failed, %v", err)
	}
	return true, nil
}

func (m *GosibleModule[P]) getCurrentAndNewContext(path string, context []string) ([]string, []string, error) {
	curContext, err := m.se.Context(path)
	if err != nil {
		return nil, nil, err
	}
	newContext := slices.Copy(curContext)
	isSpecialSe, spContext, err := m.se.IsSpecialPath(path)
	if err != nil {
		return nil, nil, err
	}
	if isSpecialSe {
		newContext = spContext
	} else {
		for i, c := range context {
			if len(curContext) <= i {
				break
			}
			if c != "" && c != curContext[i] {
				newContext[i] = c
			} else if c == "" {
				newContext[i] = curContext[i]
			}
		}
	}
	return curContext, newContext, nil
}

func (m *GosibleModule[P]) SetOwnerIfDifferent(path string, owner string, diff *modules.Diff, expand bool) (bool, error) {
	if owner == "" {
		return false, nil
	}
	if expand {
		path = pathUtils.ExpandUserAndEnv(path)
	}

	origUid, _, err := pathUtils.UserAndGroup(path, false)
	var uid64 int64
	if uid64, err = strconv.ParseInt(owner, 10, 32); err != nil {
		ownerErr := fmt.Errorf("chown failed: failed to look up user %s", owner)
		usr, err := user.Lookup(owner)
		if err != nil {
			return false, ownerErr
		}
		if uid64, err = strconv.ParseInt(usr.Uid, 10, 32); err != nil {
			return false, ownerErr
		}
	}
	uid := int(uid64)
	if origUid == uid {
		if diff != nil {
			if _, ok := diff.Before.(map[string]any); !ok {
				diff.Before = map[string]any{}
			}
			diff.Before.(map[string]any)["owner"] = origUid
			if _, ok := diff.After.(map[string]any); !ok {
				diff.After = map[string]any{}
			}
			diff.After.(map[string]any)["owner"] = uid
		}
	}
	if err = os.Lchown(path, uid, -1); err != nil {
		return false, fmt.Errorf("chown failed: %v", err)
	}
	return true, nil
}

func (m *GosibleModule[P]) SetGroupIfDifferent(path string, group string, diff *modules.Diff, expand bool) (bool, error) {
	if group == "" {
		return false, nil
	}
	if expand {
		path = pathUtils.ExpandUserAndEnv(path)
	}

	_, origGid, err := pathUtils.UserAndGroup(path, false)
	var gid64 int64
	if gid64, err = strconv.ParseInt(group, 10, 32); err != nil {
		ownerErr := fmt.Errorf("chgrp failed: failed to look up group %s", group)
		grp, err := user.LookupGroup(group)
		if err != nil {
			return false, ownerErr
		}
		if gid64, err = strconv.ParseInt(grp.Gid, 10, 32); err != nil {
			return false, ownerErr
		}
	}
	gid := int(gid64)
	if origGid == gid {
		if diff != nil {
			if _, ok := diff.Before.(map[string]any); !ok {
				diff.Before = map[string]any{}
			}
			diff.Before.(map[string]any)["group"] = origGid
			if _, ok := diff.After.(map[string]any); !ok {
				diff.After = map[string]any{}
			}
			diff.After.(map[string]any)["group"] = gid
		}
	}
	if err = os.Lchown(path, -1, gid); err != nil {
		return false, fmt.Errorf("chown failed: %v", err)
	}
	return true, nil
}

func (m *GosibleModule[P]) SetModeIfDifferent(path string, mode interface{}, diff *modules.Diff, expand bool) (bool, error) {
	if mode == nil {
		return false, nil
	}
	if expand {
		path = pathUtils.ExpandUserAndEnv(path)
	}
	stat, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	fileMode, err := m.parseMode(mode, stat)
	if err != nil {
		return false, err
	}
	prevMode := stat.Mode()
	if prevMode == fileMode {
		return false, nil
	}
	if err = pathUtils.LChmod(path, fileMode); err != nil {
		link, linkErr := pathUtils.IsSymLink(path)
		if linkErr != nil {
			return false, linkErr
		}
		if !link && !os.IsPermission(err) {
			return false, err
		}
	}
	stat, err = os.Lstat(path)
	if err != nil {
		return false, err
	}
	return stat.Mode() != prevMode, nil
}

func (m *GosibleModule[P]) parseMode(mode interface{}, stat os.FileInfo) (os.FileMode, error) {
	switch v := mode.(type) {
	case int:
		return os.FileMode(v), nil
	case string:
		mod64, err := strconv.ParseInt(v, 8, 32)
		if err == nil {
			return os.FileMode(mod64), nil
		}
		return symbolicModeToOctal(stat, v)
	default:
		return 0, errors.New("unsupported type for mode")
	}
}

func (m *GosibleModule[P]) SetAttributesIfDifferent(path string, attributes string, diff *modules.Diff, expand bool) (bool, error) {
	if attributes == "" {
		return false, nil
	}
	if expand {
		path = pathUtils.ExpandUserAndEnv(path)
	}
	existing, err := m.GetFileAttributes(path, false)
	if err != nil {
		return false, nil
	}
	attrMod := '='
	if strings.HasPrefix(attributes, "-") || strings.HasPrefix(attributes, "+") {
		attrMod = rune(attributes[0])
		attributes = attributes[1:]
	}

	if existing.Flags == attributes && attrMod == '-' {
		return false, nil
	}

	attrCmd, err := m.GetBinPath("chattr", nil, false)
	if err != nil {
		return false, err
	}
	if attrCmd == "" {
		return false, nil
	}
	attrArg := string(attrMod) + attributes

	if diff != nil {
		if _, ok := diff.Before.(map[string]any); !ok {
			diff.Before = map[string]any{}
		}
		if _, ok := diff.After.(map[string]any); !ok {
			diff.After = map[string]any{}
		}
		diff.Before.(map[string]any)["attributes"] = existing.Flags
		diff.After.(map[string]any)["attributes"] = attrArg
	}

	ret, err := m.RunCommand([]string{attrCmd, attrArg}, RunCommandDefaultKwargs())
	if err != nil {
		return false, fmt.Errorf("chattr failed: %v", err)
	}
	if ret.Rc != 0 || len(ret.Stderr) != 0 {
		return false, fmt.Errorf("chattr failed: %v", fmt.Errorf("error while setting attributes: %s%s", string(ret.Stdout), string(ret.Stderr)))
	}

	return true, nil
}

func (m *GosibleModule[P]) getCmdHelper(args []string, env []string, kwargs *RunCommandKwargs) (*exec.Cmd, error) {
	com := &exec.Cmd{
		Path:       args[0],
		Args:       args,
		Env:        env,
		ExtraFiles: kwargs.PassFds,
	}

	if kwargs.Cwd != "" {
		ok, err := pathUtils.IsDir(kwargs.Cwd)
		if err != nil {
			return nil, err
		}
		if ok {
			com.Dir = kwargs.Cwd
		} else {
			return nil, fmt.Errorf("provided cwd is not a valid directory: %s", com.Dir)
		}
	}

	return com, nil
}

func (m *GosibleModule[P]) getCmd(rawArgs interface{}, kwargs *RunCommandKwargs) (*exec.Cmd, error) {
	args, err := m.getArgs(rawArgs, kwargs)
	if err != nil {
		return nil, err
	}
	env := m.getEnvStringSlice(kwargs)
	return m.getCmdHelper(args, env, kwargs)
}

func (m *GosibleModule[P]) getPromptRe(kwargs *RunCommandKwargs) (promptRe *regexp.Regexp, err error) {
	if kwargs.PromptRegex != "" {
		promptRe, err = regexp.Compile("(?m)" + kwargs.PromptRegex) // We need regex to be multiline
	}

	return
}

func (m *GosibleModule[P]) getArgs(rawArgs interface{}, kwargs *RunCommandKwargs) (args []string, err error) {
	var isString bool
	switch rawArgs.(type) {
	case string:
		isString = true
	case []string:
		isString = false
	default:
		return nil, errors.New("argument 'rawArgs' to RunCommand must be slice or string")
	}

	if kwargs.UseUnsafeShell {
		var stringArgs string
		// stringify args for unsafe/direct shell usage
		if isString {
			stringArgs = rawArgs.(string)
		} else {
			sliceArgs := rawArgs.([]string)
			for i, arg := range sliceArgs {
				sliceArgs[i] = shellescape.Quote(arg)
			}
			stringArgs = strings.Join(sliceArgs, " ")
		}
		// not set explicitly, check if set by controller
		if kwargs.Executable != "" {
			args = []string{kwargs.Executable, "-c", stringArgs}
		} else if m.shell != nil && *m.shell != "/bin/sh" {
			args = []string{*m.shell, "-c", stringArgs}
		} else {
			// Default to system shell.
			args = []string{"/bin/sh", "-c", stringArgs}
		}
	} else {
		if isString {
			rawArgs, err = shlex.Split(rawArgs.(string))
			if err != nil {
				return
			}
		}
		args = rawArgs.([]string)
		if kwargs.ExpandUserAndVars {
			for i, arg := range args {
				args[i] = pathUtils.ExpandUserAndEnv(arg)
			}
		}
	}

	return
}

func (m *GosibleModule[P]) getEnv(kwargs *RunCommandKwargs) map[string]string {
	env := getEnvMap()
	for key, val := range m.RunCommandEnvironUpdate {
		env[key] = val
	}
	for key, val := range kwargs.EnvironUpdate {
		env[key] = val
	}
	if kwargs.PathPrefix != "" {
		if path, ok := env["PATH"]; ok {
			env["PATH"] = fmt.Sprintf("%s:%s", kwargs.PathPrefix, path)
		} else {
			env["PATH"] = kwargs.PathPrefix
		}
	}
	// Ansible also cleans python paths, but I don't think we need to as we don't use ansiballz.

	return env
}

func (m *GosibleModule[P]) getEnvStringSlice(kwargs *RunCommandKwargs) []string {
	envMap := m.getEnv(kwargs)
	env := make([]string, 0, len(envMap))
	for key, val := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}
	return env
}

func (m *GosibleModule[P]) promptError(stdout []byte) (*RunCommandResult, error) {
	const msg = "a prompt was encountered while running a command, but no input data was specified"
	return &RunCommandResult{
		Rc:     InternalErrorRc,
		Stdout: stdout,
		Stderr: []byte(msg),
	}, errors.New(msg)
}

func (m *GosibleModule[P]) waitError(err error, stdout []byte, stderr bytes.Buffer, kwargs *RunCommandKwargs) (*RunCommandResult, error) {
	if exitError, ok := err.(*exec.ExitError); ok {
		rc := exitError.ProcessState.ExitCode()
		res := &RunCommandResult{
			Rc:     rc,
			Stdout: stdout,
			Stderr: stderr.Bytes(),
		}
		if kwargs.CheckRc {
			return res, fmt.Errorf("error exit code: %d", rc)
		}
		return res, nil

	}
	return nil, err
}

func (m *GosibleModule[P]) readOutputError(err error, stdout []byte, cmd *exec.Cmd) (*RunCommandResult, error) {
	if err = cmd.Process.Kill(); err != nil {
		return nil, err
	}
	if stdout == nil {
		return nil, err
	}
	return m.promptError(stdout)
}

// PreservedCopy copies a file with preserved ownership, permissions and context.
func (m *GosibleModule[P]) PreservedCopy(src string, dest string) error {
	if err := pathUtils.Copy(src, dest); err != nil {
		return err
	}
	if err := pathUtils.CopyStat(src, dest); err != nil {
		return err
	}
	return m.copyAttributes(src, dest)
}

func (m *GosibleModule[P]) copyAttributes(src, dest string) error {
	currAttribs, err := m.GetFileAttributes(src, false)
	if err != nil {
		return err
	}
	_, err = m.SetAttributesIfDifferent(dest, currAttribs.Flags, nil, true)
	return err
}

func (m *GosibleModule[P]) GetFileAttributes(path string, includeVersion bool) (*Attributes, error) {
	var output Attributes
	attrCmd, err := m.GetBinPath("lsattr", nil, false)
	if err != nil {
		return nil, err
	}
	if attrCmd == "" {
		return &output, nil
	}
	flags := "-d"
	if includeVersion {
		flags = "-vd"
	}

	ret, err := m.RunCommand([]string{attrCmd, flags, path}, RunCommandDefaultKwargs())
	if err != nil || ret.Rc != 0 {
		return &output, nil
	}
	res := strings.Fields(string(ret.Stdout))
	attrFlagsIdx := 0
	if includeVersion {
		attrFlagsIdx = 1
		output.Version = strings.TrimSpace(res[0])
	}
	output.Flags = strings.TrimSpace(strings.ReplaceAll(res[attrFlagsIdx], "-", ""))
	output.Attributes = file.FormatAttributes(output.Flags)

	return &output, nil
}

func (m *GosibleModule[P]) Cleanup(tempFile string) error {
	err := os.Remove(tempFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

type Attributes struct {
	Version    string
	Flags      string
	Attributes []string
}

func getEnvMap() map[string]string {
	environ := make(map[string]string)
	for _, env := range os.Environ() {
		entry := strings.Split(env, "=")
		environ[entry[0]] = entry[1]
	}
	return environ
}
