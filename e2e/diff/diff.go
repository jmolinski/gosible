package diff

import (
	"encoding/json"
	"fmt"
	dc "github.com/ory/dockertest/v3/docker"
	"github.com/scylladb/gosible/e2e/env"
	f "github.com/scylladb/gosible/e2e/fixtures"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type ImageComparer struct {
	fixture *f.Fixture
	box     *f.Box
}

func NewImageComparer(prefix string, env *env.Environment) (*ImageComparer, error) {
	differName := fmt.Sprintf("%s-differ", prefix)
	if err := env.RemoveContainers(differName); err != nil {
		return nil, fmt.Errorf("failed to remove previous containers: %w", err)
	}

	fixture, err := f.NewFixture(env, differName)
	if err != nil {
		return nil, err
	}
	// Gives access to docker api from inside the container.
	dockerSock := dc.HostMount{
		Target: "/var/run/docker.sock",
		Source: "/var/run/docker.sock",
		Type:   "bind",
	}
	box, err := f.NewBox("gosible/diff", "latest", fixture, f.WithMounts(dockerSock), f.WithName(differName))
	if err != nil {
		fixture.Close()
		return nil, err
	}
	return &ImageComparer{fixture, box}, nil
}

func (d *ImageComparer) Close() error {
	return d.fixture.Close()
}

func addr(repository, tag string) string {
	return "daemon://" + repository + ":" + tag
}

type FileDiffDescription struct {
	FilePath string `json:"Name"`
}

// ShallowImageDiff contains lists of files that are different between two images,
// but not the difference between content of those files.
type ShallowImageDiff struct {
	AnsibleCommit string `json:"Image1"`
	GosibleCommit string `json:"Image2"`
	Differences   struct {
		Adds []FileDiffDescription `json:"Adds"`
		Dels []FileDiffDescription `json:"Dels"`
		Mods []FileDiffDescription `json:"Mods"`
	} `json:"Diff"`
}

func (d *ShallowImageDiff) IsIdentical() bool {
	return len(d.Differences.Adds) == 0 && len(d.Differences.Dels) == 0 && len(d.Differences.Mods) == 0
}

func (d *ShallowImageDiff) String() string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Differences between %s and %s:\n", d.AnsibleCommit, d.GosibleCommit))
	writeDiffs := func(kind string, diffs []FileDiffDescription) {
		if len(diffs) == 0 {
			return
		}
		builder.WriteString(fmt.Sprintf("%s:\n", kind))
		for _, diff := range diffs {
			builder.WriteString(fmt.Sprintf("\t%s\n", diff.FilePath))
		}
	}

	writeDiffs("Present only in Gosible", d.Differences.Adds)
	writeDiffs("Present only in Ansible", d.Differences.Dels)
	writeDiffs("File with different content", d.Differences.Mods)

	return builder.String()
}

// filterFiles removes ignored files from each of the lists of files.
func (d *ShallowImageDiff) filterFiles() *ShallowImageDiff {
	d.Differences.Adds = rejectIgnoredFiles(d.Differences.Adds)
	d.Differences.Dels = rejectIgnoredFiles(d.Differences.Dels)
	d.Differences.Mods = rejectIgnoredFiles(d.Differences.Mods)
	return d
}

func parseShallowDiffResult(diff string) (*ShallowImageDiff, error) {
	var result []ShallowImageDiff
	if err := json.Unmarshal([]byte(diff), &result); err != nil {
		return nil, err
	} else if len(result) != 1 {
		return nil, fmt.Errorf("expected 1 result, got %d", len(result))
	}

	return &result[0], nil
}

func (d *ImageComparer) ShallowDiff(ansibleImg, gosibleImg *f.BoxImage) (*ShallowImageDiff, error) {
	ansibleAddr := addr(ansibleImg.Repository, ansibleImg.Tag)
	gosibleAddr := addr(gosibleImg.Repository, gosibleImg.Tag)

	var resultJson strings.Builder
	cmd := []string{"container-diff", "diff", ansibleAddr, gosibleAddr, "--type=file", "--json"}
	code, err := d.box.Exec(cmd, &resultJson, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to execute %s: %w", cmd, err)
	} else if code != 0 {
		return nil, fmt.Errorf("failed to execute %s: exit code %d", cmd, code)
	}

	diff, err := parseShallowDiffResult(resultJson.String())
	if err != nil {
		return nil, fmt.Errorf("failed to parse result: %w", err)
	}
	return diff.filterFiles(), nil
}

func rejectDirs(paths []string) []string {
	var filtered []string
	if len(paths) == 0 {
		return filtered
	}

	sort.Strings(paths)
	for i, p := range paths[:len(paths)-1] {
		if strings.HasPrefix(paths[i+1], p) {
			continue
		}
		filtered = append(filtered, p)
	}
	filtered = append(filtered, paths[len(paths)-1])

	return filtered
}

func (d *ImageComparer) DeepDiff(ansibleImg, gosibleImg *f.BoxImage, dest string, diff *ShallowImageDiff) error {
	var ansiblePaths, gosiblePaths []string

	for _, d := range diff.Differences.Mods {
		ansiblePaths = append(ansiblePaths, d.FilePath)
		gosiblePaths = append(gosiblePaths, d.FilePath)
	}
	for _, d := range diff.Differences.Dels {
		ansiblePaths = append(ansiblePaths, d.FilePath)
	}
	for _, d := range diff.Differences.Adds {
		gosiblePaths = append(gosiblePaths, d.FilePath)
	}

	ansiblePaths = rejectDirs(ansiblePaths)
	gosiblePaths = rejectDirs(gosiblePaths)

	diffPath := filepath.Join(dest, "diff")
	ansibleDest := path.Join(diffPath, "ansible")
	gosibleDest := path.Join(diffPath, "gosible")
	if err := exec.Command("mkdir", "-p", ansibleDest, gosibleDest).Run(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	if err := ansibleImg.DownloadFiles(ansibleDest, ansiblePaths); err != nil {
		return fmt.Errorf("failed to download files from Ansible image: %w", err)
	}
	if err := gosibleImg.DownloadFiles(gosibleDest, gosiblePaths); err != nil {
		return fmt.Errorf("failed to download files from Gosible image: %w", err)
	}

	diffFile, err := os.Create(filepath.Join(dest, "results.diff"))
	if err != nil {
		return fmt.Errorf("failed to create diff file: %w", err)
	}
	defer diffFile.Close()

	diffCmd := exec.Command("diff", "-ruN", ansibleDest, gosibleDest)
	diffCmd.Stdout = diffFile
	diffCmd.Stderr = diffFile
	_ = diffCmd.Run()

	return nil
}
