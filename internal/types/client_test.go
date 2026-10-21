package types

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"terraform-provider-s4communication/internal/acctest/sapmock"
)

func TestServiceBaseURL(t *testing.T) {
	for host, want := range map[string]string{
		"my000000-api.s4hana.cloud.sap": "https://my000000-api.s4hana.cloud.sap" + string(ServiceCommunicationUser),
		"https://my000000-api.example/": "https://my000000-api.example" + string(ServiceCommunicationUser),
		"http://127.0.0.1:8080":         "http://127.0.0.1:8080" + string(ServiceCommunicationUser),
	} {
		client, err := NewClient(&ClientConfig{Host: host})
		if err != nil {
			t.Fatal(err)
		}
		if got := client.serviceBaseURL(ServiceCommunicationUser); got != want {
			t.Errorf("host %q: got %s, want %s", host, got, want)
		}
	}
}

func getSystems(t *testing.T, config *ClientConfig) (int, error) {
	t.Helper()
	client, err := NewClient(config)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Get(context.Background(), ServiceCommunicationSystem, "CommunicationSystems")
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

func TestClientCertificate(t *testing.T) {
	server := sapmock.NewTLS()
	defer server.Close()
	cert, key := server.ClientCertificate(sapmock.Username)

	status, err := getSystems(t, &ClientConfig{Host: server.URL, ClientCertificate: cert, ClientKey: key, RootCAs: server.RootCAs()})
	if err != nil || status != http.StatusOK {
		t.Errorf("trusted certificate: status %d, error %v", status, err)
	}

	// Basic authentication still works over TLS
	status, err = getSystems(t, &ClientConfig{Host: server.URL, Username: sapmock.Username, Password: sapmock.Password, RootCAs: server.RootCAs()})
	if err != nil || status != http.StatusOK {
		t.Errorf("basic authentication: status %d, error %v", status, err)
	}

	// A certificate for another user is authenticated by TLS but rejected
	otherCert, otherKey := server.ClientCertificate("SOMEONE_ELSE")
	if _, err := getSystems(t, &ClientConfig{Host: server.URL, ClientCertificate: otherCert, ClientKey: otherKey, RootCAs: server.RootCAs()}); err == nil || !strings.Contains(err.Error(), "authentication failed (status 401)") {
		t.Errorf("other user: expected an authentication error, got %v", err)
	}

	// A certificate from an issuer the server does not accept is not sent
	// at all, so the request is unauthenticated
	untrustedCert, untrustedKey := sapmock.UntrustedClientCertificate(sapmock.Username)
	if status, err := getSystems(t, &ClientConfig{Host: server.URL, ClientCertificate: untrustedCert, ClientKey: untrustedKey, RootCAs: server.RootCAs()}); err == nil {
		t.Errorf("untrusted certificate: expected an error, got status %d", status)
	}

	// The server certificate is verified
	if _, err := getSystems(t, &ClientConfig{Host: server.URL, ClientCertificate: cert, ClientKey: key}); err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Errorf("unknown server certificate: expected a verification error, got %v", err)
	}
}

func TestClientCertificateInvalid(t *testing.T) {
	cert, _ := sapmock.UntrustedClientCertificate("A")
	_, otherKey := sapmock.UntrustedClientCertificate("B")
	for name, config := range map[string]*ClientConfig{
		"not PEM":      {Host: "h", ClientCertificate: "nope", ClientKey: "nope"},
		"missing key":  {Host: "h", ClientCertificate: cert},
		"key mismatch": {Host: "h", ClientCertificate: cert, ClientKey: otherKey},
	} {
		if _, err := NewClient(config); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestNoBasicAuthWithCertificate(t *testing.T) {
	cert, key := sapmock.UntrustedClientCertificate("A")
	client, err := NewClient(&ClientConfig{Host: "h", ClientCertificate: cert, ClientKey: key})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://h", nil)
	client.authenticate(req)
	if req.Header.Get("Authorization") != "" {
		t.Error("no Authorization header must be sent when a client certificate is used")
	}
}

func TestReadPEM(t *testing.T) {
	pemText := "-----BEGIN CERTIFICATE-----\nabc\n-----END CERTIFICATE-----\n"
	if got, err := ReadPEM(pemText); err != nil || got != pemText {
		t.Errorf("PEM content: got %q, %v", got, err)
	}
	file := filepath.Join(t.TempDir(), "cert.pem")
	if err := os.WriteFile(file, []byte(pemText), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadPEM(file); err != nil || got != pemText {
		t.Errorf("file path: got %q, %v", got, err)
	}
	if _, err := ReadPEM(filepath.Join(t.TempDir(), "missing.pem")); err == nil {
		t.Error("missing file: expected an error")
	}
}
