package setup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/corbaltcode/kion/cmd/kion/config"
	"github.com/corbaltcode/kion/cmd/kion/util"
	"github.com/corbaltcode/kion/internal/client"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive setup",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run()
		},
	}

	return cmd
}

func run() error {
	userConfigName, err := config.UserConfigName()
	if err != nil {
		return err
	}
	userConfigExists, err := fileExists(userConfigName)
	if err != nil {
		return err
	}
	if userConfigExists {
		var overwrite bool
		err = survey.AskOne(
			&survey.Confirm{Message: fmt.Sprintf("Config file '%v' exists; overwrite?", userConfigName)},
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
	var kion *client.Client

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

		kion, err = client.Login(host, idms.ID, username, password)
		if errors.Is(err, client.ErrInvalidCredentials) {
			fmt.Println("Invalid credentials")
		} else if err != nil {
			return err
		} else {
			break
		}
	}

	var appAPIKeyAnswer survey.OptionAnswer
	err = survey.AskOne(
		&survey.Select{
			Message: "Create App API Key?",
			Options: []string{"Yes (recommended)", "No (user credentials will be saved in system keyring)"},
		},
		&appAPIKeyAnswer,
	)
	if err != nil {
		return err
	}

	appAPIKey := &client.AppAPIKey{}
	appAPIKeyMetadata := &client.AppAPIKeyMetadata{}

	if appAPIKeyAnswer.Index == 0 {
		appAPIKey, err = kion.CreateAppAPIKey(util.AppAPIKeyName)
		if err != nil {
			return err
		}
		appAPIKeyMetadata, err = kion.GetAppAPIKeyMetadata(appAPIKey.ID)
		if err != nil {
			return err
		}
	} else {
		err = keyring.Set(util.KeyringService(host, idms.ID), username, password)
		if err != nil {
			return err
		}
	}

	var rotateAppAPIKeys bool
	err = survey.AskOne(
		&survey.Confirm{
			Message: "Automatically rotate App API Keys?",
			Default: true,
		},
		&rotateAppAPIKeys,
	)
	if err != nil {
		return err
	}

	var appAPIKeyDuration time.Duration
	err = survey.AskOne(
		&survey.Input{Message: "Duration of App API Keys:", Default: "168h"},
		&appAPIKeyDuration,
		survey.WithValidator(survey.Required),
		survey.WithValidator(validateDuration),
	)
	if err != nil {
		return err
	}

	var sessionDuration time.Duration
	err = survey.AskOne(
		&survey.Input{Message: "Duration of temporary credentials:", Default: "60m"},
		&sessionDuration,
		survey.WithValidator(survey.Required),
		survey.WithValidator(validateDuration),
	)
	if err != nil {
		return err
	}

	settings := map[string]interface{}{
		"app-api-key-duration": appAPIKeyDuration,
		"host":                 host,
		"idms":                 idms.ID,
		"rotate-app-api-keys":  rotateAppAPIKeys,
		"session-duration":     sessionDuration,
		"username":             username,
	}

	userConfigDir := filepath.Dir(userConfigName)
	err = os.MkdirAll(userConfigDir, 0700)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(userConfigName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	err = yaml.NewEncoder(f).Encode(settings)
	if err != nil {
		return err
	}

	keyCfg := config.KeyConfig{
		Key:     appAPIKey.Key,
		Created: appAPIKeyMetadata.Created,
	}
	return keyCfg.Save()
}

func fileExists(name string) (bool, error) {
	_, err := os.Stat(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	return err == nil, nil
}

func validateDuration(t interface{}) error {
	tStr, isStr := t.(string)
	if !isStr {
		return fmt.Errorf("%s is not a string", t)
	}
	_, err := time.ParseDuration(tStr)
	return err
}
