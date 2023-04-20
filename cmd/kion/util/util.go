package util

import (
	"fmt"

	"github.com/corbaltcode/kion/cmd/kion/config"
	"github.com/corbaltcode/kion/internal/client"
	"github.com/zalando/go-keyring"
)

func NewClient(cfg *config.Config, keyCfg *config.KeyConfig) (*client.Client, error) {
	host, err := cfg.StringErr("host")
	if err != nil {
		return nil, err
	}

	if keyCfg.Key != "" {
		return client.NewWithAppAPIKey(host, keyCfg.Key), nil
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
