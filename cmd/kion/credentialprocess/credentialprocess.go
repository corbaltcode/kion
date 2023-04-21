package credentialprocess

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/corbaltcode/kion/cmd/kion/config"
	"github.com/corbaltcode/kion/cmd/kion/util"
	"github.com/corbaltcode/kion/internal/client"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func New(cfg *config.Config, keyCfg *config.KeyConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "credential-process",
		Short: "Credential process for AWS CLI",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg, keyCfg)
		},
	}

	cmd.Flags().StringP("account-id", "", "", "AWS account ID")
	cmd.Flags().StringP("cloud-access-role", "", "", "cloud access role")
	cmd.Flags().StringP("session-duration", "", "1h", "duration of temporary credentials")

	return cmd
}

func run(cfg *config.Config, keyCfg *config.KeyConfig) error {
	userConfigDir, err := config.UserConfigDir()
	if err != nil {
		return err
	}
	host, err := cfg.StringErr("host")
	if err != nil {
		return err
	}
	idms, err := cfg.IntErr("idms")
	if err != nil {
		return err
	}
	username, err := cfg.StringErr("username")
	if err != nil {
		return err
	}
	accountID, err := cfg.StringErr("account-id")
	if err != nil {
		return err
	}
	cloudAccessRole, err := cfg.StringErr("cloud-access-role")
	if err != nil {
		return err
	}
	sessionDuration, err := cfg.DurationErr("session-duration")
	if err != nil {
		return err
	}

	cacheName := filepath.Join(userConfigDir, "credential_process_cache.yml")

	credsWithExpiry, err := readCachedCredentials(cacheName, host, idms, username, accountID, cloudAccessRole)
	if err != nil {
		return err
	}

	// no cached credentials or cached credentials expired; get new ones
	if credsWithExpiry == nil {
		kion, err := util.NewClient(cfg, keyCfg)
		if err != nil {
			return err
		}
		creds, err := kion.GetTemporaryCredentialsByCloudAccessRole(accountID, cloudAccessRole)
		if err != nil {
			return err
		}

		credsWithExpiry = &credentialsWithExpiry{
			Credentials: *creds,
			Expiry:      time.Now().Add(sessionDuration),
		}

		err = writeCachedCredentials(cacheName, host, idms, username, accountID, cloudAccessRole, credsWithExpiry)
		if err != nil {
			return err
		}
	}

	out := map[string]interface{}{
		"Version":         1,
		"AccessKeyId":     credsWithExpiry.Credentials.AccessKeyID,
		"SecretAccessKey": credsWithExpiry.Credentials.SecretAccessKey,
		"SessionToken":    credsWithExpiry.Credentials.SessionToken,
		"Expiration":      credsWithExpiry.Expiry.Format(time.RFC3339),
	}

	return json.NewEncoder(os.Stdout).Encode(out)
}

type credentialsWithExpiry struct {
	Credentials client.TemporaryCredentials
	Expiry      time.Time
}

func readCachedCredentials(cacheName string, host string, idms int, username string, accountID string, cloudAccessRole string) (*credentialsWithExpiry, error) {
	cache, err := loadCache(cacheName)
	if err != nil {
		return nil, err
	}
	creds, ok := cache[cacheKey(host, idms, username, accountID, cloudAccessRole)]
	if ok && time.Now().Before(creds.Expiry) {
		return &creds, nil
	}

	return nil, nil
}

func writeCachedCredentials(cacheName string, host string, idms int, username string, accountID string, cloudAccessRole string, creds *credentialsWithExpiry) error {
	cache, err := loadCache(cacheName)
	if err != nil {
		return err
	}
	cache[cacheKey(host, idms, username, accountID, cloudAccessRole)] = *creds
	f, err := os.OpenFile(cacheName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	return yaml.NewEncoder(f).Encode(cache)
}

func loadCache(cacheName string) (map[string]credentialsWithExpiry, error) {
	f, err := os.Open(cacheName)
	if errors.Is(err, fs.ErrNotExist) {
		return make(map[string]credentialsWithExpiry), nil
	} else if err != nil {
		return nil, err
	}
	defer f.Close()

	var cache map[string]credentialsWithExpiry
	err = yaml.NewDecoder(f).Decode(&cache)
	if err != nil {
		return nil, fmt.Errorf("decoding credential process cache: %w", err)
	}

	return cache, nil
}

func cacheKey(host string, idms int, username string, accountID string, cloudAccessRole string) string {
	return fmt.Sprintf("%v:%v:%v:%v:%v", host, idms, username, accountID, cloudAccessRole)
}
