package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/knadh/koanf/v2"
	"gopkg.in/yaml.v3"
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

const keyConfigFilename = "key.yml"

type KeyConfig struct {
	Key     string
	Created time.Time
}

func LoadKeyConfig() (*KeyConfig, error) {
	dir, err := UserConfigDir()
	if err != nil {
		return nil, err
	}

	config := KeyConfig{}

	name := filepath.Join(dir, keyConfigFilename)
	f, err := os.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return &config, nil
	} else if err != nil {
		return nil, err
	}
	defer f.Close()

	err = yaml.NewDecoder(f).Decode(&config)
	if err != nil {
		return nil, err
	}

	return &config, err
}

func (c *KeyConfig) Save() error {
	dir, err := UserConfigDir()
	if err != nil {
		return err
	}

	name := filepath.Join(dir, keyConfigFilename)
	err = os.MkdirAll(dir, 0700)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	return yaml.NewEncoder(f).Encode(c)
}
