package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type RelayConfig struct {
	Plugins RelayPluginsConfig `yaml:"plugins"`
}

type RelayPluginsConfig struct {
	Source []RelayPluginConfig `yaml:"source"`

	Sink []RelayPluginConfig `yaml:"sink"`
}

type RelayPluginConfig struct {
	ID string `yaml:"id"`

	Command string `yaml:"command"`

	Args []string `yaml:"args"`

	Dir string `yaml:"dir"`

	Env map[string]string `yaml:"env"`
}

func (v *RelayPluginConfig) GetID() string {
	if v.ID == "" {
		return v.Command
	}
	return v.ID
}

func (v *RelayPluginConfig) GetEnv() []string {
	out := os.Environ()
	for k, v := range v.Env {
		out = append(out, k+"="+v)
	}
	return out
}

func LoadRelayConfig(dir string) (*RelayConfig, error) {
	names := []string{
		filepath.Join(dir, ".terraform-relay.yaml"),
		filepath.Join(dir, ".terraform-relay.yml"),
	}
	var out RelayConfig
	for _, name := range names {
		b, err := os.ReadFile(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}

		if err := yaml.Unmarshal(b, &out); err != nil {
			return nil, err
		}
		return &out, nil
	}
	return &out, nil
}
