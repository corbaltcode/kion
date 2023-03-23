package setup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/corbaltcode/kion/cmd/kion/util"
	"github.com/corbaltcode/kion/internal/client"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

func New(userConfigPath string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive setup",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(userConfigPath)
		},
	}

	return cmd
}

func run(userConfigPath string) error {
	userConfigExists, err := fileExists(userConfigPath)
	if err != nil {
		return err
	}
	if userConfigExists {
		var overwrite bool
		err = survey.AskOne(
			&survey.Confirm{Message: fmt.Sprintf("Config file '%v' exists; overwrite?", userConfigPath)},
			&overwrite,
		)
		if err != nil {
			return err
		}
		if !overwrite {
			return nil
		}
	}

	var host string
	err = survey.AskOne(
		&survey.Input{Message: "Kion host:"},
		&host,
		survey.WithValidator(survey.Required),
	)
	if err != nil {
		return err
	}

	idmss, err := client.GetIDMSs(host)
	if err != nil {
		return err
	}
	if len(idmss) < 1 {
		return fmt.Errorf("empty IDMS list")
	}

	idmsNames := []string{}
	for _, idms := range idmss {
		idmsNames = append(idmsNames, idms.Name)
	}

	var idmsAnswer survey.OptionAnswer
	err = survey.AskOne(
		&survey.Select{Message: "ID Management System:", Options: idmsNames},
		&idmsAnswer,
	)
	if err != nil {
		return err
	}
	idms := idmss[idmsAnswer.Index]

	var username string
	var password string

	for {
		err = survey.AskOne(
			&survey.Input{Message: "Username:"},
			&username,
			survey.WithValidator(survey.Required),
		)
		if err != nil {
			return err
		}

		err = survey.AskOne(
			&survey.Password{Message: "Password:"},
			&password,
			survey.WithValidator(survey.Required),
		)
		if err != nil {
			return err
		}

		_, err = client.Login(host, idms.ID, username, password)
		if errors.Is(err, client.ErrInvalidCredentials) {
			fmt.Println("Invalid credentials")
		} else if err != nil {
			return err
		} else {
			break
		}
	}

	err = keyring.Set(util.KeyringService(host, idms.ID), username, password)
	if err != nil {
		return err
	}

	var sessionDuration time.Duration
	err = survey.AskOne(
		&survey.Input{Message: "Duration of temporary credentials:", Default: "60m"},
		&sessionDuration,
		survey.WithValidator(survey.Required),
		survey.WithValidator(func(t interface{}) error {
			tStr, isStr := t.(string)
			if !isStr {
				return fmt.Errorf("%s is not a string", t)
			}
			_, err = time.ParseDuration(tStr)
			return err
		}),
	)
	if err != nil {
		return err
	}

	var region string
	err = survey.AskOne(
		&survey.Input{Message: "Default region:", Default: "us-east-1"},
		&region,
		survey.WithValidator(survey.Required),
	)
	if err != nil {
		return err
	}

	settings := map[string]interface{}{
		"host":             host,
		"idms":             idms.ID,
		"region":           region,
		"session-duration": sessionDuration,
		"username":         username,
	}

	f, err := os.OpenFile(userConfigPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	return yaml.NewEncoder(f).Encode(settings)
}

func fileExists(name string) (bool, error) {
	_, err := os.Stat(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	return err == nil, nil
}
