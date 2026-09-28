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

	var appAPIKeyDuration time.Duration
	if keyCfg.Key != "" {
		appAPIKeyDuration, err = cfg.DurationErr("app-api-key-duration")
		if err != nil {
			return nil, err
		}
		expiry := keyCfg.Created.Add(appAPIKeyDuration)
		now := time.Now()

		if now.Before(expiry) {
			// rotate if expiring within three days
			if cfg.Bool("rotate-app-api-keys") && expiry.Before(now.Add(time.Hour*72)) {
				kion := client.NewWithAppAPIKey(host, keyCfg.Key, expiry)
				key, err := kion.RotateAppAPIKey(keyCfg.Key)
				if err != nil {
					return nil, err
				}

				// can't know exact expiry before getting metadata, so pass zero Time meaning "no expiry"
				kion = client.NewWithAppAPIKey(host, key.Key, time.Time{})
				if err := SaveAppAPIKey(kion, key, keyCfg); err != nil {
					return nil, err
				}
			}

			return client.NewWithAppAPIKey(host, keyCfg.Key, keyCfg.Created.Add(appAPIKeyDuration)), nil
		}

		if !cfg.Bool("rotate-app-api-keys") || cfg.String("auth-method") != "saml" {
			return client.NewWithAppAPIKey(host, keyCfg.Key, expiry), nil
		}
	}

	idms, err := cfg.IntErr("idms")
	if err != nil {
		return nil, err
	}
	if cfg.String("auth-method") == "saml" {
		metadataFile, err := cfg.StringErr("saml-metadata-file")
		if err != nil {
			return nil, err
		}
		issuer, err := cfg.StringErr("saml-sp-issuer")
		if err != nil {
			return nil, err
		}
		kion, err := client.Login("saml", host, idms, "", "", metadataFile, issuer, cfg.Bool("saml-print-url"), false)
		if err != nil {
			return nil, err
		}
		if keyCfg.Key == "" {
			return kion, nil
		}

		// An existing key reaching this point has expired; replace it using the SAML session.
		key, err := kion.CreateAppAPIKey(AppAPIKeyName)
		if err != nil {
			return nil, err
		}
		if err := SaveAppAPIKey(kion, key, keyCfg); err != nil {
			return nil, err
		}
		return client.NewWithAppAPIKey(host, keyCfg.Key, keyCfg.Created.Add(appAPIKeyDuration)), nil
	}
	username, err := cfg.StringErr("username")
	if err != nil {
		return nil, err
	}

	password, err := keyring.Get(KeyringService(host, idms), username)
	if err != nil {
		return nil, err
	}

	return client.Login("password", host, idms, username, password, "", "", false, false)
}

// SaveAppAPIKey fetches a key's creation timestamp and persists it with the key.
func SaveAppAPIKey(kion *client.Client, key *client.AppAPIKey, keyCfg *config.KeyConfig) error {
	keyMetadata, err := kion.GetAppAPIKeyMetadata(key.ID)
	if err != nil {
		return err
	}
	keyCfg.Key = key.Key
	keyCfg.Created = keyMetadata.Created
	return keyCfg.Save()
}

func KeyringService(host string, idms int) string {
	return fmt.Sprintf("%s/%d", host, idms)
}
