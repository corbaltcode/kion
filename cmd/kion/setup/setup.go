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

	var authMethodAnswer survey.OptionAnswer
	if err = survey.AskOne(
		&survey.Select{
			Message: "Authentication method:",
			Options: []string{"SAML (browser)", "Username/password"},
		},
		&authMethodAnswer,
	); err != nil {
		return err
	}

	var idmsID int
	var username string
	var password string
	var kion *client.Client
	var samlMetadataFile string
	var samlServiceProviderIssuer string
	var samlPrintURL bool
	authMethod := "password"

	if authMethodAnswer.Index == 0 {
		authMethod = "saml"

		if err = survey.AskOne(
			&survey.Input{Message: "IDMS ID:"},
			&idmsID,
			survey.WithValidator(survey.Required),
		); err != nil {
			return err
		}
		if idmsID < 1 {
			return errors.New("IDMS ID must be greater than zero")
		}

		if err = survey.AskOne(
			&survey.Input{
				Message: "SAML metadata URL or file:",
			},
			&samlMetadataFile,
			survey.WithValidator(survey.Required),
		); err != nil {
			return err
		}

		if err = survey.AskOne(
			&survey.Input{
				Message: "SAML service provider issuer:",
			},
			&samlServiceProviderIssuer,
			survey.WithValidator(survey.Required),
		); err != nil {
			return err
		}

		if err = survey.AskOne(
			&survey.Confirm{
				Message: "Print the SAML URL instead of opening a browser?",
				Default: false,
			},
			&samlPrintURL,
		); err != nil {
			return err
		}

		if kion, err = client.Login(authMethod, host, idmsID, "", "", samlMetadataFile, samlServiceProviderIssuer, samlPrintURL, true); err != nil {
			return err
		}
	} else {
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
		idmsID = idmss[idmsAnswer.Index].ID

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

			kion, err = client.Login(authMethod, host, idmsID, username, password, "", "", false, false)
			if errors.Is(err, client.ErrInvalidCredentials) {
				fmt.Println("Invalid credentials")
			} else if err != nil {
				return err
			} else {
				break
			}
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
	} else if authMethodAnswer.Index != 0 {
		err = keyring.Set(util.KeyringService(host, idmsID), username, password)
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
		"auth-method":          authMethod,
		"host":                 host,
		"idms":                 idmsID,
		"rotate-app-api-keys":  rotateAppAPIKeys,
		"session-duration":     sessionDuration,
	}
	if authMethod == "saml" {
		settings["saml-metadata-file"] = samlMetadataFile
		settings["saml-sp-issuer"] = samlServiceProviderIssuer
		settings["saml-print-url"] = samlPrintURL
	} else {
		settings["username"] = username
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
