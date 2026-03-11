package util

import (
	"fmt"
	"time"

	"github.com/corbaltcode/kion/cmd/kion/config"
	"github.com/corbaltcode/kion/internal/client"
	"github.com/corbaltcode/kion/internal/saml"
	"github.com/zalando/go-keyring"
)

const AppAPIKeyName = "Kion Tool"

// samlTokenGracePeriod re-authenticates this long before the cached SAML token expires.
const samlTokenGracePeriod = 30 * time.Second

func NewClient(cfg *config.Config, keyCfg *config.KeyConfig) (*client.Client, error) {
	host, err := cfg.StringErr("host")
	if err != nil {
		return nil, err
	}

	// SAML takes precedence when configured, regardless of any leftover key.yml.
	samlMetadata := cfg.String("saml-metadata")
	if samlMetadata != "" {
		return newSAMLClient(cfg, host, samlMetadata)
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

func newSAMLClient(cfg *config.Config, host string, metadataSource string) (*client.Client, error) {
	// Default SP issuer matches Kion's standard registration with the IDP.
	spIssuer := cfg.String("saml-sp-issuer")
	if spIssuer == "" {
		spIssuer = "https://" + host + "/api/v1/saml/auth"
	}

	// Use cached SAML token if it is still valid.
	tokenCfg, err := config.LoadSAMLTokenConfig()
	if err != nil {
		return nil, fmt.Errorf("loading cached SAML token: %w", err)
	}
	if tokenCfg.Token != "" && time.Now().Before(tokenCfg.Expires.Add(-samlTokenGracePeriod)) {
		return client.NewWithToken(host, tokenCfg.Token, tokenCfg.Expires), nil
	}

	// Token is absent or expiring soon — re-authenticate via browser.
	printURL := cfg.Bool("saml-print-url")

	metadata, err := saml.Metadata(metadataSource)
	if err != nil {
		return nil, err
	}

	token, err := saml.Authenticate(host, metadata, spIssuer, printURL)
	if err != nil {
		return nil, fmt.Errorf("SAML authentication failed: %w", err)
	}

	// Kion SAML tokens are valid for 10 minutes.
	expires := time.Now().Add(10 * time.Minute)
	tokenCfg = &config.SAMLTokenConfig{Token: token, Expires: expires}
	if saveErr := tokenCfg.Save(); saveErr != nil {
		// Non-fatal: we have a valid token for this invocation even if caching fails.
		fmt.Printf("warning: could not cache SAML token: %v\n", saveErr)
	}

	return client.NewWithToken(host, token, expires), nil
}

func KeyringService(host string, idms int) string {
	return fmt.Sprintf("%s/%d", host, idms)
}
