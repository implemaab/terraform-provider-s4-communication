package provider

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-s4communication/internal/acctest/sapmock"
)

// These tests run the provider through Terraform against the in-memory SAP
// mock. After every step the testing framework also verifies that a second
// plan is empty, which catches drift caused by values SAP sets itself.

// providerConfig points the provider at the mock. host accepts a full URL so
// that the plain-HTTP mock on a random port can be reached.
func providerConfig(server *sapmock.Server) string {
	return fmt.Sprintf(`
provider "s4communication" {
  host     = %q
  username = %q
  password = %q
}
`, server.URL, sapmock.Username, sapmock.Password)
}

func TestCommunicationSystemAndUsers(t *testing.T) {
	server := sapmock.New()
	defer server.Close()

	config := func(description, inboundUserID string, port int) string {
		return providerConfig(server) + fmt.Sprintf(`
resource "s4communication_communication_system" "test" {
  communication_system_id = "TEST_SYSTEM"
  name                    = "Test System"
  description             = %q
  host_name               = "example.com"
  tcp_port                = %d
}

resource "s4communication_inbound_user" "test" {
  communication_system_uuid = s4communication_communication_system.test.id
  communication_user_id     = %q
}

resource "s4communication_outbound_user" "test" {
  communication_system_uuid = s4communication_communication_system.test.id
  name                      = "REMOTE_USER"
  password                  = "Secret123!"
}
`, description, port, inboundUserID)
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("first", "CC0000000001", 443),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Configured values
					resource.TestCheckResourceAttr("s4communication_communication_system.test", "description", "first"),
					resource.TestCheckResourceAttr("s4communication_communication_system.test", "host_name", "example.com"),
					// Values SAP set automatically end up in state
					resource.TestCheckResourceAttr("s4communication_communication_system.test", "type", "customer_managed"),
					resource.TestCheckResourceAttr("s4communication_communication_system.test", "logical_system_id", "TEST_SYSTEM"),
					resource.TestCheckResourceAttr("s4communication_communication_system.test", "is_hub_system", "false"),
					resource.TestCheckResourceAttrSet("s4communication_communication_system.test", "id"),
					resource.TestCheckResourceAttr("s4communication_inbound_user.test", "authentication_method", "basic"),
					resource.TestCheckResourceAttrPair("s4communication_inbound_user.test", "communication_system_uuid", "s4communication_communication_system.test", "id"),
					resource.TestCheckResourceAttr("s4communication_outbound_user.test", "password", "Secret123!"),
				),
			},
			{
				// In-place updates
				Config: config("second", "CC0000000002", 8443),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("s4communication_communication_system.test", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("s4communication_inbound_user.test", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("s4communication_outbound_user.test", plancheck.ResourceActionNoop),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("s4communication_communication_system.test", "description", "second"),
					resource.TestCheckResourceAttr("s4communication_communication_system.test", "tcp_port", "8443"),
					resource.TestCheckResourceAttr("s4communication_communication_system.test", "type", "customer_managed"),
					resource.TestCheckResourceAttr("s4communication_inbound_user.test", "communication_user_id", "CC0000000002"),
				),
			},
			{
				ResourceName:      "s4communication_communication_system.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "s4communication_inbound_user.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:            "s4communication_outbound_user.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
			},
		},
	})
}

func TestCommunicationUser(t *testing.T) {
	server := sapmock.New()
	defer server.Close()

	config := func(description, password, userType string) string {
		typeLine := ""
		if userType != "" {
			typeLine = fmt.Sprintf("type = %q", userType)
		}
		return providerConfig(server) + fmt.Sprintf(`
resource "s4communication_communication_user" "test" {
  name        = "TEST_USER"
  description = %q
  password    = %q
  %s
}
`, description, password, typeLine)
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("first", "Secret123!", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					// The ID is assigned by SAP and differs from the name
					resource.TestCheckResourceAttr("s4communication_communication_user.test", "id", "CC0000000001"),
					resource.TestCheckResourceAttr("s4communication_communication_user.test", "name", "TEST_USER"),
					resource.TestCheckResourceAttr("s4communication_communication_user.test", "type", "customer_communication"),
					resource.TestCheckResourceAttr("s4communication_communication_user.test", "is_locked", "false"),
				),
			},
			{
				// Description and password are updated in place
				Config: config("second", "Changed123!", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("s4communication_communication_user.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr("s4communication_communication_user.test", "description", "second"),
			},
			{
				// The type cannot be changed through the API
				Config: config("second", "Changed123!", "OUTBOUND"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("s4communication_communication_user.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("s4communication_communication_user.test", "type", "OUTBOUND"),
					resource.TestCheckResourceAttr("s4communication_communication_user.test", "id", "CC0000000002"),
				),
			},
			{
				ResourceName:            "s4communication_communication_user.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
			},
		},
	})
}

func TestCommunicationArrangement(t *testing.T) {
	server := sapmock.New()
	defer server.Close()

	config := func(name, inboundUser string, withOutbound bool) string {
		outbound := ""
		if withOutbound {
			outbound = "outbound_user = s4communication_outbound_user.test.id"
		}
		return providerConfig(server) + fmt.Sprintf(`
resource "s4communication_communication_system" "test" {
  communication_system_id = "TEST_SYSTEM"
  name                    = "Test System"
}

resource "s4communication_inbound_user" "first" {
  communication_system_uuid = s4communication_communication_system.test.id
  communication_user_id     = "CC0000000001"
}

resource "s4communication_inbound_user" "second" {
  communication_system_uuid = s4communication_communication_system.test.id
  communication_user_id     = "CC0000000002"
}

resource "s4communication_outbound_user" "test" {
  communication_system_uuid = s4communication_communication_system.test.id
  name                      = "REMOTE_USER"
}

resource "s4communication_communication_arrangement" "test" {
  communication_scenario_id = "SAP_COM_0008"
  communication_system_id   = s4communication_communication_system.test.communication_system_id
  name                      = %q
  inbound_user              = s4communication_inbound_user.%s.id
  %s
}
`, name, inboundUser, outbound)
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("Arrangement", "first", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("s4communication_communication_arrangement.test", "id"),
					resource.TestCheckResourceAttr("s4communication_communication_arrangement.test", "communication_system_id", "TEST_SYSTEM"),
					resource.TestCheckResourceAttr("s4communication_communication_arrangement.test", "description", "SAP_COM_0008"),
					resource.TestCheckResourceAttrPair("s4communication_communication_arrangement.test", "inbound_user", "s4communication_inbound_user.first", "id"),
					resource.TestCheckNoResourceAttr("s4communication_communication_arrangement.test", "outbound_user"),
				),
			},
			{
				// Renaming and switching the inbound user happen in place
				Config: config("Renamed", "second", false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("s4communication_communication_arrangement.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("s4communication_communication_arrangement.test", "name", "Renamed"),
					resource.TestCheckResourceAttrPair("s4communication_communication_arrangement.test", "inbound_user", "s4communication_inbound_user.second", "id"),
				),
			},
			{
				// Adding an outbound user requires a new arrangement
				Config: config("Renamed", "second", true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("s4communication_communication_arrangement.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.TestCheckResourceAttrPair("s4communication_communication_arrangement.test", "outbound_user", "s4communication_outbound_user.test", "id"),
			},
			{
				// Import also takes the services and properties SAP created
				// from the scenario into state, which this configuration does
				// not manage
				ResourceName:            "s4communication_communication_arrangement.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"properties", "inbound_services", "outbound_services"},
			},
		},
	})
}

// arrangementProperty returns the values of a property in the mock, found
// by following path (e.g. "OutboundServices", "SAP_COM_0008_OUT") from the
// only stored arrangement.
func arrangementProperty(server *sapmock.Server, name string, path ...string) []string {
	var values []string
	server.Modify(func(data map[string]map[string]map[string]interface{}) {
	arrangements:
		for _, arrangement := range data["CommunicationArrangements"] {
			parent := arrangement
			for i := 0; i+1 < len(path); i += 2 {
				var found map[string]interface{}
				for _, svc := range parent[path[i]].([]interface{}) {
					if svc.(map[string]interface{})["ID"] == path[i+1] {
						found = svc.(map[string]interface{})
					}
				}
				if found == nil {
					continue arrangements // the service belongs to another arrangement
				}
				parent = found
			}
			for _, p := range parent["Properties"].([]interface{}) {
				prop := p.(map[string]interface{})
				if prop["Name"] != name {
					continue
				}
				values = []string{}
				for _, v := range prop["Values"].([]interface{}) {
					values = append(values, v.(map[string]interface{})["Value"].(string))
				}
			}
		}
	})
	return values
}

func checkProperty(server *sapmock.Server, want []string, name string, path ...string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		got := arrangementProperty(server, name, path...)
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) && !(len(got) == 0 && len(want) == 0) {
			return fmt.Errorf("property %s %v in SAP: got %v, want %v", name, path, got, want)
		}
		return nil
	}
}

func TestCommunicationArrangementServices(t *testing.T) {
	server := sapmock.New()
	defer server.Close()

	const base = `
resource "s4communication_communication_system" "test" {
  communication_system_id = "TEST_SYSTEM"
  name                    = "Test System"
}

resource "s4communication_communication_arrangement" "test" {
  communication_scenario_id = "SAP_COM_0008"
  communication_system_id   = s4communication_communication_system.test.communication_system_id
  name                      = "Arrangement"
%s
}
`
	config := func(body string) string { return providerConfig(server) + fmt.Sprintf(base, body) }
	const arrangement = "s4communication_communication_arrangement.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(`
  properties = {
    ARR_PROP = ["a", "b"]
    NEW_PROP = ["x"]
  }
  inbound_services = {
    SAP_COM_0008_IN = {
      properties = { IN_PROP = ["in1"] }
    }
  }
  outbound_services = {
    SAP_COM_0008_OUT = {
      url_path   = "/custom"
      properties = { OUT_PROP = ["o1"] }
    }
  }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Values SAP set are filled in for unconfigured service attributes
					resource.TestCheckResourceAttr(arrangement, "outbound_services.SAP_COM_0008_OUT.status", "ACTIVE"),
					resource.TestCheckResourceAttr(arrangement, "outbound_services.SAP_COM_0008_OUT.url_path", "/custom"),
					resource.TestCheckResourceAttr(arrangement, "outbound_services.SAP_COM_0008_OUT.tcp_port", "0"),
					resource.TestCheckTypeSetElemAttr(arrangement, "properties.ARR_PROP.*", "a"),
					// The values in SAP match the configuration
					checkProperty(server, []string{"a", "b"}, "ARR_PROP"),
					checkProperty(server, []string{"x"}, "NEW_PROP"),
					checkProperty(server, []string{"in1"}, "IN_PROP", "InboundServices", "SAP_COM_0008_IN"),
					checkProperty(server, []string{"o1"}, "OUT_PROP", "OutboundServices", "SAP_COM_0008_OUT"),
				),
			},
			{
				// Change values, clear NEW_PROP, stop managing the inbound
				// service, update the outbound service and add a service the
				// scenario did not create
				Config: config(`
  properties = {
    ARR_PROP = ["b", "c"]
    NEW_PROP = []
  }
  outbound_services = {
    SAP_COM_0008_OUT = {
      status     = "INACTIVE"
      url_path   = "/custom"
      properties = { OUT_PROP = ["o2"] }
    }
    EXTRA_OUT = {
      url_path   = "/extra"
      properties = { EXTRA_PROP = ["e"] }
    }
  }`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(arrangement, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(arrangement, "outbound_services.SAP_COM_0008_OUT.status", "INACTIVE"),
					resource.TestCheckResourceAttr(arrangement, "outbound_services.EXTRA_OUT.status", ""),
					checkProperty(server, []string{"b", "c"}, "ARR_PROP"),
					checkProperty(server, []string{}, "NEW_PROP"),
					// No longer configured, so left unchanged
					checkProperty(server, []string{"in1"}, "IN_PROP", "InboundServices", "SAP_COM_0008_IN"),
					checkProperty(server, []string{"o2"}, "OUT_PROP", "OutboundServices", "SAP_COM_0008_OUT"),
					checkProperty(server, []string{"e"}, "EXTRA_PROP", "OutboundServices", "EXTRA_OUT"),
				),
			},
			{
				// A value changed outside Terraform is detected as drift
				PreConfig: func() {
					server.Modify(func(data map[string]map[string]map[string]interface{}) {
						for _, a := range data["CommunicationArrangements"] {
							for _, p := range a["Properties"].([]interface{}) {
								if prop := p.(map[string]interface{}); prop["Name"] == "ARR_PROP" {
									prop["Values"] = []interface{}{map[string]interface{}{"UUID": "manual", "Value": "manual"}}
								}
							}
						}
					})
				},
				Config:             config(`properties = { ARR_PROP = ["b", "c"] }`),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				ResourceName:            arrangement,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"properties", "inbound_services", "outbound_services"},
			},
		},
	})
}
