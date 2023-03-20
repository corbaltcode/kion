package client

import (
	"fmt"
	"os"
	"strconv"
	"testing"
)

var host string
var idms int
var username string
var password string

func TestMain(m *testing.M) {
	host = mustGetenv("KION_HOST")
	idms = mustGetenvInt("KION_IDMS")
	username = mustGetenv("KION_USERNAME")
	password = mustGetenv("KION_PASSWORD")

	m.Run()
}

func TestLogin(t *testing.T) {
	login(t)
}

func TestInvalidCredentials(t *testing.T) {
	_, err := Login(host, idms, "bad-user", "bad-pass")
	if err != ErrInvalidCredentials {
		t.Fatalf("got error %v (want ErrInvalidCredentials)", err)
	}
}

func login(t *testing.T) *Client {
	c, err := Login(host, idms, username, password)
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
