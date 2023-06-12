package roles

import (
	"fmt"

	"github.com/corbaltcode/kion/cmd/kion/config"
	"github.com/corbaltcode/kion/cmd/kion/util"
	"github.com/spf13/cobra"
)

func New(cfg *config.Config, keyCfg *config.KeyConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "roles",
		Short: "Prints accounts and Cloud Access Roles for the current user",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg, keyCfg)
		},
	}
}

func run(cfg *config.Config, keyCfg *config.KeyConfig) error {
	kion, err := util.NewClient(cfg, keyCfg)
	if err != nil {
		return err
	}

	roles, err := kion.GetCloudAccessRoles()
	if err != nil {
		return err
	}

	for _, role := range roles {
		fmt.Printf("%s\t%s\n", role.AccountNumber, role.Name)
	}

	return nil
}
