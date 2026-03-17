package saml

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/browser"
	saml2 "github.com/russellhaering/gosaml2"
	samlTypes "github.com/russellhaering/gosaml2/types"
	dsig "github.com/russellhaering/goxmldsig"
)

// callbackPort is the localhost port used to receive the SAML assertion from the IDP.
// This must match the ACS URL registered with the IDP for this SP.
const callbackPort = 8400

const authPage = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <title>Kion</title>
    <style>
      html { background: #f3f7f4; }
      body { display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; }
      #wrapper { text-align: center; font-family: monospace, monospace; }
    </style>
  </head>
  <body>
    <div id="wrapper">
      <p>Authentication successful. You may close this window.</p>
      <script type="text/javascript">window.close()</script>
    </div>
  </body>
</html>`

type csrfResponse struct {
	Data string `json:"data"`
}

type ssoAuthResponse struct {
	Data struct {
		Access struct {
			Token string `json:"token"`
		} `json:"access"`
	} `json:"data"`
}

type callbackResult struct {
	token string
	err   error
}

// Metadata loads SAML IDP metadata from a URL or local file path.
func Metadata(source string) (*samlTypes.EntityDescriptor, error) {
	if source == "" {
		return nil, errors.New("saml metadata source is empty")
	}
	var rawMetadata []byte
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		res, err := http.Get(source) //nolint:noctx
		if err != nil {
			return nil, fmt.Errorf("failed to download SAML metadata from %q: %w", source, err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to download SAML metadata from %q: HTTP %d", source, res.StatusCode)
		}
		rawMetadata, err = io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading SAML metadata from %q: %w", source, err)
		}
	} else {
		var err error
		rawMetadata, err = os.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("error reading SAML metadata file %q: %w", source, err)
		}
	}

	if len(rawMetadata) == 0 {
		return nil, fmt.Errorf("SAML metadata from %q is empty", source)
	}

	metadata := &samlTypes.EntityDescriptor{}
	if err := xml.Unmarshal(rawMetadata, metadata); err != nil {
		return nil, fmt.Errorf("error parsing SAML metadata XML from %q: %w", source, err)
	}
	return metadata, nil
}

func validateMetadata(metadata *samlTypes.EntityDescriptor) error {
	if metadata == nil {
		return errors.New("SAML metadata is nil")
	}
	if metadata.EntityID == "" {
		return errors.New("SAML metadata is missing EntityID")
	}
	if metadata.IDPSSODescriptor == nil {
		return errors.New("SAML metadata is missing IDPSSODescriptor; verify you are using IDP metadata (not SP metadata)")
	}
	if len(metadata.IDPSSODescriptor.SingleSignOnServices) == 0 {
		return errors.New("SAML metadata IDPSSODescriptor has no SingleSignOnServices defined")
	}
	if metadata.IDPSSODescriptor.SingleSignOnServices[0].Location == "" {
		return errors.New("SAML metadata SingleSignOnService Location is empty")
	}
	if len(metadata.IDPSSODescriptor.KeyDescriptors) == 0 {
		return errors.New("SAML metadata has no KeyDescriptors")
	}
	for _, kd := range metadata.IDPSSODescriptor.KeyDescriptors {
		for _, xcert := range kd.KeyInfo.X509Data.X509Certificates {
			if xcert.Data != "" {
				return nil
			}
		}
	}
	return errors.New("SAML metadata KeyDescriptors contain no valid X509 certificates")
}

// Authenticate performs a SAML SP-initiated authentication flow against the given Kion host.
// It opens the system browser to the IDP, receives the SAML assertion on localhost:8400
// (which must be registered as the ACS URL with the IDP), forwards it to Kion, and
// exchanges the resulting SSO code for a bearer token.
// Set printURL to true to print the auth URL instead of opening a browser.
func Authenticate(host string, metadata *samlTypes.EntityDescriptor, spIssuer string, printURL bool) (string, error) {
	if host == "" {
		return "", errors.New("kion host is required")
	}
	if spIssuer == "" {
		return "", errors.New("SAML SP issuer is required")
	}
	if err := validateMetadata(metadata); err != nil {
		return "", fmt.Errorf("SAML metadata validation failed: %w", err)
	}

	certStore := dsig.MemoryX509CertificateStore{Roots: []*x509.Certificate{}}
	for _, kd := range metadata.IDPSSODescriptor.KeyDescriptors {
		for idx, xcert := range kd.KeyInfo.X509Data.X509Certificates {
			if xcert.Data == "" {
				return "", fmt.Errorf("metadata certificate %d is empty", idx)
			}
			certData, err := base64.StdEncoding.DecodeString(xcert.Data)
			if err != nil {
				return "", err
			}
			idpCert, err := x509.ParseCertificate(certData)
			if err != nil {
				return "", err
			}
			certStore.Roots = append(certStore.Roots, idpCert)
		}
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", callbackPort))
	if err != nil {
		return "", fmt.Errorf("failed to bind SAML callback listener on port %d (is something else using it?): %w", callbackPort, err)
	}

	sp := &saml2.SAMLServiceProvider{
		IdentityProviderSSOURL:      metadata.IDPSSODescriptor.SingleSignOnServices[0].Location,
		IdentityProviderIssuer:      metadata.EntityID,
		ServiceProviderIssuer:       spIssuer,
		AssertionConsumerServiceURL: fmt.Sprintf("http://localhost:%d/callback", callbackPort),
		SignAuthnRequests:           false,
		IDPCertificateStore:         &certStore,
		SPKeyStore:                  dsig.RandomKeyStoreForTest(),
	}

	resultChan := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.URL.String(), "/favicon.ico") {
			http.NotFound(rw, req)
			return
		}
		if req.Method == http.MethodOptions {
			return
		}

		b, err := io.ReadAll(req.Body)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			resultChan <- callbackResult{err: fmt.Errorf("bad SAML callback request: %w", err)}
			return
		}

		httpClient := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		csrfToken, csrfCookies, err := getCSRFToken("https://"+host, httpClient)
		if err != nil {
			resultChan <- callbackResult{err: fmt.Errorf("error getting CSRF token: %w", err)}
			return
		}

		jar, err := cookiejar.New(nil)
		if err != nil {
			resultChan <- callbackResult{err: fmt.Errorf("failed to create cookie jar: %w", err)}
			return
		}
		u, err := url.Parse("https://" + host)
		if err != nil {
			resultChan <- callbackResult{err: fmt.Errorf("failed to parse kion URL: %w", err)}
			return
		}
		jar.SetCookies(u, csrfCookies)
		httpClient.Jar = jar

		r, err := http.NewRequest(http.MethodPost, "https://"+host+"/api/v1/saml/callback", bytes.NewReader(b))
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			resultChan <- callbackResult{err: fmt.Errorf("error creating SAML callback request: %w", err)}
			return
		}
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		resp, err := httpClient.Do(r)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			resultChan <- callbackResult{err: fmt.Errorf("error posting SAML assertion: %w", err)}
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			resultChan <- callbackResult{err: fmt.Errorf("error reading SAML callback response: %w", err)}
			return
		}

		ssoCodeRegexp := regexp.MustCompile(`code=(.+)">`)
		groups := ssoCodeRegexp.FindStringSubmatch(string(body))
		if len(groups) < 2 {
			resultChan <- callbackResult{err: fmt.Errorf("could not find SSO code in SAML response (response: %s)", truncate(string(body), 300))}
			return
		}

		authToken, err := getAuthToken("https://"+host, groups[1], csrfToken, httpClient)
		if err != nil {
			resultChan <- callbackResult{err: fmt.Errorf("failed to exchange SSO code for auth token: %w", err)}
			return
		}

		rw.Write([]byte(authPage)) //nolint:errcheck
		resultChan <- callbackResult{token: authToken}
	})

	server := &http.Server{Handler: mux}

	return runAuth(sp, server, listener, resultChan, printURL)
}

func runAuth(sp *saml2.SAMLServiceProvider, server *http.Server, listener net.Listener, resultChan chan callbackResult, printURL bool) (string, error) {
	authURL, err := sp.BuildAuthURL("")
	if err != nil {
		return "", fmt.Errorf("failed to build SAML auth URL: %w", err)
	}

	timeout := 60 * time.Second
	if printURL {
		fmt.Printf("Please open the following URL in your browser to authenticate:\n\n%s\n\n", authURL)
		timeout = 180 * time.Second
	} else {
		browser.Stdout = os.Stderr
		if err := browser.OpenURL(authURL); err != nil {
			fmt.Printf("Could not open browser automatically. Please open the following URL:\n\n%s\n\n", authURL)
		}
	}

	timer := time.NewTimer(timeout)
	go func() {
		select {
		case result := <-resultChan:
			timer.Stop()
			server.Shutdown(context.Background()) //nolint:errcheck
			resultChan <- result
		case <-timer.C:
			server.Shutdown(context.Background()) //nolint:errcheck
			resultChan <- callbackResult{err: errors.New("authentication timed out")}
		}
	}()

	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		return "", fmt.Errorf("SAML callback server error: %w", err)
	}

	result := <-resultChan
	return result.token, result.err
}

func getCSRFToken(appURL string, client *http.Client) (string, []*http.Cookie, error) {
	req, err := http.NewRequest(http.MethodGet, appURL+"/api/v2/csrf-token", nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create CSRF token request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to request CSRF token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("CSRF token request failed with HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read CSRF token response: %w", err)
	}
	var data csrfResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return "", nil, fmt.Errorf("failed to parse CSRF token response: %w", err)
	}
	if data.Data == "" {
		return "", nil, errors.New("CSRF token response contained empty token")
	}
	return data.Data, resp.Cookies(), nil
}

func getAuthToken(appURL string, ssoCode string, csrfToken string, client *http.Client) (string, error) {
	req, err := http.NewRequest(http.MethodGet, appURL+"/api/v2/login/sso-provider?code="+ssoCode, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create auth token request: %w", err)
	}
	req.Header.Set("X-Csrf-Token", csrfToken)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to exchange SSO code for auth token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read auth token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth token request failed with HTTP %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var data ssoAuthResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("failed to parse auth token response: %w", err)
	}
	if data.Data.Access.Token == "" {
		return "", errors.New("auth token response contained empty token")
	}
	return data.Data.Access.Token, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
