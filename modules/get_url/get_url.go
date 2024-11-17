package getUrl

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/scylladb/gosible/module_utils/gosibleModule"
	"github.com/scylladb/gosible/module_utils/urls"
	"github.com/scylladb/gosible/modules"
	pathUtils "github.com/scylladb/gosible/utils/path"
	"github.com/scylladb/gosible/utils/slices"
	"github.com/scylladb/gosible/utils/types"
	"golang.org/x/sys/unix"
	"io"
	"net/http"
	netUrl "net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Module struct {
	*gosibleModule.GosibleModule[*Params]
	force       bool
	lastModTime *time.Time
}

var _ modules.Module = &Module{}

func New() *Module {
	return &Module{GosibleModule: gosibleModule.New[*Params](&Params{
		UrlCommonParams: urls.NewParams(),
	})}
}

func (m *Module) Name() string {
	return "get_url"
}

// Se stands for SELinux file context.
type Se struct {
	Level string `mapstructure:"selevel"`
	Role  string `mapstructure:"serole"`
	Type  string `mapstructure:"setype"`
	User  string `mapstructure:"seuser"`
}

// Params description for field can be found here: https://docs.ansible.com/ansible/latest/collections/ansible/builtin/get_url_module.html#parameters
type Params struct {
	Backup                bool        `mapstructure:"backup"`
	Checksum              string      `mapstructure:"checksum"`
	Dest                  string      `mapstructure:"dest"`
	TmpDest               string      `mapstructure:"tmp_dest"`
	Timeout               int         `mapstructure:"timeout"`
	UnsafeWrites          bool        `mapstructure:"unsafe_writes"`
	Headers               http.Header `mapstructure:"headers"`
	UnredirectedHeaders   []string    `mapstructure:"unredirected_headers"`
	*urls.UrlCommonParams `mapstructure:",squash"`
}

// Return struct for description check https://docs.ansible.com/ansible/latest/collections/ansible/builtin/get_url_module.html#return-values
type Return struct {
	BackupFile   string
	ChecksumDest []byte
	ChecksumSrc  []byte
	Dest         string
	Elapsed      int
	Gid          int
	Group        string
	Md5Sum       []byte
	Mode         string
	Msg          string
	Owner        string
	Response     string
	Secontext    string
	Size         int64
	Src          string
	State        string
	StatusCode   int
	Uid          int
	Url          string
}

type checksum struct {
	checksum  []byte
	algorithm string
}

func (m *Module) Run(ctx *modules.RunContext, vars types.Vars) *modules.Return {
	vars = prepareVars(vars)
	if err := m.ParseParams(ctx, vars); err != nil {
		return m.MarkReturnFailed(err)
	}
	m.addDefaultReturn()

	chksum := &checksum{}
	var err error
	if m.Params.Checksum != "" {
		// checksum specified, parse for algorithm and checksum
		chksum, err = m.handleCheckSum()
		if err != nil {
			return m.MarkReturnFailed(err)
		}
	}
	dest := m.Params.Dest
	exists, err := pathUtils.Exists(dest)
	if err != nil {
		return m.MarkReturnFailed(err)
	}
	var destIsDir bool
	if exists {
		destIsDir, err = pathUtils.IsDir(dest)
		if err != nil {
			return m.MarkReturnFailed(err)
		}
	} else {
		destIsDir = false
	}
	if !destIsDir && exists {
		canExit, err := m.prepareDownloadToFile(chksum, vars)
		if err != nil {
			return m.MarkReturnFailed(err)
		}
		if canExit {
			if m.GetReturn().Changed {
				return m.UpdateReturn(&modules.Return{Msg: "file already exists but file attributes changed"})
			}
			return m.UpdateReturn(&modules.Return{Msg: "file already exists"})
		}
	}

	tmpSrc, rsp, err := m.DownloadToTmpSrc()
	if err != nil {
		return m.MarkReturnFailed(err)
	}
	defer os.Remove(tmpSrc)
	m.UpdateReturn(parseReturnFromResponse(rsp))
	// Now the request has completed, we can finally generate the final destination file name from the info dict.
	if destIsDir {
		filename := extractFileNameFromHeaders(rsp.Header)
		if filename == "" {
			// Fall back to extracting the filename from the URL.
			// Pluck the URL from the info, since a redirect could have changed it.
			filename = urlFilename(m.Params.Url)
		}
		dest = filepath.Join(dest, filename)
		m.GetReturn().ModuleSpecificReturn.(*Return).Dest = dest
	}

	if err = checkTmpSrc(tmpSrc); err != nil {
		return m.MarkReturnFailed(err)
	}
	var ret Return
	ret.ChecksumSrc, err = m.Sha1(tmpSrc)
	if err != nil {
		return m.MarkReturnFailed(err)
	}
	ret.ChecksumDest, err = m.checkNoDestFile(tmpSrc)
	if err != nil {
		return m.MarkReturnFailed(err)
	}
	ret.BackupFile, err = m.handleBackup(ret.ChecksumSrc, ret.ChecksumDest, tmpSrc)
	if err != nil {
		return m.MarkReturnFailed(err)
	}
	if err = m.checkChecksum(chksum); err != nil {
		return m.MarkReturnFailed(err)
	}
	// allow file attribute changes
	fileParams, err := m.LoadFileCommonParams(vars, m.Params.Dest)
	if err != nil {
		return m.MarkReturnFailed(err)
	}
	changed, err := m.SetFsAttributesIfDifferent(fileParams, nil, true)
	if err != nil {
		return m.MarkReturnFailed(err)
	}
	if changed {
		m.UpdateReturn(&modules.Return{Changed: true})
	}
	// Backwards compat only.  We'll return nil on FIPS enabled systems
	ret.Md5Sum, _ = m.Md5(m.Params.Dest)

	if err = m.Close(); err != nil {
		return m.MarkReturnFailed(err)
	}
	return m.UpdateReturn(&modules.Return{ModuleSpecificReturn: &ret})
}

var contDispRe = regexp.MustCompile(`attachment; ?filename="?([^"]+)`)

// extractFileNameFromHeaders Extracts a filename from the given dict of HTTP headers.
// Looks for the content-disposition header and applies a regex.
func extractFileNameFromHeaders(header http.Header) (res string) {
	contentDisposition := header.Get("content-disposition")
	if contentDisposition != "" {
		if contDispRe.MatchString(contentDisposition) {
			res = contDispRe.FindStringSubmatch(contentDisposition)[1]
			res = filepath.Base(res)
		}
	}

	return
}

func isUrl(s string) bool {
	u, err := netUrl.Parse(s)
	if err != nil {
		return false
	}
	return slices.Contains([]string{"http", "https", "ftp", "file"}, u.Scheme)
}

func urlFilename(url string) string {
	u, err := netUrl.Parse(url)
	if err != nil {
		return "index.html"
	}
	filename := filepath.Base(u.Path)
	if filename == "" {
		return "index.html"
	}
	return filename
}

func checkTmpSrc(src string) error {
	exists, err := pathUtils.Exists(src)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("request failed")
	}
	access, err := pathUtils.Access(src, unix.R_OK)
	if err != nil {
		return err
	}
	if !access {
		return fmt.Errorf("source %s is not readable", src)
	}
	return nil
}

func (m *Module) addDefaultReturn() {
	m.UpdateReturn(&modules.Return{
		ModuleSpecificReturn: &Return{
			Dest: m.Params.Dest,
			Url:  m.Params.Url,
		},
	})
}

func prepareVars(vars types.Vars) types.Vars {
	vars = urls.PrepareVarsForUrlCommonParams(vars)
	if _, ok := vars["url_username"].(string); !ok {
		vars["url_username"] = vars["username"]
	}
	if _, ok := vars["url_password"].(string); !ok {
		vars["url_password"] = vars["password"]
	}
	return vars
}

var chksumRe = regexp.MustCompile(`\W+`)

func (m *Module) handleCheckSum() (*checksum, error) {
	algChsum := strings.SplitN(m.Params.Checksum, ":", 2)
	if len(algChsum) != 2 {
		return nil, errors.New("the checksum parameter has to be in format <algorithm>:<checksum>")
	}
	sum := algChsum[1]
	ret := checksum{
		algorithm: algChsum[0],
	}

	if isUrl(sum) {
		var err error
		ret.checksum, err = m.getChecksumFromUrl(sum)
		if err != nil {
			return nil, err
		}
	}
	// Remove any non-alphanumeric characters, including the infamous Unicode zero-width space
	sum = strings.ToLower(chksumRe.ReplaceAllString(sum, ""))
	// Ensure the checksum portion is a hexdigest
	if _, err := strconv.ParseInt(sum, 16, 64); err != nil {
		return nil, errors.New("the checksum format is invalid")
	}
	ret.checksum = []byte(sum)
	return &ret, nil
}

func (m *Module) prepareDownloadToFile(chksum *checksum, vars types.Vars) (bool, error) {
	// If the download is not forced and there is a checksum, allow checksum match to skip the download.
	if !m.force && len(chksum.checksum) != 0 {
		dstChssum, err := m.DigestFromFile(m.Params.Dest, chksum.algorithm)
		if err != nil {
			return false, err
		}
		// Not forcing redownload, unless checksum does not match
		if !bytes.Equal(dstChssum, chksum.checksum) {
			m.force = true
			// Not forcing redownload, unless checksum does not match allow file attribute changes
			fileParams, err := m.LoadFileCommonParams(vars, m.Params.Dest)
			if err != nil {
				return false, err
			}
			changed, err := m.SetFsAttributesIfDifferent(fileParams, nil, true)
			if err != nil {
				return false, err
			}
			m.UpdateReturn(&modules.Return{Changed: changed})
			return true, nil
		}
	}

	state, err := os.Stat(m.Params.Dest)
	if err != nil {
		return false, err
	}
	modTime := state.ModTime()
	m.lastModTime = &modTime

	return false, nil
}

func (m *Module) DownloadToTmpSrc() (string, *urls.Response, error) {
	startTime := time.Now()
	tmpSrc, rsp, err := m.urlGet(m.Params.Url, "GET")
	if err != nil {
		return "", nil, err
	}
	ret := m.GetReturn().ModuleSpecificReturn.(*Return)
	ret.Elapsed = int(time.Now().Sub(startTime).Seconds())
	ret.Src = tmpSrc
	return tmpSrc, rsp, nil
}

// Download data from the url and store in a temporary file.
func (m *Module) urlGet(url, method string) (string, *urls.Response, error) {
	start := time.Now()
	rsp, err := urls.NewRequest(nil).Open(url, method, m.getRequestData())
	if err != nil {
		return "", nil, err
	}
	defer rsp.Close_()
	elapsed := int(time.Now().Sub(start).Seconds())
	if rsp.StatusCode != http.StatusOK &&
		!strings.HasPrefix(url, "file:/") &&
		!(strings.HasPrefix(url, "ftp:/") && strings.HasPrefix(rsp.Msg(), "OK")) {
		m.UpdateReturn(&modules.Return{ModuleSpecificReturn: Return{
			Elapsed:    elapsed,
			StatusCode: rsp.StatusCode,
			Response:   rsp.Msg(),
			Dest:       m.Params.Dest,
			Url:        url,
		}})
		return "", nil, errors.New("request failed")
	}
	m.UpdateReturn(&modules.Return{ModuleSpecificReturn: Return{Elapsed: elapsed}})
	tmpDest := m.Params.TmpDest
	if tmpDest == "" {
		tmpDest, err = m.TmpDir()
		if err != nil {
			return "", nil, err
		}
	} else {
		// tmpDest should be an existing dir
		isDir, err := pathUtils.IsDir(tmpDest)
		if err != nil {
			if os.IsNotExist(err) {
				return "", nil, fmt.Errorf("%s is a file but should be a directory", tmpDest)
			}
			return "", nil, err
		}
		if !isDir {
			return "", nil, fmt.Errorf("%s directory does not exist", tmpDest)
		}
	}

	tmpFile, err := os.CreateTemp(tmpDest, "")
	if err != nil {
		return "", nil, err
	}
	tmpName := tmpFile.Name()

	if _, err = io.Copy(tmpFile, rsp.Body); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		return "", nil, fmt.Errorf("failed to create temporary content file: %v", err)
	}
	if err = tmpFile.Close(); err != nil {
		return "", nil, err
	}

	return tmpName, rsp, nil
}

func (m *Module) getRequestData() *urls.RequestData {
	req := urls.NewRequestData()
	return req.Merge(&urls.RequestData{
		UrlCommonParams:     m.Params.UrlCommonParams,
		Headers:             m.Params.Headers,
		UnredirectedHeaders: m.Params.UnredirectedHeaders,
		Timeout:             m.Params.Timeout,
		LastModTime:         m.lastModTime,
	})
}

func (m *Module) getChecksumFromUrl(checksumUrl string) ([]byte, error) {
	// download checksum file to checksumTmpsrc
	checksumTmpsrc, rsp, err := m.urlGet(checksumUrl, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer rsp.Close_()
	contents, err := os.ReadFile(checksumTmpsrc)
	if err != nil {
		return nil, err
	}
	if err = os.Remove(checksumTmpsrc); err != nil {
		return nil, err
	}
	lines := bytes.Split(contents, []byte("\n"))
	checksumMap := make(map[string][]byte)
	for _, line := range lines {
		// Split by one whitespace to keep the leading type char ' ' (whitespace) for text and '*' for binary
		parts := bytes.SplitN(line, []byte(" "), 2)
		if len(parts) == 2 {
			name := string(parts[0])
			// Remove the leading type char, we expect
			if strings.HasPrefix(name, " ") || strings.HasPrefix(name, "*") {
				parts[1] = parts[1][1:]
			}
			// Append checksum and path without potential leading './'
			if _, ok := checksumMap[name]; !ok {
				checksumMap[name] = bytes.TrimLeft(parts[1], "./")
			}
		}
	}

	fileName := urlFilename(m.Params.Url)
	// Look through each line in the checksum file for a hash corresponding to
	// the filename in the url, returning the first hash that is found.
	if v, ok := checksumMap[fileName]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("unable to find a checksum for file '%s' in '%s'", fileName, checksumUrl)
}

func (m *Module) checkNoDestFile(tmpsrc string) ([]byte, error) {
	exists, err := pathUtils.Exists(m.Params.Dest)
	if err != nil {
		return nil, err
	}
	if exists {
		// raise an error if copy has no permission on dest
		access, err := pathUtils.Access(m.Params.Dest, unix.W_OK)
		if err != nil {
			return nil, err
		}
		if !access {
			return nil, fmt.Errorf("destination %s is not writable", m.Params.Dest)
		}
		access, err = pathUtils.Access(m.Params.Dest, unix.R_OK)
		if err != nil {
			return nil, err
		}
		if !access {
			return nil, fmt.Errorf("destination %s is not readable", m.Params.Dest)
		}
		return m.Sha1(m.Params.Dest)
	}
	dir := filepath.Dir(m.Params.Dest)
	access, err := pathUtils.Access(dir, unix.W_OK)
	if err != nil {
		if err == syscall.ENOENT {
			err = fmt.Errorf("destination %s does not exist", dir)
		}
		return nil, err
	}
	if !access {
		return nil, fmt.Errorf("destination %s is not writable", dir)
	}
	return nil, err
}

func (m *Module) handleBackup(chksumSrc, chksumDst []byte, tmpsrc string) (string, error) {
	var backupFile string
	if bytes.Equal(chksumSrc, chksumDst) {
		m.UpdateReturn(&modules.Return{Changed: false})
		return backupFile, nil
	}
	wrapErr := func(err error) error { return fmt.Errorf("failed to copy %s to %s: %v", tmpsrc, m.Params.Dest, err) }

	if m.Params.Backup {
		exists, err := pathUtils.Exists(m.Params.Dest)
		if err != nil {
			return backupFile, err
		}
		if exists {
			backupFile, err = m.BackupLocal(m.Params.Dest)
			if err != nil {
				return "", wrapErr(err)
			}
		}
	}
	if err := m.AtomicMove(tmpsrc, m.Params.Dest, m.Params.UnsafeWrites); err != nil {
		return "", wrapErr(err)
	}
	m.UpdateReturn(&modules.Return{Changed: true})

	return backupFile, nil
}

func (m *Module) checkChecksum(chksum *checksum) error {
	if len(chksum.checksum) == 0 {
		return nil
	}
	dstChesum, err := m.DigestFromFile(m.Params.Dest, chksum.algorithm)
	if err != nil {
		return err
	}
	if bytes.Equal(dstChesum, chksum.checksum) {
		return nil
	}
	return fmt.Errorf("the checksum for %s did not match %s; it was %s", m.Params.Dest, chksum.checksum, dstChesum)
}

func parseReturnFromResponse(resp *urls.Response) *modules.Return {
	failed := resp.StatusCode != http.StatusOK
	var err string
	if failed {
		err = "download failed"
	}

	return &modules.Return{
		Changed: !failed,
		Failed:  failed,
		ModuleSpecificReturn: &Return{
			StatusCode: resp.StatusCode,
			Url:        resp.Request.RequestURI,
		},
		InternalReturn: &modules.InternalReturn{
			Exception: err,
		},
	}
}

func (p *Params) Validate() error {
	if err := p.UrlCommonParams.Validate(); err != nil {
		return err
	}
	if p.Url == "" {
		return errors.New("required `url` argument doesn't exist")
	}
	if p.Dest == "" {
		return errors.New("required `dest` argument doesn't exist")
	}
	return nil
}
