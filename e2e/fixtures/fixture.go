package fixtures

import (
	"fmt"
	"github.com/scylladb/gosible/e2e/env"
	"io"

	dt "github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
)

type Fixture struct {
	name    string
	env     *env.Environment
	network *dt.Network
	closers []io.Closer
}

const networkNameLen = 8

func networkName(name string) string {
	return fmt.Sprintf("%s-network", name)
}

func removePreviousNetwork(name string, client *dc.Client) error {
	networks, err := client.ListNetworks()
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}
	for _, network := range networks {
		if network.Name == name {
			if err := client.RemoveNetwork(network.ID); err != nil {
				return fmt.Errorf("failed to remove network: %w", err)
			}
		}
	}
	return nil
}

func NewFixture(env *env.Environment, name string) (*Fixture, error) {
	var fixture *Fixture
	var err error

	netName := networkName(name)

	err = env.With(func(pool *dt.Pool, client *dc.Client) error {
		if err := removePreviousNetwork(netName, client); err != nil {
			return fmt.Errorf("failed to remove previous network: %w", err)
		}

		net, err := pool.CreateNetwork(netName)
		if err != nil {
			return fmt.Errorf("failed to create network: %w", err)
		}
		fixture = &Fixture{
			name:    name,
			env:     env,
			network: net,
			closers: []io.Closer{},
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	env.AppendCloser(fixture)
	return fixture, nil
}

func (f *Fixture) Close() error {
	var errs []error
	for _, c := range f.closers {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := f.network.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) != 0 {
		return fmt.Errorf("close failed: %v", errs)
	}

	return nil
}
