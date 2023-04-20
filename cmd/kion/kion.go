package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/corbaltcode/kion/cmd/kion/config"
	"github.com/corbaltcode/kion/cmd/kion/console"
	"github.com/corbaltcode/kion/cmd/kion/credentialprocess"
	"github.com/corbaltcode/kion/cmd/kion/credentials"
	"github.com/corbaltcode/kion/cmd/kion/login"
	"github.com/corbaltcode/kion/cmd/kion/logout"
	"github.com/corbaltcode/kion/cmd/kion/setup"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/cobra"
)

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	userConfigDir := filepath.Join(homeDir, ".config", "kion")
	userConfigPath := filepath.Join(userConfigDir, "config.yml")
	configPaths := []string{
		userConfigPath,
		filepath.Join(".", "kion.yml"),
	}

	k := koanf.New(".")

	for _, path := range configPaths {
		err = k.Load(file.Provider(path), yaml.Parser())
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "bad config in %v: %v\n", path, err)
			os.Exit(1)
		}
	}

	rootCmd := &cobra.Command{
		Use:  "kion",
		Args: cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return k.Load(posflag.Provider(cmd.Flags(), ".", k), nil)
		},
		SilenceErrors: true,
		SilenceUsage:  true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	cfg := &config.Config{Koanf: k}

	rootCmd.AddCommand(credentialprocess.New(cfg, userConfigDir))
	rootCmd.AddCommand(credentials.New(cfg))
	rootCmd.AddCommand(console.New(cfg))
	rootCmd.AddCommand(login.New(cfg))
	rootCmd.AddCommand(logout.New(cfg))
	rootCmd.AddCommand(setup.New(userConfigPath))

	err = rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
