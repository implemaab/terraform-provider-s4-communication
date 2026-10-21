package util

import (
	"testing"

	s4types "terraform-provider-s4-communication/internal/types"
)

func TestNextEndpoint(t *testing.T) {
	service := s4types.ServiceCommunicationSystem
	for next, want := range map[string]string{
		"":                                   "",
		"CommunicationSystems?$skiptoken=10": "CommunicationSystems?$skiptoken=10",
		string(service) + "/InboundUsers?$skiptoken=5":                                     "InboundUsers?$skiptoken=5",
		"https://host.example" + string(service) + "/OutboundUsers?$skiptoken=2&$expand=X": "OutboundUsers?$skiptoken=2&$expand=X",
	} {
		if got := nextEndpoint(next, service); got != want {
			t.Errorf("nextEndpoint(%q) = %q, want %q", next, got, want)
		}
	}
}
