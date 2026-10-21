package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"

	"terraform-provider-s4communication/internal/acctest/sapmock"
	s4types "terraform-provider-s4communication/internal/types"
)

// The objects created by seedExportData, by their keys in the mock
const (
	extSystemUUID   = "00000000-0000-0000-0000-000000000001"
	otherSystemUUID = "00000000-0000-0000-0000-000000000002"
	apiUserID       = "CC0000000003"
)

func TestQuery(t *testing.T) {
	server := sapmock.New()
	defer server.Close()
	server.PageSize = 2 // results span several pages

	client, err := s4types.NewClient(&s4types.ClientConfig{Host: server.URL, Username: sapmock.Username, Password: sapmock.Password})
	if err != nil {
		t.Fatal(err)
	}
	seedExportData(t, client)

	byID := func(id string) queryfilter.QueryFilter {
		return queryfilter.ByResourceIdentity(map[string]knownvalue.Check{"id": knownvalue.StringExact(id)})
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_14_0)},
		Steps: []resource.TestStep{
			{
				Query: true,
				Config: providerConfig(server) + `
list "s4communication_communication_system" "all" {
  provider = s4communication
}

list "s4communication_communication_system" "other" {
  provider = s4communication
  config {
    communication_system_id = "OTHER"
  }
}

list "s4communication_communication_system" "limited" {
  provider = s4communication
  limit    = 1
}

list "s4communication_communication_user" "all" {
  provider = s4communication
}

list "s4communication_inbound_user" "ext" {
  provider = s4communication
  config {
    communication_system_uuid = "` + extSystemUUID + `"
  }
}

list "s4communication_outbound_user" "all" {
  provider = s4communication
}

list "s4communication_communication_arrangement" "all" {
  provider         = s4communication
  include_resource = true
}

list "s4communication_communication_arrangement" "scenario" {
  provider = s4communication
  config {
    communication_scenario_id = "SAP_COM_0009"
  }
}
`,
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectLength("s4communication_communication_system.all", 2),
					querycheck.ExpectLength("s4communication_communication_system.other", 1),
					querycheck.ExpectIdentity("s4communication_communication_system.other", map[string]knownvalue.Check{
						"id": knownvalue.StringExact(otherSystemUUID),
					}),
					querycheck.ExpectResourceDisplayName("s4communication_communication_system.all", byID(extSystemUUID),
						knownvalue.StringExact("EXT-API.1 (External API)")),
					querycheck.ExpectLength("s4communication_communication_system.limited", 1),

					querycheck.ExpectLength("s4communication_communication_user.all", 2),
					querycheck.ExpectResourceDisplayName("s4communication_communication_user.all", byID(apiUserID),
						knownvalue.StringExact("API_USER")),

					querycheck.ExpectLength("s4communication_inbound_user.ext", 1),
					querycheck.ExpectLength("s4communication_outbound_user.all", 1),

					querycheck.ExpectLength("s4communication_communication_arrangement.all", 2),
					querycheck.ExpectLength("s4communication_communication_arrangement.scenario", 1),
					querycheck.ExpectResourceKnownValues("s4communication_communication_arrangement.all",
						queryfilter.ByDisplayName(knownvalue.StringExact("EXT API arrangement")),
						[]querycheck.KnownValueCheck{
							{Path: tfjsonpath.New("communication_system_id"), KnownValue: knownvalue.StringExact("EXT-API.1")},
							{Path: tfjsonpath.New("communication_scenario_id"), KnownValue: knownvalue.StringExact("SAP_COM_0008")},
							{Path: tfjsonpath.New("properties").AtMapKey("ARR_PROP"), KnownValue: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("default"), knownvalue.StringExact("extra"),
							})},
							{Path: tfjsonpath.New("outbound_services").AtMapKey("SAP_COM_0008_OUT").AtMapKey("status"), KnownValue: knownvalue.StringExact("INACTIVE")},
							{Path: tfjsonpath.New("inbound_user"), KnownValue: knownvalue.NotNull()},
						}),
				},
			},
		},
	})
}

// Every resource can be imported with an import block that uses the
// resource identity, as returned by terraform query. The configuration is
// written the way query or s4export generate it, so each import is a no-op:
// no password (SAP never returns it) and the arrangement's services as SAP
// created them from the scenario.
func TestImportByIdentity(t *testing.T) {
	server := sapmock.New()
	defer server.Close()

	importStep := func(name string, ignore ...string) resource.TestStep {
		return resource.TestStep{
			ResourceName:            name,
			ImportState:             true,
			ImportStateKind:         resource.ImportBlockWithResourceIdentity,
			ImportStateVerifyIgnore: ignore,
		}
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_12_0)},
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server) + `
resource "s4communication_communication_system" "test" {
  communication_system_id = "IDENTITY"
  name                    = "Identity"
}

resource "s4communication_communication_user" "test" {
  name = "IDENTITY_USER"
}

resource "s4communication_inbound_user" "test" {
  communication_system_uuid = s4communication_communication_system.test.id
  communication_user_id     = s4communication_communication_user.test.id
}

resource "s4communication_outbound_user" "test" {
  communication_system_uuid = s4communication_communication_system.test.id
  name                      = "remote"
}

resource "s4communication_communication_arrangement" "test" {
  communication_scenario_id = "SAP_COM_0008"
  communication_system_id   = s4communication_communication_system.test.communication_system_id
  name                      = "Identity arrangement"
  inbound_user              = s4communication_inbound_user.test.id

  properties = {
    ARR_PROP = ["default"]
  }
  outbound_services = {
    SAP_COM_0008_OUT = {
      properties = {
        OUT_PROP = ["default"]
      }
    }
  }
}
`,
			},
			importStep("s4communication_communication_system.test"),
			importStep("s4communication_communication_user.test"),
			importStep("s4communication_inbound_user.test"),
			importStep("s4communication_outbound_user.test"),
			importStep("s4communication_communication_arrangement.test"),
		},
	})
}
