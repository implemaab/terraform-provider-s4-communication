package provider

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-s4communication/internal/acctest/sapmock"
	"terraform-provider-s4communication/internal/exporter"
	s4types "terraform-provider-s4communication/internal/types"
	"terraform-provider-s4communication/internal/util"
)

// seedExportData creates objects in the mock directly through the API, the
// way they would exist in a system that is not managed by Terraform yet.
func seedExportData(t *testing.T, client *s4types.Client) {
	ctx := context.Background()
	create := func(service s4types.Service, endpoint string, body map[string]interface{}) map[string]interface{} {
		t.Helper()
		created, err := util.CreateEntity(ctx, client, service, endpoint, body)
		if err != nil {
			t.Fatalf("creating %s: %s", endpoint, err)
		}
		return created
	}
	cs, cu, ca := s4types.ServiceCommunicationSystem, s4types.ServiceCommunicationUser, s4types.ServiceCommunicationArrangement

	system := create(cs, "CommunicationSystems", map[string]interface{}{
		"ID": "EXT-API.1", "Name": "External API", "HostName": "api.example.com", "OwnerContactPersonEMail": "jane@example.com",
	})
	other := create(cs, "CommunicationSystems", map[string]interface{}{"ID": "OTHER", "Name": "Other system"})

	user := create(cu, "CommunicationUsers", map[string]interface{}{"Name": "API_USER", "Type": "customer_communication", "Description": "API user", "Password": "secret"})
	create(cu, "CommunicationUsers", map[string]interface{}{"Name": "UNUSED_USER", "Type": "customer_communication", "Password": "secret"})

	inbound := create(cs, "InboundUsers", map[string]interface{}{"CommunicationSystemUUID": system["UUID"], "CommunicationUserID": user["ID"]})
	outbound := create(cs, "OutboundUsers", map[string]interface{}{"CommunicationSystemUUID": system["UUID"], "Name": "remote", "Password": "secret"})
	create(cs, "InboundUsers", map[string]interface{}{"CommunicationSystemUUID": other["UUID"], "CommunicationUserID": user["ID"]})

	arrangement := create(ca, "CommunicationArrangements", map[string]interface{}{
		"CommunicationScenarioID": "SAP_COM_0008", "CommunicationSystemID": "EXT-API.1", "Name": "EXT API arrangement",
		"InboundUser":  map[string]interface{}{"CommunicationSystemInboundUserUUID": inbound["UUID"]},
		"OutboundUser": map[string]interface{}{"CommunicationSystemOutboundUserUUID": outbound["UUID"]},
	})
	create(ca, "CommunicationArrangements", map[string]interface{}{
		"CommunicationScenarioID": "SAP_COM_0009", "CommunicationSystemID": "OTHER", "Name": "Other arrangement",
	})

	// Change service settings and property values away from the scenario defaults
	expanded, _, err := util.GetEntity(ctx, client, ca, fmt.Sprintf("CommunicationArrangements/%s?$expand=OutboundServices,InboundServices($expand=Properties),Properties", arrangement["UUID"]))
	if err != nil {
		t.Fatal(err)
	}
	first := func(nav string, parent map[string]interface{}) map[string]interface{} {
		return parent[nav].([]interface{})[0].(map[string]interface{})
	}
	outboundService := first("OutboundServices", expanded)
	if err := util.PatchEntity(ctx, client, ca, "OutboundServices/"+outboundService["UUID"].(string), map[string]interface{}{"Status": "INACTIVE", "TCPPort": 8443}); err != nil {
		t.Fatal(err)
	}
	create(ca, fmt.Sprintf("Properties/%s/Values", first("Properties", expanded)["UUID"]), map[string]interface{}{"Value": "extra"})
	create(ca, fmt.Sprintf("InboundServiceProperties/%s/Values", first("Properties", first("InboundServices", expanded))["UUID"]), map[string]interface{}{"Value": "in"})
}

func exportedConfig(result *exporter.Result) string {
	names := make([]string, 0, len(result.Files))
	for name := range result.Files {
		if name != "provider.tf" { // the test supplies its own provider block
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		b.Write(result.Files[name])
		b.WriteString("\n")
	}
	return b.String()
}

func TestExport(t *testing.T) {
	server := sapmock.New()
	defer server.Close()
	server.PageSize = 2 // exercise @odata.nextLink paging

	client, err := s4types.NewClient(&s4types.ClientConfig{Host: server.URL, Username: sapmock.Username, Password: sapmock.Password})
	if err != nil {
		t.Fatal(err)
	}
	seedExportData(t, client)

	result, err := exporter.Export(context.Background(), client, exporter.Options{})
	if err != nil {
		t.Fatal(err)
	}
	config := exportedConfig(result)
	t.Log(config)

	for _, want := range []string{
		`resource "s4communication_communication_system" "ext_api_1"`,
		`communication_system_uuid = s4communication_communication_system.ext_api_1.id`,
		`communication_user_id     = s4communication_communication_user.api_user.id`,
		`communication_system_id   = s4communication_communication_system.ext_api_1.communication_system_id`,
		`inbound_user              = s4communication_inbound_user.ext_api_1_api_user.id`,
		`password = var.api_user_password`,
		`status   = "INACTIVE"`,
	} {
		if !strings.Contains(config, want) {
			t.Errorf("exported configuration does not contain %q", want)
		}
	}
	if strings.Contains(config, "UNUSED_USER") {
		t.Error("unused communication user should not be exported without AllUsers")
	}
	if strings.Contains(config, `password = "`) || strings.Contains(config, "secret") {
		t.Error("passwords must not be exported")
	}
	wantCounts := map[string]int{
		"s4communication_communication_system":      2,
		"s4communication_communication_user":        1,
		"s4communication_inbound_user":              2,
		"s4communication_outbound_user":             1,
		"s4communication_communication_arrangement": 2,
	}
	for resourceType, want := range wantCounts {
		if got := result.Counts[resourceType]; got != want {
			t.Errorf("%s: exported %d, want %d", resourceType, got, want)
		}
	}

	// The exported configuration imports everything without any changes
	var noChanges []plancheck.PlanCheck
	for resourceType := range wantCounts {
		for _, m := range regexp.MustCompile(`(?m)^resource "`+resourceType+`" "(\w+)"`).FindAllStringSubmatch(config, -1) {
			noChanges = append(noChanges, plancheck.ExpectResourceAction(resourceType+"."+m[1], plancheck.ResourceActionNoop))
		}
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server) + config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: append(noChanges, plancheck.ExpectEmptyPlan()),
				},
			},
		},
	})
}

func TestExportFilters(t *testing.T) {
	server := sapmock.New()
	defer server.Close()

	client, err := s4types.NewClient(&s4types.ClientConfig{Host: server.URL, Username: sapmock.Username, Password: sapmock.Password})
	if err != nil {
		t.Fatal(err)
	}
	seedExportData(t, client)

	result, err := exporter.Export(context.Background(), client, exporter.Options{Systems: regexp.MustCompile(`^OTHER$`), AllUsers: true})
	if err != nil {
		t.Fatal(err)
	}
	config := exportedConfig(result)

	if strings.Contains(config, "EXT-API.1") || strings.Contains(config, "SAP_COM_0008") {
		t.Error("objects of systems not matching the filter should not be exported")
	}
	if !strings.Contains(config, "UNUSED_USER") || !strings.Contains(config, "Other arrangement") {
		t.Errorf("expected all users and the arrangement of OTHER, got:\n%s", config)
	}
	if result.Counts["s4communication_communication_user"] != 2 {
		t.Errorf("expected 2 users with AllUsers, got %d", result.Counts["s4communication_communication_user"])
	}
}

// Importing an arrangement with a configuration that does not list its
// services only drops them from state; nothing changes in SAP.
func TestImportArrangementWithMinimalConfig(t *testing.T) {
	server := sapmock.New()
	defer server.Close()

	client, err := s4types.NewClient(&s4types.ClientConfig{Host: server.URL, Username: sapmock.Username, Password: sapmock.Password})
	if err != nil {
		t.Fatal(err)
	}
	seedExportData(t, client)
	var uuid string
	server.Modify(func(data map[string]map[string]map[string]interface{}) {
		for key, a := range data["CommunicationArrangements"] {
			if a["Name"] == "Other arrangement" {
				uuid = key
			}
		}
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server) + fmt.Sprintf(`
import {
  to = s4communication_communication_arrangement.test
  id = %q
}

resource "s4communication_communication_arrangement" "test" {
  communication_scenario_id = "SAP_COM_0009"
  communication_system_id   = "OTHER"
  name                      = "Other arrangement"
}
`, uuid),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("s4communication_communication_arrangement.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("s4communication_communication_arrangement.test", "properties.%"),
					func(*terraform.State) error {
						var values []interface{}
						server.Modify(func(data map[string]map[string]map[string]interface{}) {
							for _, p := range data["CommunicationArrangements"][uuid]["Properties"].([]interface{}) {
								if prop := p.(map[string]interface{}); prop["Name"] == "ARR_PROP" {
									values = prop["Values"].([]interface{})
								}
							}
						})
						if len(values) != 1 || values[0].(map[string]interface{})["Value"] != "default" {
							return fmt.Errorf("ARR_PROP changed in SAP: %v", values)
						}
						return nil
					},
					checkProperty(server, []string{"default"}, "OUT_PROP", "OutboundServices", "SAP_COM_0009_OUT"),
				),
			},
		},
	})
}
