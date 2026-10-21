package provider

import (
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-s4communication/internal/acctest/sapmock"
)

// tlsProviderFactories returns provider factories that trust the server
// certificate of a TLS mock.
func tlsProviderFactories(rootCAs *x509.CertPool) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"s4communication": providerserver.NewProtocol6WithError(&S4CommunicationProvider{version: "test", rootCAs: rootCAs}),
	}
}

// clearAuthEnv makes sure credentials from the environment do not leak into
// tests that check the configuration.
func clearAuthEnv(t *testing.T) {
	for _, name := range []string{"S4_HOST", "S4_USERNAME", "S4_PASSWORD", "S4_CLIENT_CERTIFICATE", "S4_CLIENT_KEY"} {
		t.Setenv(name, "")
	}
}

const systemConfig = `
resource "s4communication_communication_system" "test" {
  communication_system_id = "MTLS_SYSTEM"
  name                    = "mTLS System"
}
`

func TestClientCertificateAuthentication(t *testing.T) {
	clearAuthEnv(t)
	server := sapmock.NewTLS()
	defer server.Close()
	cert, key := server.ClientCertificate(sapmock.Username)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: tlsProviderFactories(server.RootCAs()),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "s4communication" {
  host               = %q
  client_certificate = %q
  client_key         = %q
}
`, server.URL, cert, key) + systemConfig,
				Check: resource.TestCheckResourceAttrSet("s4communication_communication_system.test", "id"),
			},
			{
				ResourceName:      "s4communication_communication_system.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestClientCertificateFromEnvironmentFiles(t *testing.T) {
	clearAuthEnv(t)
	server := sapmock.NewTLS()
	defer server.Close()
	cert, key := server.ClientCertificate(sapmock.Username)

	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certFile, []byte(cert), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, []byte(key), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("S4_HOST", server.URL)
	t.Setenv("S4_CLIENT_CERTIFICATE", certFile)
	t.Setenv("S4_CLIENT_KEY", keyFile)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: tlsProviderFactories(server.RootCAs()),
		Steps: []resource.TestStep{
			{
				Config: `provider "s4communication" {}` + systemConfig,
				Check:  resource.TestCheckResourceAttrSet("s4communication_communication_system.test", "id"),
			},
		},
	})
}

func TestClientCertificateRejected(t *testing.T) {
	clearAuthEnv(t)
	server := sapmock.NewTLS()
	defer server.Close()
	cert, key := server.ClientCertificate("SOMEONE_ELSE")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: tlsProviderFactories(server.RootCAs()),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "s4communication" {
  host               = %q
  client_certificate = %q
  client_key         = %q
}
`, server.URL, cert, key) + systemConfig,
				ExpectError: regexp.MustCompile(`authentication failed\s+\(status 401\)`),
			},
		},
	})
}

func TestAuthenticationConfigurationErrors(t *testing.T) {
	clearAuthEnv(t)
	cert, key := sapmock.UntrustedClientCertificate("A")

	for name, tc := range map[string]struct {
		provider string
		err      string
	}{
		"none":         {`host = "h"`, `Missing SAP S/4HANA Authentication`},
		"both":         {fmt.Sprintf(`host = "h"`+"\n"+`username = "u"`+"\n"+`password = "p"`+"\n"+`client_certificate = %q`+"\n"+`client_key = %q`, cert, key), `Conflicting SAP S/4HANA Authentication`},
		"no key":       {fmt.Sprintf(`host = "h"`+"\n"+`client_certificate = %q`, cert), `Missing SAP S/4HANA Client Key`},
		"no password":  {`host = "h"` + "\n" + `username = "u"`, `Missing SAP S/4HANA Password`},
		"invalid cert": {`host = "h"` + "\n" + `client_certificate = "nope"` + "\n" + `client_key = "nope"`, `invalid client certificate`},
	} {
		t.Run(name, func(t *testing.T) {
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      "provider \"s4communication\" {\n" + tc.provider + "\n}\n" + systemConfig,
						ExpectError: regexp.MustCompile(tc.err),
					},
				},
			})
		})
	}
}
