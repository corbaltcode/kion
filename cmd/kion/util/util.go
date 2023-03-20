package util

import (
	"fmt"
	"time"

	"github.com/corbaltcode/kion/internal/client"
	"github.com/knadh/koanf/v2"
	"github.com/zalando/go-keyring"
)

func NewClient(cfg *Config) (*client.Client, error) {
	host, err := cfg.StringErr("host")
	if err != nil {
		return nil, err
	}

	idms, err := cfg.IntErr("idms")
	if err != nil {
		return nil, err
	}

	username, err := cfg.StringErr("username")
	if err != nil {
		return nil, err
	}

	// TODO: better error if no creds
	password, err := keyring.Get(KeyringService(host, idms), username)
	if err != nil {
		return nil, err
	}

	return client.Login(host, idms, username, password)
}

func KeyringService(host string, idms int) string {
	return fmt.Sprintf("%s/%d", host, idms)
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
