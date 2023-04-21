package key

import (
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/corbaltcode/kion/cmd/kion/config"
	"github.com/corbaltcode/kion/cmd/kion/util"
	"github.com/corbaltcode/kion/internal/client"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
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

	password, err := keyring.Get(util.KeyringService(host, idms), username)
	if errors.Is(err, keyring.ErrNotFound) {
		err = survey.AskOne(
			&survey.Password{Message: fmt.Sprintf("Password for '%v' on '%v' (IDMS %v):", username, host, idms)},
			&password,
			survey.WithValidator(survey.Required),
		)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	kion, err := client.Login(host, idms, username, password)
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

	kion := client.NewWithAppAPIKey(host, keyCfg.Key)
	key, err := kion.RotateAppAPIKey(keyCfg.Key)
	if errors.Is(err, client.ErrUnauthorized) {
		return errors.New("existing key unauthorized; run \"kion key create --force\"")
	} else if err != nil {
		return err
	}
	kion = client.NewWithAppAPIKey(host, key.Key)
	keyMetadata, err := kion.GetAppAPIKeyMetadata(key.ID)
	if err != nil {
		return err
	}

	keyCfg.Key = key.Key
	keyCfg.Created = keyMetadata.Created
	return keyCfg.Save()
}
