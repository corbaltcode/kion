package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/knadh/koanf/v2"
)

func UserConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "kion"), nil
}

func UserConfigName() (string, error) {
	dir, err := UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yml"), nil
}

type Config struct {
	*koanf.Koanf
}

func (c *Config) DurationErr(path string) (time.Duration, error) {
	v := c.Duration(path)
	if v == 0 {
		return 0, fmt.Errorf("missing config value: %v", path)
	}
	return v, nil
}

func (c *Config) IntErr(path string) (int, error) {
	v := c.Int(path)
	if v == 0 {
		return 0, fmt.Errorf("missing config value: %v", path)
	}
	return v, nil
}

func (c *Config) StringErr(path string) (string, error) {
	v := c.String(path)
	if v == "" {
		return "", fmt.Errorf("missing config value: %v", path)
	}
	return v, nil
}
