package util

import (
	"errors"
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

type Config struct {
	Directory string `yaml:"directory"`
	Files     struct {
		Debug string `yaml:"debug"`
		Info  string `yaml:"info"`
		Error string `yaml:"error"`
	} `yaml:"files"`
	Rotation struct {
		MaxSize    int  `yaml:"max_size"`
		MaxBackups int  `yaml:"max_backups"`
		MaxAge     int  `yaml:"max_age"`
		Compress   bool `yaml:"compress"`
	} `yaml:"rotation"`
}

func LoadConfic(path string) (Config, error) {
	if path == "" {
		path = "config/config.yaml"
	}
	data, err := os.ReadFile(path)

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read logging config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse logging config: %w", err)
	}

	return cfg, nil
}
