package credentials

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/corbaltcode/kion/cmd/kion/config"
	"github.com/corbaltcode/kion/cmd/kion/util"
	"github.com/spf13/cobra"
)

func New(cfg *config.Config, keyCfg *config.KeyConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "credentials",
		Aliases: []string{"creds"},
		Short:   "Prints temporary credentials",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg, keyCfg)
		},
	}

	cmd.Flags().StringP("account-id", "", "", "AWS account ID")
	cmd.Flags().StringP("cloud-access-role", "", "", "cloud access role")
	cmd.Flags().StringP("format", "f", "aws", "format (aws, export, or json)")

	return cmd
}

func run(cfg *config.Config, keyCfg *config.KeyConfig) error {
	accountID, err := cfg.StringErr("account-id")
	if err != nil {
		return err
	}
	cloudAccessRole, err := cfg.StringErr("cloud-access-role")
	if err != nil {
		return err
	}
	format, err := cfg.StringErr("format")
	if err != nil {
		return err
	}
	if format != "aws" && format != "export" && format != "json" {
		return fmt.Errorf("invalid format: %v", format)
	}

	kion, err := util.NewClient(cfg, keyCfg)
	if err != nil {
		return err
	}
	creds, err := kion.GetTemporaryCredentialsByCloudAccessRole(accountID, cloudAccessRole)
	if err != nil {
		return err
	}

	switch format {
	case "aws":
		fmt.Printf("aws_access_key_id = %v\n", creds.AccessKeyID)
		fmt.Printf("aws_secret_access_key = %v\n", creds.SecretAccessKey)
		fmt.Printf("aws_session_token = %v\n", creds.SessionToken)
	case "export":
		fmt.Printf("export AWS_ACCESS_KEY_ID=%v\n", creds.AccessKeyID)
		fmt.Printf("export AWS_SECRET_ACCESS_KEY=%v\n", creds.SecretAccessKey)
		fmt.Printf("export AWS_SESSION_TOKEN=%v\n", creds.SessionToken)
	case "json":
		json.NewEncoder(os.Stdout).Encode(creds)
	default:
		panic(fmt.Sprintf("unexpected format: %v", format))
	}

	return nil
}
