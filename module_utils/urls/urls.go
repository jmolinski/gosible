package urls

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"github.com/hhkbp2/go-strftime"
	"github.com/jdxcode/netrc"
	"github.com/mitchellh/mapstructure"
	"github.com/scylladb/gosible/utils/slices"
	"github.com/scylladb/gosible/utils/types"
	"net/http"
	"net/url"
	"os"
	"time"
)

type UrlCommonParams struct {
	ClientCert     string `mapstructure:"client_cert"`
	ClientKey      string `mapstructure:"client_key"`
	Force          bool   `mapstructure:"force"`
	ForceBasicAuth bool   `mapstructure:"force_basic_auth"`
	HttpAgent      string `mapstructure:"http_agent"`
	Url            string `mapstructure:"url"`
	UrlPassword    string `mapstructure:"url_password"`
	UrlUserName    string `mapstructure:"url_username"`
	UseGssapi      bool   `mapstructure:"use_proxy"`
	UseProxy       bool   `mapstructure:"use_gssapi"`
	ValidateCerts  bool   `mapstructure:"validate_certs"`
}

func PrepareVarsForUrlCommonParams(vars types.Vars) types.Vars {
	return vars
}

type RequestData struct {
	*UrlCommonParams
	Data                []byte         `mapstructure:""`
	LastModTime         *time.Time     `mapstructure:""`
	UnixSocket          string         `mapstructure:""`
	CaPath              string         `mapstructure:""`
	Cookies             http.CookieJar `mapstructure:""`
	Headers             http.Header    `mapstructure:""`
	UnredirectedHeaders []string       `mapstructure:""`
	FollowRedirects     string         `mapstructure:""`
	Timeout             int            `mapstructure:""`
}

func (r *RequestData) Merge(data *RequestData) *RequestData {
	var ret RequestData
	_ = mapstructure.Decode(r, &ret)
	_ = mapstructure.Decode(data, &ret)
	return &ret
}

type Request struct {
	*RequestData
}

type Response http.Response

func (r *Response) Msg() string {
	length := r.Header.Get("Content-Length")
	if length == "" {
		length = "unknown"
	}
	return fmt.Sprintf("OK (%s)", length)
}

func (r *Response) Close_() error {
	return r.Body.Close()
}

func NewRequest(data *RequestData) *Request {
	return &Request{
		RequestData: data,
	}
}

func (r *Request) Open(url, method string, openData *RequestData) (*Response, error) {
	// TODO better cookie handling
	// TODO better handle SSL
	// TODO handle redirects
	// TODO handle proxy
	// TODO handle Gssapi
	// TODO unix socket handling
	// TODO handle header redirection

	data := r.RequestData.Merge(openData)

	client := r.getClient(data)
	err := r.handleAuth(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(data.Data))
	if err != nil {
		return nil, err
	}
	req.Header = getHeader(data)

	rsp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	cast := Response(*rsp)
	return &cast, err
}

var gmt, _ = time.LoadLocation("GMT")

func toIfModifiedSinceDate(time time.Time) string {
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/If-Modified-Since
	if gmt != nil {
		time = time.In(gmt)
	}
	return strftime.Format("%a, %d %b %Y %T GMT", time)
}

func getHeader(data *RequestData) http.Header {
	header := make(http.Header)
	// add the custom agent header, to help prevent issues with sites that block the default urllib agent string
	if data.HttpAgent != "" {
		header.Set("User-agent", data.HttpAgent)
	}
	// Cache control
	if data.Force {
		// Either we directly force a cache refresh
		header.Set("cache-control", "no-cache")
	} else if data.LastModTime != nil {
		// or we do it if the original is more recent than our copy
		header.Set("If-Modified-Since", toIfModifiedSinceDate(*data.LastModTime))
	}

	// user defined headers now, which may override things we've set above
	for k, h := range data.Headers {
		// TODO handle unredirected headers.
		header[k] = slices.Copy(h)
	}

	return header
}

func (r *Request) Get(url string, openData *RequestData) (*Response, error) {
	return r.Open(url, http.MethodGet, openData)
}

func (r *Request) Options(url string, openData *RequestData) (*Response, error) {
	return r.Open(url, http.MethodOptions, openData)
}

func (r *Request) Head(url string, openData *RequestData) (*Response, error) {
	return r.Open(url, http.MethodHead, openData)
}

func (r *Request) Post(url string, openData *RequestData) (*Response, error) {
	return r.Open(url, http.MethodPost, openData)
}

func (r *Request) Patch(url string, openData *RequestData) (*Response, error) {
	return r.Open(url, http.MethodPatch, openData)
}

func (r *Request) Delete(url string, openData *RequestData) (*Response, error) {
	return r.Open(url, http.MethodDelete, openData)
}

func (r *Request) handleAuth(data *RequestData) error {
	parsed, err := url.Parse(data.Url)
	if err != nil {
		return err
	}
	if parsed.Scheme == "ftp" {
		return nil
	}
	username, password := data.UrlUserName, data.UrlPassword
	if data.UseGssapi {
		// TODO implement GSSAPI support
	} else if username != "" {
		if data.ForceBasicAuth {
			data.Headers.Set("Authorization", basicAuthHeader(username, password))
		} else {
			// TODO
		}
	} else {
		parse, err := netrc.Parse(os.Getenv("NETRC"))
		if err == nil {
			if m := parse.Machine(parsed.Hostname()); m != nil {
				username := m.Get("username")
				password := m.Get("password")
				data.Headers.Set("Authorization", basicAuthHeader(username, password))
			}
		}
	}

	return nil
}

func (r *Request) getClient(data *RequestData) http.Client {
	return http.Client{
		Timeout: time.Duration(data.Timeout) * time.Second,
		Jar:     data.Cookies,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !data.ValidateCerts,
			},
		},
	}
}

// basicAuthHeader takes a username and password and returns a string suitable for
// using as value of an Authorization header to do basic auth.
func basicAuthHeader(username string, password string) string {
	data := fmt.Sprintf("%s:%s", username, password)
	enc := base64.StdEncoding.EncodeToString([]byte(data))
	return "Basic " + enc
}

func NewParams() *UrlCommonParams {
	return &UrlCommonParams{
		HttpAgent:     "ansible-httpget",
		UseProxy:      true,
		ValidateCerts: true,
	}
}

func NewRequestData() *RequestData {
	return &RequestData{
		UrlCommonParams: NewParams(),
		Timeout:         10,
		FollowRedirects: "urllib2",
	}
}

func (p *UrlCommonParams) Validate() error {
	return nil
}
