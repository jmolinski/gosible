package getUrl

import (
	"errors"
	"github.com/davecgh/go-spew/spew"
	"github.com/scylladb/gosible/modules"
	"github.com/scylladb/gosible/testUtils"
	"github.com/scylladb/gosible/utils/types"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path"
	"reflect"
	"testing"
)

var randBytes = make([]byte, 42)
var tempDir, _ = os.MkdirTemp("", "gosible-get_Url-test-*")

var randDest = path.Join(tempDir, "rand_result")

const namespace = "lookup/url"
const randomName = "random"

func TestGetRealHttps(t *testing.T) {
	url := "https://github.com/"
	dest := path.Join(tempDir, "https_test")
	module := New()
	r := module.Run(&modules.RunContext{}, types.Vars{
		"url":  url,
		"dest": dest,
	})

	if r.Failed {
		t.Error(r.Exception)
	}

	if _, err := os.Stat(dest); errors.Is(err, os.ErrNotExist) {
		t.Error(err)
	}
}

func TestGet(t *testing.T) {
	https := false
	f := func(addr string) {
		checkRandom(t, addr, https)
		checkNonExistingPath(t, addr, https)
	}

	testUtils.RunTestHttp(t, f, false)
	https = true
	testUtils.RunTestHttp(t, f, https)
}

func checkRandom(t *testing.T, addr string, https bool) {
	module := New()
	vars := getRandomVars(https, addr)
	if https {
		vars["validate_certs"] = false
	}
	r := module.Run(&modules.RunContext{}, vars)

	if r.Failed {
		t.Error(r.Exception)
	}
	content, err := ioutil.ReadFile(randDest)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(content, randBytes) {
		spew.Dump(content)
		spew.Dump(randBytes)
		spew.Dump(r)
		t.Fatal("Wrong data")
	}
}

func checkNonExistingPath(t *testing.T, addr string, https bool) {
	module := New()
	vars := getRandomVars(https, addr)
	vars["url"] = vars["url"].(string) + "404"
	r := module.Run(&modules.RunContext{}, vars)
	if !r.Failed {
		t.Fatal("Expecting request to fail")
	}
}

func getRandomVars(https bool, addr string) types.Vars {
	pre := "http"
	if https {
		pre += "s"
	}

	return types.Vars{
		"dest": randDest,
		"url":  testUtils.GetUrl(https, addr, namespace, randomName),
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
