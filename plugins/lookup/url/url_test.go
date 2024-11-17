package url

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/jmolinski/gosible-templates/exec"
	"github.com/scylladb/gosible/testUtils"
	"math/rand"
	"net/http"
	"reflect"
	"testing"
)

var randBytes = make([]byte, 42)

const namespace = "lookup/url"
const randomName = "random"

func TestGet(t *testing.T) {
	https := false
	f := func(addr string) {
		checkRandom(t, addr, https)
	}

	testUtils.RunTestHttp(t, f, false)
	https = true
	testUtils.RunTestHttp(t, f, https)
}

func checkRandom(t *testing.T, addr string, https bool) {
	url := testUtils.GetUrl(https, addr, namespace, randomName)
	res := Run(&exec.VarArgs{Args: []*exec.Value{{Val: reflect.ValueOf(url)}}})
	content := getContent(res)
	if !reflect.DeepEqual(content, randBytes) {
		t.Fatal("Wrong data", spew.Sdump(content), "expected", spew.Sdump(randBytes))
	}
}

func getContent(res *exec.Value) []byte {
	if res.IsNil() {
		return nil
	}
	resolved := getResolvedValue(res)

	switch resolved.Kind() {
	case reflect.Slice:
		return []byte(resolved.Index(0).String())
	default:
		return nil
	}
}

func init() {
	testUtils.RegisterHttpHandler(namespace, randomName, randomHandler)
}

func randomHandler(writer http.ResponseWriter, request *http.Request) {
	rand.Read(randBytes)
	_, err := writer.Write(randBytes)
	if err != nil {
		panic(err)
	}
}
