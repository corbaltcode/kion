package saml

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	kioncli "github.com/kionsoftware/kion-cli/lib/kion"
	"github.com/zalando/go-keyring"
)

const (
	sessionKey             = "session"
	accessTokenLifetime    = 570 * time.Second
	accessTokenMinValidity = 30 * time.Second
)

type Config struct {
	Host                  string
	IDMS                  int
	MetadataFile          string
	ServiceProviderIssuer string
	PrintURL              bool
}

type storedSession struct {
	AccessToken   string    `json:"access_token"`
	AccessExpiry  time.Time `json:"access_expiry"`
	RefreshToken  string    `json:"refresh_token,omitempty"`
	RefreshExpiry time.Time `json:"refresh_expiry,omitempty"`
}

// Authenticate returns an access token from a cached, refreshed, or newly
// created SAML session. When force is true, it always starts a browser login.
func Authenticate(cfg Config, force bool) (string, time.Time, error) {
	if force {
		return authenticate(cfg)
	}

	session, err := loadSession(cfg)
	if err != nil {
		return "", time.Time{}, err
	}
	if session.AccessToken != "" && session.AccessExpiry.After(time.Now().Add(accessTokenMinValidity)) {
		return session.AccessToken, session.AccessExpiry, nil
	}

	if session.RefreshToken != "" && session.RefreshExpiry.After(time.Now()) {
		refreshed, refreshErr := kioncli.RefreshSession(appURL(cfg.Host), session.RefreshToken)
		if refreshErr == nil && refreshed.Access.Token != "" {
			session.AccessToken = refreshed.Access.Token
			session.AccessExpiry = parseAccessExpiry(refreshed.Access.Expiry)
			if err := saveSession(cfg, session); err != nil {
				return "", time.Time{}, err
			}
			return session.AccessToken, session.AccessExpiry, nil
		}
	}

	return authenticate(cfg)
}

func authenticate(cfg Config) (string, time.Time, error) {
	loadMetadata := kioncli.ReadSAMLMetadataFile
	if strings.HasPrefix(cfg.MetadataFile, "http://") || strings.HasPrefix(cfg.MetadataFile, "https://") {
		loadMetadata = kioncli.DownloadSAMLMetadata
	}
	metadata, err := loadMetadata(cfg.MetadataFile)
	if err != nil {
		return "", time.Time{}, err
	}
	authData, err := kioncli.AuthenticateSAML(
		appURL(cfg.Host),
		metadata,
		cfg.ServiceProviderIssuer,
		cfg.PrintURL,
	)
	if err != nil {
		return "", time.Time{}, err
	}

	session := storedSession{
		AccessToken:   authData.AuthToken,
		AccessExpiry:  time.Now().Add(accessTokenLifetime),
		RefreshToken:  authData.RefreshToken,
		RefreshExpiry: authData.RefreshExpiry,
	}
	if err := saveSession(cfg, session); err != nil {
		return "", time.Time{}, err
	}
	return session.AccessToken, session.AccessExpiry, nil
}

func loadSession(cfg Config) (storedSession, error) {
	encoded, err := keyring.Get(keyringService(cfg), sessionKey)
	if errors.Is(err, keyring.ErrNotFound) {
		return storedSession{}, nil
	}
	if err != nil {
		return storedSession{}, err
	}

	var session storedSession
	if err := json.Unmarshal([]byte(encoded), &session); err != nil {
		return storedSession{}, fmt.Errorf("decode cached SAML session: %w", err)
	}
	return session, nil
}

func saveSession(cfg Config, session storedSession) error {
	encoded, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return keyring.Set(keyringService(cfg), sessionKey, string(encoded))
}

func keyringService(cfg Config) string {
	return fmt.Sprintf("%s/%d/saml", cfg.Host, cfg.IDMS)
}

func appURL(host string) string {
	u := url.URL{
		Scheme: "https",
		Host:   host,
	}
	return u.String()
}

func parseAccessExpiry(value string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05-0700"} {
		if expiry, err := time.Parse(layout, value); err == nil {
			return expiry
		}
	}
	return time.Now().Add(accessTokenLifetime)
}
