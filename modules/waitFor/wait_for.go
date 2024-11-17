package waitFor

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/scylladb/gosible/module_utils/gosibleModule"
	"github.com/scylladb/gosible/modules"
	pathUtils "github.com/scylladb/gosible/utils/path"
	"github.com/scylladb/gosible/utils/slices"
	"github.com/scylladb/gosible/utils/types"
	netPs "github.com/shirou/gopsutil/v3/net"
	"math"
	"net"
	"os"
	"regexp"
	"syscall"
	"time"
)

type Return struct {
	Elapsed        int
	MatchGroupDict map[string]string
	MatchGroups    []string
	State          string
	Port           uint32
	SearchRegex    string
	Path           string
}

type Params struct {
	Host                   string   `mapstructure:"host"`
	Timeout                int      `mapstructure:"timeout"`
	ConnectTimeout         int      `mapstructure:"connect_timeout"`
	Delay                  int      `mapstructure:"delay"`
	Port                   uint32   `mapstructure:"port"`
	ActiveConnectionStates []string `mapstructure:"active_connection_states"`
	Path                   string   `mapstructure:"path"`
	SearchRegex            string   `mapstructure:"search_regex"`
	State                  string   `mapstructure:"state"`
	ExcludeHosts           []string `mapstructure:"exclude_hosts"`
	Sleep                  int      `mapstructure:"sleep"`
	Msg                    string   `mapstructure:"msg"`
}

func (p *Params) Validate() error {
	opts := []string{"absent", "drained", "present", "started", "stopped"}
	if slices.Contains(opts, p.State) {
		return fmt.Errorf("invalid option for state: %s, expected one of %v", p.State, opts)
	}
	if p.Path != "" {
		if p.Port != 0 {
			return errors.New("port and path parameter can not both be passed to wait_for")
		}
		if p.State == "stopped" {
			return errors.New("state=stopped should only be used for checking a port in the wait_for module")
		}
		if p.State == "drained" {
			return errors.New("state=drained should only be used for checking a port in the wait_for module")
		}
	}
	if len(p.ExcludeHosts) != 0 && p.State != "drained" {
		return errors.New("exclude_hosts should only be with state=drained")
	}
	return nil
}

type Module struct {
	*gosibleModule.GosibleModule[*Params]
}

var _ modules.Module = &Module{}

func New() *Module {
	return &Module{GosibleModule: gosibleModule.New(&Params{
		Host:                   "127.0.0.1",
		Timeout:                300,
		ConnectTimeout:         5,
		ActiveConnectionStates: []string{"ESTABLISHED", "FIN_WAIT1", "FIN_WAIT2", "SYN_RECV", "SYN_SENT", "TIME_WAIT"},
		State:                  "started",
		Sleep:                  1,
	})}
}

func (m *Module) Name() string {
	return "wait_for"
}

type tcpConnectionInfo struct {
	*gosibleModule.GosibleModule[*Params]
	ips        []net.IP
	excludeIps []net.IP
}

func newTcpConnectionInfo(m *gosibleModule.GosibleModule[*Params]) (*tcpConnectionInfo, error) {
	ips, err := convertHostToIp(m.Params.Host)
	if err != nil {
		return nil, err
	}
	excludeIps, err := getExcludeIps(m.Params.ExcludeHosts)
	if err != nil {
		return nil, err
	}
	return &tcpConnectionInfo{
		GosibleModule: m,
		ips:           ips,
		excludeIps:    excludeIps,
	}, nil
}

func getExcludeIps(hosts []string) ([]net.IP, error) {
	excludeIps := make([]net.IP, 0, len(hosts))
	for _, host := range hosts {
		ips, err := convertHostToIp(host)
		if err != nil {
			return nil, err
		}
		excludeIps = append(excludeIps, ips...)
	}
	return excludeIps, nil
}

// convertHostToIp performs forward DNS resolution on host, IP will give the same IP.
func convertHostToIp(host string) ([]net.IP, error) {
	lookUpIps, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, 0, len(lookUpIps))
	for _, ip := range lookUpIps {
		ips = append(ips, ip)
		if ip.To4() != nil {
			ips = append(ips, net.IP(fmt.Sprintf("%s:%s", string(ipv4MappedIpv6AddressPrefix), string(ip))))
		}
	}
	return ips, nil
}

var matchAllIps = map[uint32]net.IP{
	syscall.AF_INET:  net.ParseIP("0.0.0.0"),
	syscall.AF_INET6: net.ParseIP("::"),
}

var ipv4MappedIpv6AddressPrefix = []byte("::ffff")
var ipv4MappedIpv6AddressMatchAll = []byte("::ffff:0.0.0.0")

func (t *tcpConnectionInfo) getActiveConnectionsCount() (int, error) {
	conns, err := netPs.ConnectionsWithoutUids("inet")
	if err != nil {
		return 0, err
	}
	activeConnsCount := 0
	for _, conn := range conns {
		laddr := conn.Laddr
		raddr := conn.Raddr
		if !slices.Contains(t.Params.ActiveConnectionStates, conn.Status) ||
			laddr.Port != t.Params.Port ||
			inIpList(t.excludeIps, net.IP(raddr.IP)) {
			continue
		}
		if t.isInIps(conn.Family, net.IP(laddr.IP)) {
			activeConnsCount++
		}
	}

	return activeConnsCount, nil
}

func (t *tcpConnectionInfo) isInIps(family uint32, ip net.IP) bool {
	return inIpList(t.ips, ip) ||
		inIpList(t.ips, matchAllIps[family]) ||
		(bytes.HasPrefix(ip, ipv4MappedIpv6AddressPrefix) && inIpList(t.ips, ipv4MappedIpv6AddressMatchAll))
}

func inIpList(ips []net.IP, ip net.IP) bool {
	for _, _ip := range ips {
		if bytes.Equal(_ip, ip) {
			return true
		}
	}
	return false
}

var connectionStateId = map[string]string{
	"ESTABLISHED": "01",
	"SYN_SENT":    "02",
	"SYN_RECV":    "03",
	"FIN_WAIT1":   "04",
	"FIN_WAIT2":   "05",
	"TIME_WAIT":   "06",
}

func (m *Module) Run(ctx *modules.RunContext, vars types.Vars) *modules.Return {
	m.UpdateReturn(&modules.Return{ModuleSpecificReturn: &Return{Elapsed: 0}}) // Ansible always returns this value in case of an error.
	err := m.ParseParams(ctx, vars)
	if err != nil {
		return m.MarkReturnFailed(err)
	}

	startTime := time.Now()
	if m.Params.Delay != 0 {
		time.Sleep(time.Duration(m.Params.Delay) * time.Second)
	}

	for _, connState := range m.Params.ActiveConnectionStates {
		if _, ok := connectionStateId[connState]; !ok {
			return m.MarkReturnFailed(fmt.Errorf("unknown active_connection_state (%s) defined", connState))
		}
	}

	var matchGroups []string
	var matchGroupsDict map[string]string
	if m.Params.Port == 0 && m.Params.Path == "" && m.Params.State != "drained" {
		time.Sleep(time.Duration(m.Params.Timeout) * time.Second)
	} else {
		switch m.Params.State {
		case "absent", "stopped":
			err = m.handleAbsentStoppedState(startTime)
		case "started", "present":
			matchGroups, matchGroupsDict, err = m.handleStartedPresentState(startTime)
		case "drained":
			err = m.handleDrainedState(startTime)
		}
		if err != nil {
			return m.MarkReturnFailed(err)
		}
	}

	elapsedTime := time.Now().Sub(startTime)

	return m.UpdateReturn(&modules.Return{
		ModuleSpecificReturn: &Return{
			State:          m.Params.State,
			Port:           m.Params.Port,
			SearchRegex:    m.Params.SearchRegex,
			MatchGroups:    matchGroups,
			MatchGroupDict: matchGroupsDict,
			Path:           m.Params.Path,
			Elapsed:        int(elapsedTime.Seconds()),
		},
	})
}

func (m *Module) handleDrainedState(startTime time.Time) error {
	// wait until all active connections are gone
	end := startTime.Add(time.Duration(m.Params.Timeout) * time.Second)
	tci, err := newTcpConnectionInfo(m.GosibleModule)
	if err != nil {
		return err
	}
	for time.Now().Before(end) {
		connsCount, err := tci.getActiveConnectionsCount()
		if err != nil {
			return err
		}
		if connsCount == 0 {
			return nil
		}
		// Conditions not yet met, wait and try again
		time.Sleep(time.Duration(m.Params.Delay) * time.Second)
	}
	elapsedTime := time.Now().Sub(startTime)
	m.UpdateReturn(&modules.Return{
		ModuleSpecificReturn: &Return{
			Elapsed: int(elapsedTime.Seconds()),
		},
	})
	errorMsg := m.Params.Msg
	if errorMsg == "" {
		errorMsg = fmt.Sprintf("Timeout when waiting for %s:%s to drain", m.Params.Host, m.Params.Host)
	}
	return fmt.Errorf(errorMsg)
}

func (m *Module) handleStartedPresentState(startTime time.Time) ([]string, map[string]string, error) {
	// first wait for start condition
	end := startTime.Add(time.Duration(m.Params.Timeout) * time.Second)
	var searchRe *regexp.Regexp
	var err error
	if m.Params.SearchRegex != "" {
		searchRe, err = regexp.Compile(m.Params.SearchRegex)
		if err != nil {
			return nil, nil, err
		}
	}
	for time.Now().Before(end) {
		if m.Params.Path != "" {
			found, groups, groupDict, err := m.handleStartedPresentStatePath(searchRe)
			if err != nil {
				return nil, nil, err
			}
			if found {
				return groups, groupDict, nil
			}
		} else if m.Params.Port != 0 {
			found, err := m.handleStartedPresentStatePort(end, searchRe)
			if err != nil {
				return nil, nil, err
			}
			if found {
				return nil, nil, nil
			}
		}

		// Conditions not yet met, wait and try again
		time.Sleep(time.Duration(m.Params.Sleep) * time.Second)
	}

	elapsedTime := time.Now().Sub(startTime)
	m.UpdateReturn(&modules.Return{
		ModuleSpecificReturn: &Return{
			Elapsed: int(elapsedTime.Seconds()),
		},
	})
	if m.Params.Port != 0 {
		if searchRe == nil {
			return nil, nil, fmt.Errorf("timeout when waiting for %s:%d", m.Params.Host, m.Params.Port)
		} else {
			return nil, nil, fmt.Errorf("timeout when waiting for search string %s in %s:%d", m.Params.SearchRegex, m.Params.Host, m.Params.Port)
		}
	} else if m.Params.Path != "" {
		if searchRe == nil {
			return nil, nil, fmt.Errorf("timeout when waiting for file %s", m.Params.Path)
		} else {
			return nil, nil, fmt.Errorf("timeout when waiting for search string %s in %s", m.Params.SearchRegex, m.Params.Path)
		}
	}
	return nil, nil, nil
}

func isNotConn(err error) bool {
	netErr, ok := err.(net.Error)
	return ok && !netErr.Timeout()
}

func (m *Module) handleAbsentStoppedState(startTime time.Time) error {
	// first wait for the stop condition
	end := startTime.Add(time.Duration(m.Params.Timeout) * time.Second)
	for time.Now().Before(end) {
		if m.Params.Path != "" {
			exists, err := pathUtils.Exists(m.Params.Path)
			if err != nil {
				return err
			}
			if !exists {
				return nil
			}
		} else if m.Params.Port != 0 {
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%s", m.Params.Host, m.Params.Host), time.Duration(m.Params.ConnectTimeout)*time.Second)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					return nil
				} else {
					return err
				}
			}
			_ = conn.Close() // Ignore error.
		}
		// Conditions not yet met, wait and try again
		time.Sleep(time.Duration(m.Params.Sleep) * time.Second)
	}
	elapsedTime := time.Now().Sub(startTime)
	m.UpdateReturn(&modules.Return{
		ModuleSpecificReturn: &Return{
			Elapsed: int(elapsedTime.Seconds()),
		},
	})
	if m.Params.Port != 0 {
		return fmt.Errorf("timeout when waiting for %s:%d to stop", m.Params.Host, m.Params.Port)
	}
	if m.Params.Path != "" {
		return fmt.Errorf("timeout when waiting for %s to be absent", m.Params.Path)
	}
	return nil
}

func (m *Module) handleStartedPresentStatePath(searchRe *regexp.Regexp) (bool, []string, map[string]string, error) {
	exists, err := pathUtils.Exists(m.Params.Path)
	if err != nil {
		return false, nil, nil, err
	}
	if exists {
		// File exists. Are there additional things to check?
		if searchRe == nil {
			// nope, succeed!
			return true, nil, nil, nil
		}
		contents, err := os.ReadFile(m.Params.Path)
		if err != nil {
			return false, nil, nil, err
		}
		if searchRe.Match(contents) {
			groups := searchRe.FindStringSubmatch(string(contents))
			var groupDict map[string]string
			if names := searchRe.SubexpNames(); names != nil {
				groupDict = make(map[string]string)
				for i, name := range names {
					groupDict[name] = groups[i]
				}
			}
			return true, groups, groupDict, nil
		}
	}
	return false, nil, nil, nil
}

func (m *Module) handleStartedPresentStatePort(end time.Time, searchRe *regexp.Regexp) (bool, error) {
	altConnTimeout := math.Ceil(end.Sub(time.Now()).Seconds())
	cTimeout := int(math.Min(altConnTimeout, float64(m.Params.ConnectTimeout)))
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%s", m.Params.Host, m.Params.Host), time.Duration(cTimeout)*time.Second)
	if err == nil {
		// Connection established, success!
		if searchRe == nil {
			if err := conn.SetReadDeadline(end); err != nil {
				return false, err
			}
			var data bytes.Buffer
			_, _ = data.ReadFrom(conn)
			err = conn.Close()
			if isNotConn(err) {
				return false, err
			}
			if searchRe.Match(data.Bytes()) {
				return true, nil
			}
		} else {
			err = conn.Close()
			if isNotConn(err) {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}
