package readiness

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version int          `yaml:"version"`
	Checks  CheckConfig  `yaml:"checks"`
	Docker  DockerConfig `yaml:"docker"`
}

type CheckConfig struct {
	Profile []string `yaml:"profile"`
	Ignore  []string `yaml:"ignore"`
}

type DockerConfig struct {
	AllowDockerSocket []string `yaml:"allowDockerSocket"`
	AllowRoot         []string `yaml:"allowRoot"`
	WorkerServices    []string `yaml:"workerServices"`
}

func EmptyConfig() Config {
	return Config{Version: 1}
}

func LoadConfig(path string) (Config, string, error) {
	if path == "" {
		found, err := findOptional(".deployshuttle.yml")
		if err != nil {
			return EmptyConfig(), "", err
		}
		path = found
	}
	if path == "" {
		return EmptyConfig(), "", nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return EmptyConfig(), "", err
	}
	cfg := EmptyConfig()
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return EmptyConfig(), "", err
	}
	if cfg.Version != 0 && cfg.Version != 1 {
		return EmptyConfig(), "", errors.New("unsupported .deployshuttle.yml version")
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	return cfg, path, nil
}

func findOptional(name string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}
