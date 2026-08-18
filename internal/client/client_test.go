package client

import (
	"fmt"
	"os"
	"strconv"
	"testing"
)

var host string
var idms int
var authMethod string
var username string
var password string
var samlMetadataFile string
var samlSPIssuer string

func TestMain(m *testing.M) {
	host = mustGetenv("KION_HOST")
	idms = mustGetenvInt("KION_IDMS")
	authMethod = mustGetenv("KION_AUTHMETHOD")

	switch authMethod {
	case "password":
		username = mustGetenv("KION_USERNAME")
		password = mustGetenv("KION_PASSWORD")
	case "saml":
		samlMetadataFile = mustGetenv("KION_SAML_METADATA_FILE")
		samlSPIssuer = mustGetenv("KION_SAML_SP_ISSUER")
	default:
		panic(fmt.Sprintf("invalid KION_AUTHMETHOD: %v", authMethod))
	}

	m.Run()
}

func TestLogin(t *testing.T) {
	if authMethod == "saml" {
		_, err := Login(authMethod, host, idms, "", "", samlMetadataFile, samlSPIssuer, false, true)
		if err != nil {
			t.Fatalf("SAML login failed: %v", err)
		}
		return
	}

	login(t)
}

func TestInvalidCredentials(t *testing.T) {
	if authMethod == "saml" {
		t.Skip("invalid username/password credentials do not apply to SAML authentication")
	}

	_, err := Login(authMethod, host, idms, "bad-user", "bad-pass", "", "", false, false)
	if err != ErrInvalidCredentials {
		t.Fatalf("got error %v (want ErrInvalidCredentials)", err)
	}
}

func login(t *testing.T) *Client {
	c, err := Login(authMethod, host, idms, username, password, "", "", false, false)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	return c
}

func mustGetenv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		panic(fmt.Sprintf("missing env var: %v", key))
	}
	return v
}

func mustGetenvInt(key string) int {
	v, err := strconv.Atoi(mustGetenv(key))
	if err != nil {
		panic(fmt.Sprintf("env var not int: %v", key))
	}
	return v
}
