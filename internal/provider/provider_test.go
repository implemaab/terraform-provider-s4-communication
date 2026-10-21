package provider

import (
	"context"
	"testing"

	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testProtoV6ProviderFactories are used to instantiate a provider during
// testing. The factory function will be invoked for every Terraform CLI
// command executed to create a provider server to which the CLI can reattach.
var testProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"s4communication": providerserver.NewProtocol6WithError(New("test")()),
}

func TestProviderMetadata(t *testing.T) {
	resp := &fwprovider.MetadataResponse{}
	New("test")().Metadata(context.Background(), fwprovider.MetadataRequest{}, resp)

	if resp.TypeName != "s4communication" {
		t.Errorf("Expected TypeName to be 's4communication', got %s", resp.TypeName)
	}
	if resp.Version != "test" {
		t.Errorf("Expected Version to be 'test', got %s", resp.Version)
	}
}

func TestProviderResources(t *testing.T) {
	resources := New("test")().Resources(context.Background())
	if len(resources) != 5 {
		t.Errorf("Expected 5 resources, got %d", len(resources))
	}
}
