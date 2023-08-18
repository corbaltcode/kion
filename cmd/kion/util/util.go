package util

import (
	"fmt"
	"time"

	"github.com/corbaltcode/kion/cmd/kion/config"
	"github.com/corbaltcode/kion/internal/client"
	"github.com/zalando/go-keyring"
)

const AppAPIKeyName = "Kion Tool"

func NewClient(cfg *config.Config, keyCfg *config.KeyConfig) (*client.Client, error) {
	host, err := cfg.StringErr("host")
	if err != nil {
		return nil, err
	}

	if keyCfg.Key != "" {
		appAPIKeyDuration, err := cfg.DurationErr("app-api-key-duration")
		if err != nil {
			return nil, err
		}

		if cfg.Bool("rotate-app-api-keys") {
			expiry := keyCfg.Created.Add(appAPIKeyDuration)

			// rotate if expiring within three days
			if expiry.Before(time.Now().Add(time.Hour * 72)) {
				kion := client.NewWithAppAPIKey(host, keyCfg.Key, expiry)
				key, err := kion.RotateAppAPIKey(keyCfg.Key)
				if err != nil {
					return nil, err
				}

				// can't know exact expiry before getting metadata, so pass zero Time meaning "no expiry"
				kion = client.NewWithAppAPIKey(host, key.Key, time.Time{})
				keyMetadata, err := kion.GetAppAPIKeyMetadata(key.ID)
				if err != nil {
					return nil, err
				}

				keyCfg.Key = key.Key
				keyCfg.Created = keyMetadata.Created
				err = keyCfg.Save()
				if err != nil {
					return nil, err
				}
			}
		}

		return client.NewWithAppAPIKey(host, keyCfg.Key, keyCfg.Created.Add(appAPIKeyDuration)), nil
	}

	idms, err := cfg.IntErr("idms")
	if err != nil {
		return nil, err
	}
	username, err := cfg.StringErr("username")
	if err != nil {
		return nil, err
	}

	password, err := keyring.Get(KeyringService(host, idms), username)
	if err != nil {
		return nil, err
	}

	return client.Login(host, idms, username, password)
}

func KeyringService(host string, idms int) string {
	return fmt.Sprintf("%s/%d", host, idms)
}
