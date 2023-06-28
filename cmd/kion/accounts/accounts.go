package accounts

import (
	"fmt"

	"github.com/corbaltcode/kion/cmd/kion/config"
	"github.com/corbaltcode/kion/cmd/kion/util"
	"github.com/spf13/cobra"
)

func New(cfg *config.Config, keyCfg *config.KeyConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:  "accounts",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg, keyCfg)
		},
	}

	return cmd
}

func run(cfg *config.Config, keyCfg *config.KeyConfig) error {
	kion, err := util.NewClient(cfg, keyCfg)
	if err != nil {
		return err
	}

	accounts, err := kion.GetAccounts()
	if err != nil {
		return err
	}

	for _, account := range accounts {
		fmt.Printf("%s\t%s\n", account.AccountID, account.Name)
	}

	return nil
}
