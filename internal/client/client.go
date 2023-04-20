package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

var ErrInvalidCredentials = errors.New("kion: invalid credentials")
var ErrUnauthorized = errors.New("kion: unauthorized")

type Client struct {
	Host        string
	accessToken string
}

type IDMS struct {
	ID   int
	Name string
}

type TemporaryCredentials struct {
	AccessKeyID     string `json:"access_key"`
	SecretAccessKey string `json:"secret_access_key"`
	SessionToken    string `json:"session_token"`
}

func NewWithAppAPIKey(host string, key string) *Client {
	return &Client{
		Host:        host,
		accessToken: key,
	}
}

// TODO: how to use the refresh token (currently dropped)?
func Login(host string, idms int, username string, password string) (*Client, error) {
	req := map[string]interface{}{
		"idms":     idms,
		"username": username,
		"password": password,
	}
	resp := struct {
		Access struct {
			Token string
		}
	}{}

	err := do(http.MethodPost, host, "", "v3/token", req, &resp)
	if err != nil {
		return nil, err
	}

	client := Client{
		Host:        host,
		accessToken: resp.Access.Token,
	}
	return &client, nil
}

func GetIDMSs(host string) ([]IDMS, error) {
	resp := []IDMS{}

	err := do(http.MethodGet, host, "", "v2/idms", nil, &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *Client) GetTemporaryCredentialsByIAMRole(accountID string, iamRole string) (*TemporaryCredentials, error) {
	req := map[string]interface{}{
		"account_number": accountID,
		"iam_role_name":  iamRole,
	}
	resp := TemporaryCredentials{}

	err := c.do(http.MethodPost, "v3/temporary-credentials", req, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *Client) GetTemporaryCredentialsByCloudAccessRole(accountID string, cloudAcccessRole string) (*TemporaryCredentials, error) {
	req := map[string]interface{}{
		"account_number":         accountID,
		"cloud_access_role_name": cloudAcccessRole,
	}
	resp := TemporaryCredentials{}

	err := c.do(http.MethodPost, "v3/temporary-credentials/cloud-access-role", req, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

type kionResponse struct {
	Status  int
	Message string
	Data    interface{}
}

func (r kionResponse) Error() string {
	return fmt.Sprintf("kion: %v (%v)", r.Message, r.Status)
}

func (c *Client) do(method string, path string, data interface{}, out interface{}) error {
	return do(method, c.Host, c.accessToken, path, data, out)
}

func do(method string, host string, accessToken string, path string, data interface{}, out interface{}) error {
	u := url.URL{
		Scheme: "https",
		Host:   host,
		Path:   "api/" + path,
	}

	var dataJSON []byte

	var err error
	if data != nil {
		dataJSON, err = json.Marshal(data)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequest(method, u.String(), bytes.NewReader(dataJSON))
	if err != nil {
		return err
	}
	if accessToken != "" {
		req.Header.Add("Authorization", "Bearer "+accessToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	kionResp := kionResponse{Data: out}

	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&kionResp)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if kionResp.Status == 400 && kionResp.Message == "Invalid username or password." {
			return ErrInvalidCredentials
		} else if kionResp.Status == 401 {
			return ErrUnauthorized
		} else {
			return kionResp
		}
	}

	return nil
}
