package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/relvacode/iso8601"
)

var ErrAppAPIKeyExpired = errors.New("kion: app API key expired")
var ErrInvalidCredentials = errors.New("kion: invalid credentials")
var ErrUnauthorized = errors.New("kion: unauthorized")

type Client struct {
	Host        string
	accessToken *accessToken
}

type AppAPIKey struct {
	ID  int
	Key string
}

type AppAPIKeyMetadata struct {
	ID      int
	Created time.Time
}

type CloudAccessRole struct {
	ID            int
	AccountID     int    `json:"account_id"`
	AccountNumber string `json:"account_number"`
	Name          string
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

type accessToken struct {
	Token       string
	Expiry      time.Time
	IsAppAPIKey bool
}

func (t *accessToken) IsExpired() bool {
	return !t.Expiry.IsZero() && time.Now().After(t.Expiry)
}

// NewWithAppAPIKey creates a Client that authenticates with an App API Key.
// expiry allows the Client to generate an error if it is used after the key has
// expired. A zero expiry (time.Time{}) means the key doesn't expire.
func NewWithAppAPIKey(host string, key string, expiry time.Time) *Client {
	return &Client{
		Host: host,
		accessToken: &accessToken{
			Token:       key,
			Expiry:      expiry,
			IsAppAPIKey: true,
		},
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

	err := do(http.MethodPost, host, nil, "v3/token", req, &resp)
	if err != nil {
		return nil, err
	}

	client := Client{
		Host: host,
		accessToken: &accessToken{
			Token:       resp.Access.Token,
			Expiry:      time.Time{},
			IsAppAPIKey: false,
		},
	}
	return &client, nil
}

func GetIDMSs(host string) ([]IDMS, error) {
	resp := []IDMS{}

	err := do(http.MethodGet, host, nil, "v2/idms", nil, &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *Client) CreateAppAPIKey(name string) (*AppAPIKey, error) {
	req := map[string]interface{}{
		"name": name,
	}
	resp := AppAPIKey{}

	err := c.do(http.MethodPost, "v3/app-api-key", req, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *Client) RotateAppAPIKey(key string) (*AppAPIKey, error) {
	req := map[string]interface{}{
		"key": key,
	}
	resp := AppAPIKey{}

	err := c.do(http.MethodPost, "v3/app-api-key/rotate", req, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *Client) GetAppAPIKeyMetadata(id int) (*AppAPIKeyMetadata, error) {
	resp := struct {
		ID             int
		CreatedISO8601 string `json:"created_at"`
	}{}

	err := c.do(http.MethodGet, fmt.Sprintf("v3/app-api-key/%d", id), nil, &resp)
	if err != nil {
		return nil, err
	}

	created, err := iso8601.ParseString(resp.CreatedISO8601)
	if err != nil {
		return nil, err
	}

	return &AppAPIKeyMetadata{
		ID:      resp.ID,
		Created: created,
	}, nil
}

func (c *Client) GetCloudAccessRoles() ([]CloudAccessRole, error) {
	resp := []CloudAccessRole{}

	err := c.do(http.MethodGet, "v3/me/cloud-access-role", nil, &resp)
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

func do(method string, host string, accessToken *accessToken, path string, data interface{}, out interface{}) error {
	if accessToken != nil && accessToken.IsAppAPIKey && accessToken.IsExpired() {
		return ErrAppAPIKeyExpired
	}

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
	if accessToken != nil {
		req.Header.Add("Authorization", "Bearer "+accessToken.Token)
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
