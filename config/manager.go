package config

import (
	"fmt"
	"github.com/jinzhu/copier"
	"github.com/scylladb/gosible/utils/callbacks"
)

var managerSingleton *manager

// TODO add support for env-based config variables

type manager struct {
	Settings       ConfigData
	BaseDefs       ConfigData
	PluginSettings map[string]map[string]interface{} // TODO add support for plugin settings

	configFilePath string
}

func DestroyDefaultManager() {
	managerSingleton = nil
}

func Manager() *manager {
	if managerSingleton != nil {
		return managerSingleton
	}

	baseDefs, err := ParseBaseDefsConfigData(baseDefsYamlContents)
	if err != nil {
		panic(fmt.Sprintf("Error when parsing base defs file: %s", err))
	}

	var settings ConfigData
	err = copier.CopyWithOption(&settings, baseDefs, copier.Option{IgnoreEmpty: false, DeepCopy: true})
	if err != nil {
		panic(fmt.Sprintf("error when copying settings: %s", err))
	}

	managerSingleton = &manager{
		BaseDefs: *baseDefs,
		Settings: settings,
	}
	return managerSingleton
}

// TryLoadConfigFile determines the location of a config file and tries to load config variables from it.
// Pass an empty string as configFilePath to try to find the config file in standard locations.
func (m *manager) TryLoadConfigFile(configFilePath string) error {
	defer m.markConfigLoaded()

	if configFilePath == "" {
		configFilePath = findIniConfigFile()
	}

	if configFilePath == "" {
		// No config file is not an error.
		return nil
	}

	if err := parseConfigFile(configFilePath, &m.Settings); err != nil {
		_ = copier.CopyWithOption(&m.Settings, &m.BaseDefs, copier.Option{IgnoreEmpty: false, DeepCopy: true})
		return fmt.Errorf("config file at %s could not be parsed: %w", configFilePath, err)
	}

	m.configFilePath = configFilePath

	return nil
}

func (m *manager) markConfigLoaded() {
	callbacks.RegisterPersistentEvent(callbacks.ConfigLoaded)
}
