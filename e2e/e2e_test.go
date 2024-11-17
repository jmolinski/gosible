package e2e

import (
	"flag"
	"fmt"
	dc "github.com/ory/dockertest/v3/docker"
	d "github.com/scylladb/gosible/e2e/diff"
	"github.com/scylladb/gosible/e2e/env"
	f "github.com/scylladb/gosible/e2e/fixtures"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const containerWorkdir = "/test_ground"

var workingDir = func() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("failed to get working directory: %s", err))
	}
	return wd
}()

var defaultTestEnv = fmt.Sprintf("%s/assets/defaults", workingDir)

var testDir = fmt.Sprintf("%s/logs/%d.e2e", workingDir, time.Now().UnixMilli())

var e2eRun = flag.Bool("e2e", false, "Run e2e tests")
var testRegexString = flag.String("only", "", "Run only tests matching regex")

type testCase struct {
	name      string
	envPath   string
	filesPath string
	logPath   string
}

// Run both ansible and gosible control nodes with the same configuration
// and compare the resulting managed nodes file systems.
func runTestCase(t *testing.T, tc testCase, env *env.Environment) {
	removeImage := func(img *f.BoxImage) {
		if img == nil {
			return
		}
		if err := img.Remove(); err != nil {
			t.Errorf("failed to remove image %s:%s: %s", img.Repository, img.Tag, err)
		}
	}

	t.Logf("Running test case %s", tc.name)
	t.Cleanup(func() {
		t.Logf("Cleaning up test case %s", tc.name)
	})

	var wg sync.WaitGroup
	var ansibleResult, gosibleResult *f.BoxImage
	var ansibleError, gosibleError error

	defer func() {
		removeImage(ansibleResult)
		removeImage(gosibleResult)
	}()

	wg.Add(2)

	go func() {
		t.Logf("Running ansible controller")
		defer wg.Done()
		ansibleResult, ansibleError = runController("ansible", tc, env)
	}()
	go func() {
		t.Logf("Running gosible controller")
		defer wg.Done()
		gosibleResult, gosibleError = runController("gosible", tc, env)
	}()

	wg.Wait()
	if ansibleError != nil {
		t.Fatalf("failed to run ansible controller: %s", ansibleError)
	}
	if gosibleError != nil {
		t.Fatalf("failed to run gosible controller: %s", gosibleError)
	}

	cmp, err := d.NewImageComparer(tc.name, env)
	if err != nil {
		t.Fatalf("Failed to create image comparer: %s", err)
	}
	defer cmp.Close()

	t.Logf("Comparing ansible and gosible results")
	shallowDiff, err := cmp.ShallowDiff(ansibleResult, gosibleResult)
	if err != nil {
		t.Fatalf("%s: %s", tc.name, err)
	}

	passed, err := handleResults(tc, shallowDiff)
	if err != nil {
		t.Fatalf("failed to handle results: %s: %s", tc.name, err)
	}
	if !passed {
		err = cmp.DeepDiff(ansibleResult, gosibleResult, tc.envPath, shallowDiff)
		if err != nil {
			t.Fatalf("failed to diff: %s: %s", tc.name, err)
		}

		t.Fatalf("%s\n", shallowDiff.String())
	}
}

func handleResults(tc testCase, diff *d.ShallowImageDiff) (bool, error) {
	resultFile, err := os.Create(fmt.Sprintf("%s/results.txt", tc.envPath))
	if err != nil {
		return false, fmt.Errorf("failed to create results file: %s", err)
	}
	defer resultFile.Close()

	pass := diff.IsIdentical()
	return pass, writeResult(pass, tc, resultFile, diff)

}

func statusString(pass bool) string {
	if pass {
		return "PASS"
	}
	return "FAIL"
}

func writeResult(pass bool, tc testCase, file *os.File, diff *d.ShallowImageDiff) error {
	content := []string{
		fmt.Sprintf("name: %s", tc.name),
		fmt.Sprintf("status: %s", statusString(pass)),
	}
	if !pass {
		content = append(content, diff.String())
	}
	_, err := file.WriteString(strings.Join(content, "\n"))
	if err != nil {
		return fmt.Errorf("failed to write results: %s", err)
	}
	return nil
}

func runController(controller string, tc testCase, env *env.Environment) (*f.BoxImage, error) {
	if controller != "gosible" && controller != "ansible" {
		return nil, fmt.Errorf("invalid controller: %s", controller)
	}
	repo := fmt.Sprintf("gosible/%s", controller)
	tag := "latest"
	script := fmt.Sprintf("run_%s.sh", controller)
	logfile := fmt.Sprintf("%s/%s.log", tc.logPath, controller)
	containerPrefix := fmt.Sprintf("%s-%s", tc.name, controller)

	return runControllerImpl(repo, tag, script, tc.filesPath, logfile, containerPrefix, env)
}

func managedContainerName(prefix string) string {
	return fmt.Sprintf("%s-managed", prefix)
}

func controllerContainerName(prefix string) string {
	return fmt.Sprintf("%s-controller", prefix)
}

func runControllerImpl(repo, tag, script, filesPath, logfile, containerPrefix string, env *env.Environment) (*f.BoxImage, error) {
	managedName := managedContainerName(containerPrefix)
	controllerName := controllerContainerName(containerPrefix)
	if err := env.RemoveContainers(managedName, controllerName); err != nil {
		return nil, fmt.Errorf("failed to remove containers: %s", err)
	}

	fixture, err := f.NewFixture(env, containerPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to create fixture: %s", err)
	}
	defer fixture.Close()

	managed, err := f.NewBox("gosible/ubuntu", "latest", fixture, f.WithName(managedName), f.WithNetworkAlias("managed"))
	if err != nil {
		return nil, fmt.Errorf("failed to create managed node: %s", err)
	}

	data := dc.HostMount{
		Source:   filesPath,
		Target:   containerWorkdir,
		Type:     "bind",
		ReadOnly: true,
	}
	ctrl, err := f.NewBox(repo, tag, fixture, f.WithMounts(data), f.WithName(controllerContainerName(containerPrefix)), f.WithNetworkAlias("controller"))
	setupPath := fmt.Sprintf("%s/%s", containerWorkdir, "setup.sh")
	scriptPath := fmt.Sprintf("%s/%s", containerWorkdir, script)

	if err != nil {
		return nil, fmt.Errorf("failed to create controller node: %s", err)
	}

	// open logfile
	log, err := os.OpenFile(logfile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open logfile: %s", err)
	}
	defer log.Close()

	code, err := ctrl.Exec([]string{setupPath}, log, log)
	if err != nil {
		return nil, fmt.Errorf("failed to run setup script: %s", err)
	} else if code != 0 {
		return nil, fmt.Errorf("setup script exited with code %d", code)
	}

	code, err = ctrl.Exec([]string{scriptPath}, log, log)

	if err != nil {
		return nil, fmt.Errorf("failed to run script: %s", err)
	} else if code != 0 {
		return nil, fmt.Errorf("script %s exited with code %d", script, code)
	}

	return managed.Commit()
}

func preprocessTestCase(file fs.FileInfo) (*testCase, error) {
	if !file.IsDir() {
		return nil, nil
	}
	testPath := fmt.Sprintf("%s/cases/%s", workingDir, file.Name())

	testEnvPath := fmt.Sprintf("%s/%s", testDir, file.Name())
	testLogPath := fmt.Sprintf("%s/log", testEnvPath)
	testFilesPath := fmt.Sprintf("%s/files", testEnvPath)

	if err := exec.Command("mkdir", testEnvPath, testLogPath, testFilesPath).Run(); err != nil {
		return nil, err
	}
	// copy default test environment to test case directory
	if err := exec.Command("cp", "-a", fmt.Sprintf("%s/.", defaultTestEnv), testFilesPath).Run(); err != nil {
		return nil, fmt.Errorf("failed to copy default test environment: %s", err)
	}
	if err := exec.Command("cp", "-a", fmt.Sprintf("%s/.", testPath), testFilesPath).Run(); err != nil {
		return nil, fmt.Errorf("failed to copy test case: %s", err)
	}

	return &testCase{
		name:      file.Name(),
		envPath:   testEnvPath,
		logPath:   testLogPath,
		filesPath: testFilesPath,
	}, nil
}

func getTestCases() ([]testCase, error) {
	testRegex, err := regexp.Compile(*testRegexString)
	if err != nil {
		return nil, fmt.Errorf("failed to compile test regex: %s", err)
	}

	files, err := ioutil.ReadDir(fmt.Sprintf("%s/cases", workingDir))
	if err != nil {
		return nil, err
	}
	var testCases []testCase

	for _, file := range files {
		if !testRegex.MatchString(file.Name()) {
			continue
		}

		tc, err := preprocessTestCase(file)
		if err != nil {
			return nil, err
		}
		if tc != nil {
			testCases = append(testCases, *tc)
		}
	}
	return testCases, nil
}

func TestE2e(t *testing.T) {
	var err error

	if !*e2eRun {
		t.Skip("Skipping e2e tests, run go test with -e2e flag to run them")
		return
	}
	if err = os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create e2e logs dir: %s", err)
	}

	environment, err := env.NewEnvironment()
	if err != nil {
		t.Fatal(fmt.Errorf("failed to create environment: %v", err))
	}

	t.Cleanup(func() {
		environment.Close()
	})
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		t.Log("Received interrupt, cleaning up")
		environment.Close()
		os.Exit(1)
		return
	}()

	testCases, err := getTestCases()
	if err != nil {
		t.Fatal(fmt.Errorf("failed to get test cases: %v", err))
	}

	if *testRegexString != "" {
		fmt.Printf("\nRunning tests matching %s\n\n", *testRegexString)
	}

	for _, tc := range testCases {
		func(tc testCase) {
			runner := func(t *testing.T) {
				// FIXME: parallel is hard and there is a sneaky bug somewhere, so for now we just run sequentially
				//t.Parallel()
				runTestCase(t, tc, environment)
			}
			t.Run(tc.name, runner)
		}(tc)
	}
}
