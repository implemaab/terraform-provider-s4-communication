package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"terraform-provider-s4communication/internal/acctest/sapmock"
	s4types "terraform-provider-s4communication/internal/types"
	"terraform-provider-s4communication/internal/util"
)

func runWithArgs(t *testing.T, args ...string) error {
	t.Helper()
	flag.CommandLine = flag.NewFlagSet("s4export", flag.ContinueOnError)
	os.Args = append([]string{"s4export"}, args...)
	return run()
}

func TestRun(t *testing.T) {
	server := sapmock.New()
	defer server.Close()
	t.Setenv("S4_HOST", server.URL)
	t.Setenv("S4_USERNAME", sapmock.Username)
	t.Setenv("S4_PASSWORD", sapmock.Password)
	t.Setenv("S4_CLIENT_CERTIFICATE", "")
	t.Setenv("S4_CLIENT_KEY", "")

	client, err := s4types.NewClient(&s4types.ClientConfig{Host: server.URL, Username: sapmock.Username, Password: sapmock.Password})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := util.CreateEntity(context.Background(), client, s4types.ServiceCommunicationSystem, "CommunicationSystems",
		map[string]interface{}{"ID": "SYS", "Name": "System"}); err != nil {
		t.Fatal(err)
	}

	out := t.TempDir()
	if err := runWithArgs(t, "-out", out); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"provider.tf", "communication_systems.tf", "imports.tf"} {
		content, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatalf("%s not written: %s", name, err)
		}
		if name == "communication_systems.tf" && !strings.Contains(string(content), `communication_system_id = "SYS"`) {
			t.Errorf("unexpected %s:\n%s", name, content)
		}
	}

	if err := runWithArgs(t, "-out", out); err == nil || !strings.Contains(err.Error(), "-force") {
		t.Errorf("expected an error about existing files, got %v", err)
	}
	if err := runWithArgs(t, "-out", out, "-force"); err != nil {
		t.Errorf("-force should overwrite: %s", err)
	}
}

func TestClientConfigFromEnv(t *testing.T) {
	cert, key := sapmock.UntrustedClientCertificate("A")
	keyFile := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(keyFile, []byte(key), 0o600); err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		env map[string]string
		err string
	}{
		"basic":        {map[string]string{"S4_HOST": "h", "S4_USERNAME": "u", "S4_PASSWORD": "p"}, ""},
		"certificate":  {map[string]string{"S4_HOST": "h", "S4_CLIENT_CERTIFICATE": cert, "S4_CLIENT_KEY": keyFile}, ""},
		"no host":      {map[string]string{"S4_USERNAME": "u", "S4_PASSWORD": "p"}, "S4_HOST"},
		"both":         {map[string]string{"S4_HOST": "h", "S4_USERNAME": "u", "S4_PASSWORD": "p", "S4_CLIENT_CERTIFICATE": cert, "S4_CLIENT_KEY": key}, "not both"},
		"no key":       {map[string]string{"S4_HOST": "h", "S4_CLIENT_CERTIFICATE": cert}, "set both"},
		"no password":  {map[string]string{"S4_HOST": "h", "S4_USERNAME": "u"}, "S4_PASSWORD"},
		"missing file": {map[string]string{"S4_HOST": "h", "S4_CLIENT_CERTIFICATE": "/does/not/exist.pem", "S4_CLIENT_KEY": key}, "S4_CLIENT_CERTIFICATE"},
	} {
		t.Run(name, func(t *testing.T) {
			for _, v := range []string{"S4_HOST", "S4_USERNAME", "S4_PASSWORD", "S4_CLIENT_CERTIFICATE", "S4_CLIENT_KEY"} {
				t.Setenv(v, tc.env[v])
			}
			config, err := clientConfigFromEnv()
			if tc.err == "" {
				if err != nil {
					t.Fatal(err)
				}
				if name == "certificate" && config.ClientKey != key {
					t.Error("the key should be read from the file")
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("expected an error containing %q, got %v", tc.err, err)
			}
		})
	}
}
