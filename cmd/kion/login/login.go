package login

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

func New(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Saves credentials to system keyring",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg)
		},
	}

	return cmd
}

func run(cfg *config.Config) error {
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

	var password string

	for {
		err = survey.AskOne(
			&survey.Password{Message: fmt.Sprintf("Password for '%v' on '%v' (IDMS %v):", username, host, idms)},
			&password,
			survey.WithValidator(survey.Required),
		)
		if err != nil {
			return err
		}

		_, err := client.Login(host, idms, username, password)

		if errors.Is(err, client.ErrInvalidCredentials) {
			fmt.Println("Invalid credentials")
		} else if err != nil {
			return err
		} else {
			break
		}
	}

	return keyring.Set(util.KeyringService(host, idms), username, password)
}
