package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is lazylore's on-disk configuration. Every field is optional.
type Config struct {
	LorePath string `yaml:"lorePath"`
}

// Load reads config.yml from dir. A missing file is not an error - it
// returns the zero Config, matching "use the defaults" semantics.
func Load(dir string) (Config, error) {
	path := filepath.Join(dir, "config.yml")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return cfg, nil
}
