package hostname

import (
	"github.com/scylladb/gosible/module_utils/gosibleModule"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

var testDir string
var networkFile string

func setUp() {
	var err error
	testDir, err = ioutil.TempDir("", "gosible-test-hostname-*")
	if err != nil {
		panic(err)
	}
	networkFile = path.Join(testDir, "network")
}

func teardown() {
	err := os.RemoveAll(testDir)
	if err != nil {
		panic(err)
	}
}

func getInstance(name string) strategy {
	return newBaseStrategy(gosibleModule.New(&Params{Name: name}), &redHatStrategy{networkFile: networkFile})
}

func TestRedhatGetPermanentHostnameMissing(t *testing.T) {
	setUp()
	defer teardown()
	ins := getInstance("")

	hostname, err := ins.getPermanentHostname()
	if err == nil {
		t.Error("expected error")
	}
	if hostname != "" {
		t.Errorf("expected empty hostname got: %s", hostname)
	}
}

func TestRedhatGetPermanentHostnameLineMissing(t *testing.T) {
	setUp()
	defer teardown()
	ins := getInstance("")
	err := os.WriteFile(networkFile, []byte("# some other content"), 0777)
	if err != nil {
		t.Fatal(err)
	}

	hostname, err := ins.getPermanentHostname()
	if err == nil {
		t.Error("expected error")
	}
	if hostname != "" {
		t.Errorf("expected empty hostname got: %s", hostname)
	}
}

func TestRedhatGetPermanentHostnameExisting(t *testing.T) {
	setUp()
	defer teardown()
	ins := getInstance("")

	err := os.WriteFile(networkFile, []byte("some other content\nHOSTNAME=foobar\nmore content\n"), 0777)
	if err != nil {
		t.Fatal(err)
	}

	hostname, err := ins.getPermanentHostname()
	if err != nil {
		t.Error(err)
	}
	if hostname != "foobar" {
		t.Errorf("expected 'foobar' as hostname got '%s'", hostname)
	}
}

func TestRedhatGetPermanentHostnameExistingWhitespace(t *testing.T) {
	setUp()
	defer teardown()
	ins := getInstance("")

	err := os.WriteFile(networkFile, []byte("some other content\n      HOSTNAME=foobar    \nmore content\n"), 0777)
	if err != nil {
		t.Fatal(err)
	}

	hostname, err := ins.getPermanentHostname()
	if err != nil {
		t.Error(err)
	}
	if hostname != "foobar" {
		t.Errorf("expected 'foobar' as hostname got '%s'", hostname)
	}
}

func TestRedhatSetPermanentHostnameMissing(t *testing.T) {
	setUp()
	defer teardown()
	ins := getInstance("")

	err := ins.setPermanentHostname("foobar")
	if err != nil {
		t.Error(err)
	}
	content, err := ioutil.ReadFile(networkFile)
	if err != nil {
		t.Error(err)
	}
	if string(content) != "HOSTNAME=foobar\n" {
		t.Errorf("Unexpected file content, %s", content)
	}
}

func TestRedhatSetPermanentHostnameLineMissing(t *testing.T) {
	setUp()
	defer teardown()
	ins := getInstance("")

	err := os.WriteFile(networkFile, []byte("some other content\n"), 0777)
	if err != nil {
		t.Fatal(err)
	}

	err = ins.setPermanentHostname("foobar")
	if err != nil {
		t.Error(err)
	}
	content, err := ioutil.ReadFile(networkFile)
	if err != nil {
		t.Error(err)
	}
	if string(content) != "some other content\nHOSTNAME=foobar\n" {
		t.Errorf("Unexpected file content, %s", content)
	}
}

func TestRedhatSetPermanentHostnameExisting(t *testing.T) {
	setUp()
	defer teardown()
	ins := getInstance("")

	err := os.WriteFile(networkFile, []byte("some other content\nHOSTNAME=spam\nmore content\n"), 0777)
	if err != nil {
		t.Fatal(err)
	}

	err = ins.setPermanentHostname("foobar")
	if err != nil {
		t.Error(err)
	}
	content, err := ioutil.ReadFile(networkFile)
	if err != nil {
		t.Error(err)
	}
	if string(content) != "some other content\nHOSTNAME=foobar\nmore content\n" {
		t.Errorf("Unexpected file content, %s", content)
	}
}

func TestRedhatSetPermanentHostnameExistingWhitespace(t *testing.T) {
	setUp()
	defer teardown()
	ins := getInstance("")

	err := os.WriteFile(networkFile, []byte("some other content\n     HOSTNAME=spam   \nmore content\n"), 0777)
	if err != nil {
		t.Fatal(err)
	}

	err = ins.setPermanentHostname("foobar")
	if err != nil {
		t.Error(err)
	}
	content, err := ioutil.ReadFile(networkFile)
	if err != nil {
		t.Error(err)
	}
	if string(content) != "some other content\nHOSTNAME=foobar\nmore content\n" {
		t.Errorf("Unexpected file content, %s", content)
	}
}
