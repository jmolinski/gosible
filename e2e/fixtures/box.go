package fixtures

import (
	"fmt"
	dt "github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
	"github.com/scylladb/gosible/e2e/env"
	"github.com/scylladb/gosible/e2e/utils"
	"io"
	"os"
	"os/exec"
	"path"
)

type Box struct {
	fixture  *Fixture
	resource *dt.Resource
}

func NewBox(repo, tag string, fixture *Fixture, options ...BoxOption) (box *Box, err error) {
	runOptions := dt.RunOptions{
		Repository: repo,
		Tag:        tag,
	}
	for _, o := range options {
		o.asRunOption(&runOptions)
	}
	hostConfig := func(conf *dc.HostConfig) {
		for _, o := range options {
			o.asHostConfig(conf)
		}
	}
	err = fixture.env.With(func(pool *dt.Pool, client *dc.Client) error {
		res, err := pool.RunWithOptions(&runOptions, hostConfig)
		if err != nil {
			return fmt.Errorf("failed to run image: %w", err)
		}
		box = &Box{
			fixture:  fixture,
			resource: res,
		}
		fixture.closers = append(fixture.closers, box)

		var networkConnectionOptions dc.NetworkConnectionOptions
		networkConnectionOptions.EndpointConfig = &dc.EndpointConfig{}
		networkConnectionOptions.Container = res.Container.ID

		for _, o := range options {
			o.asNetworkConnectionOptions(&networkConnectionOptions)
		}

		err = client.ConnectNetwork(fixture.network.Network.ID, networkConnectionOptions)
		if err != nil {
			return fmt.Errorf("failed to connect network: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return box, nil
}

func (b *Box) Close() error {
	return b.resource.Close()
}

func (b *Box) Exec(cmd []string, stdout, stderr io.Writer) (int, error) {
	options := dt.ExecOptions{
		StdOut: stdout,
		StdErr: stderr,
	}
	return b.resource.Exec(cmd, options)
}

type BoxImage struct {
	env        *env.Environment
	img        *dc.Image
	Repository string
	Tag        string
}

func (i *BoxImage) Remove() error {
	return i.env.With(func(_ *dt.Pool, client *dc.Client) error {
		return client.RemoveImage(i.img.ID)
	})
}

func (i *BoxImage) ID() string {
	return i.img.ID
}

func (i *BoxImage) Img() *dc.Image {
	return i.img
}

func downloadFileTo(containerID, path, filePath string, client *dc.Client) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer file.Close()

	downloadOptions := dc.DownloadFromContainerOptions{
		Path:         path,
		OutputStream: file,
	}
	if err := client.DownloadFromContainer(containerID, downloadOptions); err != nil {
		return fmt.Errorf("failed to download file %s: %w", path, err)
	}
	return nil
}

func (i *BoxImage) DownloadFiles(dest string, paths []string) error {
	runOptions := dt.RunOptions{
		Repository: i.Repository,
		Tag:        i.Tag,
	}
	return i.env.With(func(pool *dt.Pool, client *dc.Client) error {
		res, err := pool.RunWithOptions(&runOptions)
		if err != nil {
			return fmt.Errorf("failed to run image: %w", err)
		}
		defer res.Close()

		for _, p := range paths {
			filePath := path.Join(dest, p)
			dirPath := path.Dir(filePath)

			if err := exec.Command("mkdir", "-p", dirPath).Run(); err != nil {
				return fmt.Errorf("failed to create directory %s: %v", dirPath, err)
			}

			if err := downloadFileTo(res.Container.ID, p, filePath, client); err != nil {
				return fmt.Errorf("failed to download file %s: %w", p, err)
			}
			if err := exec.Command("tar", "-xf", filePath, "-C", dirPath).Run(); err != nil {
				return fmt.Errorf("failed to extract file %s: %w", p, err)
			}
		}
		return nil
	})

}

const tagLength = 10

func randomTag() string {
	return utils.RandomString(tagLength)
}

func (b *Box) Commit() (boxImg *BoxImage, err error) {
	repo := "gosible/commit"
	tag := randomTag()

	options := dc.CommitContainerOptions{
		Container:  b.resource.Container.ID,
		Repository: repo,
		Tag:        tag,
	}
	err = b.fixture.env.With(func(_ *dt.Pool, client *dc.Client) error {
		img, err := client.CommitContainer(options)
		if err != nil {
			return err
		}
		boxImg = &BoxImage{
			env:        b.fixture.env,
			img:        img,
			Repository: repo,
			Tag:        tag,
		}
		return nil
	})

	return boxImg, err
}
