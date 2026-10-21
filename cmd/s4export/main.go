// Command s4export exports the communication management configuration of an
// SAP S/4HANA Cloud system as Terraform configuration with import blocks.
//
//	S4_HOST=my000000-api.s4hana.cloud.sap S4_USERNAME=... S4_PASSWORD=... \
//	  go run ./cmd/s4export -out ./exported
//
// Instead of S4_USERNAME and S4_PASSWORD, a client certificate can be given
// with S4_CLIENT_CERTIFICATE and S4_CLIENT_KEY (PEM content or file paths).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"terraform-provider-s4-communication/internal/exporter"
	s4types "terraform-provider-s4-communication/internal/types"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// clientConfigFromEnv reads the connection settings from the environment.
func clientConfigFromEnv() (*s4types.ClientConfig, error) {
	config := &s4types.ClientConfig{
		Host:     os.Getenv("S4_HOST"),
		Username: os.Getenv("S4_USERNAME"),
		Password: os.Getenv("S4_PASSWORD"),
	}
	var err error
	if config.ClientCertificate, err = s4types.ReadPEM(os.Getenv("S4_CLIENT_CERTIFICATE")); err != nil {
		return nil, fmt.Errorf("S4_CLIENT_CERTIFICATE: %w", err)
	}
	if config.ClientKey, err = s4types.ReadPEM(os.Getenv("S4_CLIENT_KEY")); err != nil {
		return nil, fmt.Errorf("S4_CLIENT_KEY: %w", err)
	}

	basic := config.Username != "" || config.Password != ""
	certificate := config.ClientCertificate != "" || config.ClientKey != ""
	switch {
	case config.Host == "":
		return nil, fmt.Errorf("set S4_HOST")
	case basic && certificate:
		return nil, fmt.Errorf("set either S4_USERNAME and S4_PASSWORD, or S4_CLIENT_CERTIFICATE and S4_CLIENT_KEY, not both")
	case certificate && (config.ClientCertificate == "" || config.ClientKey == ""):
		return nil, fmt.Errorf("set both S4_CLIENT_CERTIFICATE and S4_CLIENT_KEY")
	case !certificate && (config.Username == "" || config.Password == ""):
		return nil, fmt.Errorf("set S4_USERNAME and S4_PASSWORD, or S4_CLIENT_CERTIFICATE and S4_CLIENT_KEY")
	}
	return config, nil
}

func run() error {
	out := flag.String("out", "exported", "directory to write the Terraform files to")
	systems := flag.String("systems", "", "only export communication systems whose ID matches this regular expression, with their users and arrangements")
	allUsers := flag.Bool("all-users", false, "export all communication users, not only those used by the exported inbound users")
	force := flag.Bool("force", false, "overwrite existing files in the output directory")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: s4export [flags]\n\n"+
			"Exports SAP S/4HANA Cloud communication systems, users and arrangements as\n"+
			"Terraform configuration with import blocks. Connection settings are read\n"+
			"from S4_HOST and either S4_USERNAME and S4_PASSWORD, or S4_CLIENT_CERTIFICATE\n"+
			"and S4_CLIENT_KEY (PEM content or file paths).\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	config, err := clientConfigFromEnv()
	if err != nil {
		return err
	}

	opts := exporter.Options{AllUsers: *allUsers}
	if *systems != "" {
		re, err := regexp.Compile(*systems)
		if err != nil {
			return fmt.Errorf("invalid -systems expression: %w", err)
		}
		opts.Systems = re
	}

	client, err := s4types.NewClient(config)
	if err != nil {
		return err
	}
	result, err := exporter.Export(context.Background(), client, opts)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	names := make([]string, 0, len(result.Files))
	for name := range result.Files {
		names = append(names, name)
		if _, err := os.Stat(filepath.Join(*out, name)); err == nil && !*force {
			return fmt.Errorf("%s already exists; use -force to overwrite", filepath.Join(*out, name))
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(*out, name), result.Files[name], 0o644); err != nil {
			return err
		}
	}

	types := make([]string, 0, len(result.Counts))
	for t := range result.Counts {
		types = append(types, t)
	}
	sort.Strings(types)
	fmt.Printf("Exported to %s:\n", *out)
	for _, t := range types {
		fmt.Printf("  %-45s %d\n", t, result.Counts[t])
	}
	fmt.Println("\nNext: cd into the directory, run \"terraform init\" and check that \"terraform plan\" only imports.")
	return nil
}
