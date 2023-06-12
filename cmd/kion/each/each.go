package each

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/corbaltcode/kion/cmd/kion/config"
	"github.com/corbaltcode/kion/cmd/kion/util"
	"github.com/spf13/cobra"
)

func New(cfg *config.Config, keyCfg *config.KeyConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "each command",
		Short: "Runs a command under each of the account-role pairs provided on stdin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg, keyCfg, args)
		},
	}

	cmd.Flags().StringP("shell", "", "/bin/sh", "Shell with which to execute command")

	return cmd
}

func run(cfg *config.Config, keyCfg *config.KeyConfig, args []string) error {
	shell, err := cfg.StringErr("shell")
	if err != nil {
		return err
	}

	kion, err := util.NewClient(cfg, keyCfg)
	if err != nil {
		return err
	}

	type accountRole struct {
		accountID string
		roleName  string
	}
	accountRoles := []accountRole{}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return fmt.Errorf("invalid (needs two fields, account ID and Cloud Access Role): %s", line)
		}
		accountRoles = append(accountRoles, accountRole{fields[0], fields[1]})
	}

	for _, accountRole := range accountRoles {
		creds, err := kion.GetTemporaryCredentialsByCloudAccessRole(accountRole.accountID, accountRole.roleName)
		if err != nil {
			return err
		}

		awsEnv := map[string]string{
			"AWS_ACCESS_KEY_ID":     creds.AccessKeyID,
			"AWS_SECRET_ACCESS_KEY": creds.SecretAccessKey,
			"AWS_SESSION_TOKEN":     creds.SessionToken,
		}

		env := os.Environ()
		for k, v := range awsEnv {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}

		cmd := exec.Cmd{
			Path:   shell,
			Args:   []string{shell, "-c", strings.Join(args, " ")},
			Stdout: os.Stdout,
			Stderr: os.Stderr,
			Env:    env,
		}

		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	return nil
}
