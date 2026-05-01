package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	envAPIURL = "HOMELAB_API_URL"
	envToken  = "HOMELAB_TOKEN"
)

type Config struct {
	APIURL string `yaml:"api_url"`
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "homelab"), nil
}

func configPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ResolveAPIURL returns the API URL with env var taking precedence over config file.
func (c *Config) ResolveAPIURL() (string, error) {
	if v := os.Getenv(envAPIURL); v != "" {
		return v, nil
	}
	if c.APIURL != "" {
		return c.APIURL, nil
	}
	return "", fmt.Errorf("API URL not configured (run 'hlctl config set-url' or set %s)", envAPIURL)
}

// Token returns the token from env var, or empty string if not set.
func Token() string {
	return os.Getenv(envToken)
}
