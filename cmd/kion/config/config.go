package config

import (
	"fmt"
	"time"

	"github.com/knadh/koanf/v2"
)

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
