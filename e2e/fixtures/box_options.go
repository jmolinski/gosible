package fixtures

import (
	dt "github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
)

type BoxOption struct {
	runOption                func(*dt.RunOptions)
	hostConfig               func(config *dc.HostConfig)
	networkConnectionOptions func(*dc.NetworkConnectionOptions)
}

func newRunOption(option func(*dt.RunOptions)) BoxOption {
	return BoxOption{runOption: option}
}

func newHostConfig(config func(*dc.HostConfig)) BoxOption {
	return BoxOption{hostConfig: config}
}

func newNetworkConnectionOptions(options func(*dc.NetworkConnectionOptions)) BoxOption {
	return BoxOption{networkConnectionOptions: options}
}

func (b BoxOption) asRunOption(option *dt.RunOptions) {
	if b.runOption != nil {
		b.runOption(option)
	}
}
func (b BoxOption) asHostConfig(config *dc.HostConfig) {
	if b.hostConfig != nil {
		b.hostConfig(config)
	}
}

func (b BoxOption) asNetworkConnectionOptions(options *dc.NetworkConnectionOptions) {
	if b.networkConnectionOptions != nil {
		b.networkConnectionOptions(options)
	}
}

func WithName(name string) BoxOption {
	return newRunOption(func(option *dt.RunOptions) {
		option.Name = name
	})
}

func WithMounts(mounts ...dc.HostMount) BoxOption {
	return newHostConfig(func(config *dc.HostConfig) {
		config.Mounts = append(config.Mounts, mounts...)
	})
}

func WithNetworkAlias(name string) BoxOption {
	return newNetworkConnectionOptions(func(options *dc.NetworkConnectionOptions) {
		options.EndpointConfig.Aliases = append(options.EndpointConfig.Aliases, name)
	})
}
