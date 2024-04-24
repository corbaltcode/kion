package access

import (
	"fmt"

	"github.com/corbaltcode/kion/cmd/kion/config"
	"github.com/corbaltcode/kion/cmd/kion/util"
	"github.com/spf13/cobra"
)

func New(cfg *config.Config, keyCfg *config.KeyConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "access",
		Short: "Prints roles and associated accounts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg, keyCfg)
		},
	}

	cmd.Flags().StringP("account", "", "", "filter by account name")
	cmd.Flags().StringP("account-id", "", "", "filter by account ID")
	cmd.Flags().StringP("cloud-access-role", "r", "", "filter by cloud access role")

	return cmd
}

func run(cfg *config.Config, keyCfg *config.KeyConfig) error {
	account := cfg.String("account")
	accountID := cfg.String("account-id")
	role := cfg.String("cloud-access-role")

	kion, err := util.NewClient(cfg, keyCfg)
	if err != nil {
		return err
	}

	acars, err := kion.GetAccountCloudAccessRoles()
	if err != nil {
		return err
	}

	for _, acar := range acars {
		if account != "" && account != acar.AccountName {
			continue
		}
		if accountID != "" && accountID != acar.AccountID {
			continue
		}
		if role != "" && role != acar.CloudAccessRole {
			continue
		}

		fmt.Printf("%v\t%v\t%v\n", acar.CloudAccessRole, acar.AccountID, acar.AccountName)
	}

	return nil
}
