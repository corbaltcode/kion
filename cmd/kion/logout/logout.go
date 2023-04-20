package logout

import (
	"github.com/corbaltcode/kion/cmd/kion/config"
	"github.com/corbaltcode/kion/cmd/kion/util"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
)

func New(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Removes credentials from system keyring",
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

	return keyring.Delete(util.KeyringService(host, idms), username)
}
