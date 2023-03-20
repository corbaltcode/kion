package console

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/corbaltcode/kion/cmd/kion/util"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

func New(cfg *util.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "console",
		Short: "Opens the AWS console",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg)
		},
	}

	cmd.Flags().StringP("account-id", "", "", "AWS account ID")
	cmd.Flags().StringP("cloud-access-role", "", "", "cloud access role")
	cmd.Flags().BoolP("print", "p", false, "print URL instead of opening a browser")
	cmd.Flags().StringP("region", "", "", "AWS region")
	cmd.Flags().StringP("session-duration", "", "1h", "duration of temporary credentials")

	return cmd
}

// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_enable-console-custom-url.html
func run(cfg *util.Config) error {
	accountID, err := cfg.StringErr("account-id")
	if err != nil {
		return err
	}
	cloudAccessRole, err := cfg.StringErr("cloud-access-role")
	if err != nil {
		return err
	}
	host, err := cfg.StringErr("host")
	if err != nil {
		return err
	}
	region, err := cfg.StringErr("region")
	if err != nil {
		return err
	}

	kion, err := util.NewClient(cfg)
	if err != nil {
		return err
	}
	creds, err := kion.GetTemporaryCredentialsByCloudAccessRole(accountID, cloudAccessRole)
	if err != nil {
		return err
	}

	signinToken, err := getAWSSigninToken(creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken)
	if err != nil {
		return err
	}

	v := url.Values{}
	v.Add("Action", "login")
	v.Add("Issuer", fmt.Sprintf("https://%s/login", host))
	v.Add("Destination", fmt.Sprintf("https://%s.console.aws.amazon.com", region))
	v.Add("SigninToken", signinToken)
	signinUrl := "https://signin.aws.amazon.com/federation?" + v.Encode()

	if cfg.Bool("print") {
		fmt.Println(signinUrl)
	} else {
		err = browser.OpenURL(signinUrl)
		if err != nil {
			return err
		}
	}

	return nil
}

func getAWSSigninToken(accessKeyID string, secretAccessKey string, sessionToken string) (string, error) {
	session := map[string]string{
		"sessionId":    accessKeyID,
		"sessionKey":   secretAccessKey,
		"sessionToken": sessionToken,
	}
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return "", err
	}

	v := url.Values{}
	v.Add("Action", "getSigninToken")
	v.Add("Session", string(sessionJSON))
	url := "https://signin.aws.amazon.com/federation?" + v.Encode()

	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errors.New(resp.Status)
	}

	out := struct {
		SigninToken string
	}{}

	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		return "", err
	}

	return out.SigninToken, nil
}
