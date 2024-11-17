package url

import (
	"bytes"
	"github.com/jmolinski/gosible-templates/exec"
	"github.com/scylladb/gosible/module_utils/urls"
	"net/http"
	"reflect"
	"strings"
)

const Name = "url"

func Run(va *exec.VarArgs) *exec.Value {
	ret := make([]string, 0, len(va.Args))
	splitLines := va.GetKwarg("split_lines", false).Bool()
	req := getReq(va)
	for _, term := range va.Args {
		rsp, err := req.Get(term.String(), nil)
		if err != nil {
			return exec.AsValue(err)
		}

		s, err := rspToString(rsp)
		if splitLines {
			ret = append(ret, strings.Split(s, "\n")...)
		} else {
			ret = append(ret, s)
		}
	}

	return exec.AsValue(ret)
}

func rspToString(rsp *urls.Response) (string, error) {
	var b bytes.Buffer
	_, err := b.ReadFrom(rsp.Body)
	if err != nil {
		return "", exec.AsValue(err)
	}
	return b.String(), nil
}

func getReq(args *exec.VarArgs) *urls.Request {
	data := urls.NewRequestData()
	data.ValidateCerts = args.GetKwarg("validate_certs", false).Bool()
	data.UseProxy = args.GetKwarg("use_proxy", false).Bool()
	data.UrlUserName = args.GetKwarg("username", "").String()
	data.UrlPassword = args.GetKwarg("password", "").String()
	data.Headers = toHeader(args.GetKwarg("headers", nil))
	data.Force = args.GetKwarg("force", false).Bool()
	data.Timeout = args.GetKwarg("timeout", 0).Integer()
	data.HttpAgent = args.GetKwarg("http_agent", "").String()
	data.ForceBasicAuth = args.GetKwarg("force_basic_auth", false).Bool()
	data.FollowRedirects = args.GetKwarg("follow_redirects", "").String()
	data.UseGssapi = args.GetKwarg("use_gssapi", false).Bool()
	data.UnixSocket = args.GetKwarg("unix_socket", "").String()
	data.CaPath = args.GetKwarg("ca_path", "").String()
	data.UnredirectedHeaders = toStringSlice(args.GetKwarg("unredirected_headers", nil))

	return urls.NewRequest(data)
}

func toStringSlice(v *exec.Value) []string {
	if v.IsNil() {
		return nil
	}
	resolved := getResolvedValue(v)

	switch resolved.Kind() {
	case reflect.Slice:
		l := resolved.Len()
		res := make([]string, 0, l)
		for i := 0; i < l; i++ {
			res = append(res, resolved.Index(i).String())
		}
		return res
	default:
		return nil
	}
}

func getResolvedValue(v *exec.Value) reflect.Value {
	if v.Val.IsValid() && v.Val.Kind() == reflect.Ptr {
		return v.Val.Elem()
	}
	return v.Val
}

func toHeader(v *exec.Value) http.Header {
	if v.IsNil() {
		return nil
	}
	resolved := getResolvedValue(v)
	switch resolved.Kind() {
	case reflect.Map:
		res := make(http.Header)
		iter := resolved.MapRange()
		for iter.Next() {
			res.Add(iter.Key().String(), iter.Value().String())
		}
		return res
	default:
		return nil
	}
}
