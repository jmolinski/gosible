package env

import (
	"fmt"
	"io"
	"sync"

	dt "github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
)

type Environment struct {
	pool    *dt.Pool
	client  *dc.Client
	closers []io.Closer
	mu      sync.Mutex
}

func NewEnvironment() (*Environment, error) {
	pool, err := dt.NewPool("")
	if err != nil {
		return nil, err
	}
	return &Environment{
		pool:   pool,
		client: pool.Client,
	}, nil
}

func (e *Environment) Close() error {
	var errs []error
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, c := range e.closers {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("close failed: %v", errs)
	}

	return nil
}

type SyncAction func(pool *dt.Pool, client *dc.Client) error

func (e *Environment) With(action SyncAction) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return action(e.pool, e.client)
}

func (e *Environment) AppendCloser(c io.Closer) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.closers = append(e.closers, c)
}

func (e *Environment) RemoveContainers(names ...string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, name := range names {
		container, found := e.pool.ContainerByName(name)
		if found {
			if err := container.Close(); err != nil {
				return fmt.Errorf("failed to remove container: %w", err)
			}
		}
	}
	return nil
}
