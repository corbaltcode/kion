package key

import (
	"errors"
	"time"

	"github.com/corbaltcode/kion/cmd/kion/config"
	"github.com/corbaltcode/kion/cmd/kion/util"
	"github.com/corbaltcode/kion/internal/client"
	"github.com/spf13/cobra"
)

func New(cfg *config.Config, keyCfg *config.KeyConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Manages the App API Key",
		Args:  cobra.NoArgs,
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Creates the App API Key",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cfg, keyCfg)
		},
	}
	createCmd.Flags().BoolP("force", "f", false, "overwrite existing key")

	rotateCmd := &cobra.Command{
		Use:   "rotate",
		Short: "Rotates the App API Key",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRotate(cfg, keyCfg)
		},
	}

	cmd.AddCommand(createCmd)
	cmd.AddCommand(rotateCmd)

	return cmd
}

func runCreate(cfg *config.Config, keyCfg *config.KeyConfig) error {
	if keyCfg.Key != "" && !cfg.Bool("force") {
		return errors.New("key exists; use --force to overwrite")
	}

	kion, err := util.NewClient(cfg, keyCfg)
	if err != nil {
		return err
	}

	key, err := kion.CreateAppAPIKey(util.AppAPIKeyName)
	if err != nil {
		return err
	}
	keyMetadata, err := kion.GetAppAPIKeyMetadata(key.ID)
	if err != nil {
		return err
	}

	keyCfg.Key = key.Key
	keyCfg.Created = keyMetadata.Created
	return keyCfg.Save()
}

func runRotate(cfg *config.Config, keyCfg *config.KeyConfig) error {
	host, err := cfg.StringErr("host")
	if err != nil {
		return err
	}
	appAPIKeyDuration, err := cfg.DurationErr("app-api-key-duration")
	if err != nil {
		return err
	}

	kion := client.NewWithAppAPIKey(host, keyCfg.Key, keyCfg.Created.Add(appAPIKeyDuration))
	key, err := kion.RotateAppAPIKey(keyCfg.Key)
	if err != nil {
		return err
	}

	// can't know exact expiry before getting metadata, so pass zero Time meaning "no expiry"
	kion = client.NewWithAppAPIKey(host, key.Key, time.Time{})
	keyMetadata, err := kion.GetAppAPIKeyMetadata(key.ID)
	if err != nil {
		return err
	}

	keyCfg.Key = key.Key
	keyCfg.Created = keyMetadata.Created
	return keyCfg.Save()
}
