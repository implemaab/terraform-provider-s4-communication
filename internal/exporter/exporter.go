// Package exporter reads the communication management configuration of an
// SAP S/4HANA Cloud system and writes it as Terraform configuration, with
// import blocks that bring the existing objects under Terraform management.
package exporter

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zclconf/go-cty/cty"

	"terraform-provider-s4communication/internal/service/communicationarrangement"
	"terraform-provider-s4communication/internal/service/communicationsystem"
	"terraform-provider-s4communication/internal/service/communicationuser"
	"terraform-provider-s4communication/internal/service/inbounduser"
	"terraform-provider-s4communication/internal/service/outbounduser"
	s4types "terraform-provider-s4communication/internal/types"
	"terraform-provider-s4communication/internal/util"
)

const (
	typeSystem       = "s4communication_communication_system"
	typeInboundUser  = "s4communication_inbound_user"
	typeOutboundUser = "s4communication_outbound_user"
	typeUser         = "s4communication_communication_user"
	typeArrangement  = "s4communication_communication_arrangement"
)

// Options controls what is exported.
type Options struct {
	// Systems selects communication systems by their ID; nil exports all.
	// Only the users and arrangements of selected systems are exported.
	Systems *regexp.Regexp
	// AllUsers exports every communication user, not only the ones used by
	// the inbound users of the exported systems.
	AllUsers bool
}

// Result is the generated configuration.
type Result struct {
	// Files maps file names to their contents.
	Files map[string][]byte
	// Counts maps resource types to the number of exported resources.
	Counts map[string]int
}

// Export reads the configuration from SAP and generates Terraform files.
func Export(ctx context.Context, client *s4types.Client, opts Options) (*Result, error) {
	systems, err := util.ListAll(ctx, client, s4types.ServiceCommunicationSystem, "CommunicationSystems")
	if err != nil {
		return nil, fmt.Errorf("listing communication systems: %w", err)
	}
	inboundUsers, err := util.ListAll(ctx, client, s4types.ServiceCommunicationSystem, "InboundUsers")
	if err != nil {
		return nil, fmt.Errorf("listing inbound users: %w", err)
	}
	outboundUsers, err := util.ListAll(ctx, client, s4types.ServiceCommunicationSystem, "OutboundUsers")
	if err != nil {
		return nil, fmt.Errorf("listing outbound users: %w", err)
	}
	users, err := util.ListAll(ctx, client, s4types.ServiceCommunicationUser, "CommunicationUsers")
	if err != nil {
		return nil, fmt.Errorf("listing communication users: %w", err)
	}
	arrangements, err := util.ListAll(ctx, client, s4types.ServiceCommunicationArrangement, "CommunicationArrangements?$expand="+communicationarrangement.ExpandAll)
	if err != nil {
		return nil, fmt.Errorf("listing communication arrangements: %w", err)
	}

	g := newGenerator()

	// Communication systems and their users
	systemsFile := g.file("communication_systems.tf")
	selected := map[string]bool{} // system UUIDs
	usedUserIDs := map[string]bool{}
	sortBy(systems, "ID")
	for _, sys := range systems {
		if opts.Systems != nil && !opts.Systems.MatchString(str(sys, "ID")) {
			continue
		}
		selected[str(sys, "UUID")] = true
		var model communicationsystem.CommunicationSystemResourceModel
		util.ApplyResponse(&model, sys)
		name := g.add(typeSystem, str(sys, "ID"), str(sys, "UUID"), str(sys, "UUID"))
		block := g.resource(systemsFile, typeSystem, name)
		g.attributes(block, util.ConfigValues(&model), nil, "communication_system_id", "name")
	}

	// Communication users are generated first so inbound users can refer to them
	for _, in := range inboundUsers {
		if selected[str(in, "CommunicationSystemUUID")] {
			usedUserIDs[str(in, "CommunicationUserID")] = true
		}
	}
	usersFile := g.file("communication_users.tf")
	sortBy(users, "Name")
	for _, user := range users {
		if !opts.AllUsers && !usedUserIDs[str(user, "ID")] {
			continue
		}
		var model communicationuser.CommunicationUserResourceModel
		util.ApplyResponse(&model, user)
		name := g.add(typeUser, str(user, "Name"), str(user, "ID"), str(user, "ID"))
		block := g.resource(usersFile, typeUser, name)
		g.attributes(block, util.ConfigValues(&model), nil, "name")
		passwordHint(block, name)
	}

	sortBy(inboundUsers, "CommunicationUserID")
	for _, in := range inboundUsers {
		systemUUID := str(in, "CommunicationSystemUUID")
		if !selected[systemUUID] {
			continue
		}
		var model inbounduser.InboundUserResourceModel
		util.ApplyResponse(&model, in)
		name := g.add(typeInboundUser, g.label(typeSystem, systemUUID)+"_"+g.label(typeUser, str(in, "CommunicationUserID")), str(in, "UUID"), str(in, "UUID"))
		block := g.resource(systemsFile, typeInboundUser, name)
		g.attributes(block, util.ConfigValues(&model), map[string]hclwrite.Tokens{
			"communication_system_uuid": g.ref(typeSystem, systemUUID, "id"),
			"communication_user_id":     g.ref(typeUser, str(in, "CommunicationUserID"), "id"),
		}, "communication_system_uuid", "communication_user_id")
	}

	sortBy(outboundUsers, "Name")
	for _, out := range outboundUsers {
		systemUUID := str(out, "CommunicationSystemUUID")
		if !selected[systemUUID] {
			continue
		}
		var model outbounduser.OutboundUserResourceModel
		util.ApplyResponse(&model, out)
		name := g.add(typeOutboundUser, g.label(typeSystem, systemUUID)+"_"+str(out, "Name"), str(out, "UUID"), str(out, "UUID"))
		block := g.resource(systemsFile, typeOutboundUser, name)
		g.attributes(block, util.ConfigValues(&model), map[string]hclwrite.Tokens{
			"communication_system_uuid": g.ref(typeSystem, systemUUID, "id"),
		}, "communication_system_uuid", "name")
		passwordHint(block, name)
	}

	// Communication arrangements
	selectedSystemIDs := map[string]string{} // ID -> UUID
	for _, sys := range systems {
		if selected[str(sys, "UUID")] {
			selectedSystemIDs[str(sys, "ID")] = str(sys, "UUID")
		}
	}
	arrangementsFile := g.file("communication_arrangements.tf")
	sortBy(arrangements, "Name")
	for _, arr := range arrangements {
		systemUUID, ok := selectedSystemIDs[str(arr, "CommunicationSystemID")]
		if !ok {
			continue
		}
		var model communicationarrangement.CommunicationArrangementResourceModel
		util.ApplyResponse(&model, arr)
		name := g.add(typeArrangement, str(arr, "Name"), str(arr, "UUID"), str(arr, "UUID"))
		block := g.resource(arrangementsFile, typeArrangement, name)
		g.attributes(block, util.ConfigValues(&model), map[string]hclwrite.Tokens{
			"communication_system_id": g.ref(typeSystem, systemUUID, "communication_system_id"),
		}, "communication_scenario_id", "communication_system_id", "name")

		body := block.Body()
		if in := nestedUser(arr, "InboundUser", "CommunicationSystemInboundUserUUID"); in != "" {
			body.SetAttributeRaw("inbound_user", g.ref(typeInboundUser, in, "id"))
		}
		if out := nestedUser(arr, "OutboundUser", "CommunicationSystemOutboundUserUUID"); out != "" {
			body.SetAttributeRaw("outbound_user", g.ref(typeOutboundUser, out, "id"))
		}
		writeServices(body, communicationarrangement.SnapshotServices(arr))
	}

	return g.result(), nil
}

// --- reading ---

func str(m map[string]interface{}, key string) string {
	s, _ := m[key].(string)
	return s
}

func nestedUser(arrangement map[string]interface{}, nav, key string) string {
	nested, _ := arrangement[nav].(map[string]interface{})
	return str(nested, key)
}

func sortBy(items []map[string]interface{}, key string) {
	sort.SliceStable(items, func(i, j int) bool { return str(items[i], key) < str(items[j], key) })
}

// --- generating ---

type generator struct {
	files   map[string]*hclwrite.File
	order   []string
	imports *hclwrite.File
	names   map[string]map[string]string // type -> key -> resource name
	used    map[string]map[string]bool   // type -> resource name -> used
	counts  map[string]int
}

func newGenerator() *generator {
	return &generator{
		files:   map[string]*hclwrite.File{},
		imports: hclwrite.NewEmptyFile(),
		names:   map[string]map[string]string{},
		used:    map[string]map[string]bool{},
		counts:  map[string]int{},
	}
}

func (g *generator) file(name string) *hclwrite.File {
	f := hclwrite.NewEmptyFile()
	g.files[name] = f
	g.order = append(g.order, name)
	return f
}

var invalidNameChars = regexp.MustCompile(`[^a-z0-9_]+`)

// add registers a resource under key (the value other resources refer to it
// by), derives a unique resource name from label and adds its import block.
func (g *generator) add(resourceType, label, key, importID string) string {
	name := strings.Trim(invalidNameChars.ReplaceAllString(strings.ToLower(label), "_"), "_")
	if name == "" || (name[0] >= '0' && name[0] <= '9') {
		name = "r_" + name
	}
	if g.used[resourceType] == nil {
		g.used[resourceType] = map[string]bool{}
		g.names[resourceType] = map[string]string{}
	}
	unique := name
	for i := 2; g.used[resourceType][unique]; i++ {
		unique = fmt.Sprintf("%s_%d", name, i)
	}
	g.used[resourceType][unique] = true
	g.names[resourceType][key] = unique
	g.counts[resourceType]++

	block := g.imports.Body().AppendNewBlock("import", nil)
	block.Body().SetAttributeTraversal("to", hcl.Traversal{hcl.TraverseRoot{Name: resourceType}, hcl.TraverseAttr{Name: unique}})
	block.Body().SetAttributeValue("id", cty.StringVal(importID))
	g.imports.Body().AppendNewline()
	return unique
}

// label returns the resource name registered for key, or key itself.
func (g *generator) label(resourceType, key string) string {
	if name, ok := g.names[resourceType][key]; ok {
		return name
	}
	return key
}

// ref returns a reference to an attribute of the resource registered for
// key, or key as a literal when that resource is not exported.
func (g *generator) ref(resourceType, key, attribute string) hclwrite.Tokens {
	if name, ok := g.names[resourceType][key]; ok {
		return hclwrite.TokensForTraversal(hcl.Traversal{
			hcl.TraverseRoot{Name: resourceType}, hcl.TraverseAttr{Name: name}, hcl.TraverseAttr{Name: attribute},
		})
	}
	return hclwrite.TokensForValue(cty.StringVal(key))
}

func (g *generator) resource(f *hclwrite.File, resourceType, name string) *hclwrite.Block {
	if len(f.Body().Blocks()) > 0 {
		f.Body().AppendNewline()
	}
	return f.Body().AppendNewBlock("resource", []string{resourceType, name})
}

// attributes writes the values read from SAP. Empty optional values are
// left out: the attributes are computed, so Terraform keeps SAP's value.
func (g *generator) attributes(block *hclwrite.Block, values []util.ConfigValue, refs map[string]hclwrite.Tokens, required ...string) {
	body := block.Body()
	for _, v := range values {
		if tokens, ok := refs[v.Name]; ok {
			body.SetAttributeRaw(v.Name, tokens)
			continue
		}
		val, empty := toCty(v.Value)
		if val == cty.NilVal || (empty && !contains(required, v.Name)) {
			continue
		}
		body.SetAttributeValue(v.Name, val)
	}
}

func toCty(v attr.Value) (val cty.Value, empty bool) {
	if v == nil || v.IsNull() || v.IsUnknown() {
		return cty.NilVal, true
	}
	switch value := v.(type) {
	case types.String:
		return cty.StringVal(value.ValueString()), value.ValueString() == ""
	case types.Bool:
		return cty.BoolVal(value.ValueBool()), !value.ValueBool()
	case types.Int64:
		return cty.NumberIntVal(value.ValueInt64()), value.ValueInt64() == 0
	}
	return cty.NilVal, true
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

func passwordHint(block *hclwrite.Block, name string) {
	block.Body().AppendUnstructuredTokens(hclwrite.Tokens{{
		Type:  hclsyntax.TokenComment,
		Bytes: []byte(fmt.Sprintf("# SAP does not return passwords. To manage it, add: password = var.%s_password\n", name)),
	}})
}

// propertiesTokens writes property name -> values, sorted by name.
func propertiesTokens(props map[string][]string) hclwrite.Tokens {
	var attrs []hclwrite.ObjectAttrTokens
	for _, name := range sortedKeys(props) {
		values := make([]cty.Value, 0, len(props[name]))
		for _, v := range props[name] {
			values = append(values, cty.StringVal(v))
		}
		attrs = append(attrs, hclwrite.ObjectAttrTokens{Name: keyTokens(name), Value: hclwrite.TokensForValue(cty.ListVal(values))})
	}
	return hclwrite.TokensForObject(attrs)
}

// keyTokens writes an object key, quoted unless it is a valid identifier.
func keyTokens(key string) hclwrite.Tokens {
	if hclsyntax.ValidIdentifier(key) {
		return hclwrite.TokensForIdentifier(key)
	}
	return hclwrite.TokensForValue(cty.StringVal(key))
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// writeServices writes the arrangement's services and properties exactly as
// they are imported (see communicationarrangement.Snapshot).
func writeServices(body *hclwrite.Body, snapshot communicationarrangement.Snapshot) {
	if len(snapshot.Properties) > 0 {
		body.SetAttributeRaw("properties", propertiesTokens(snapshot.Properties))
	}
	if len(snapshot.InboundServices) > 0 {
		var services []hclwrite.ObjectAttrTokens
		for _, id := range sortedKeys(snapshot.InboundServices) {
			services = append(services, hclwrite.ObjectAttrTokens{
				Name: keyTokens(id),
				Value: hclwrite.TokensForObject([]hclwrite.ObjectAttrTokens{{
					Name: hclwrite.TokensForIdentifier("properties"), Value: propertiesTokens(snapshot.InboundServices[id]),
				}}),
			})
		}
		body.SetAttributeRaw("inbound_services", hclwrite.TokensForObject(services))
	}
	if len(snapshot.OutboundServices) > 0 {
		var services []hclwrite.ObjectAttrTokens
		for _, id := range sortedKeys(snapshot.OutboundServices) {
			svc := snapshot.OutboundServices[id]
			var attrs []hclwrite.ObjectAttrTokens
			for _, v := range svc.Attributes {
				if val, empty := toCty(v.Value); !empty {
					attrs = append(attrs, hclwrite.ObjectAttrTokens{Name: hclwrite.TokensForIdentifier(v.Name), Value: hclwrite.TokensForValue(val)})
				}
			}
			if len(svc.Properties) > 0 {
				attrs = append(attrs, hclwrite.ObjectAttrTokens{Name: hclwrite.TokensForIdentifier("properties"), Value: propertiesTokens(svc.Properties)})
			}
			services = append(services, hclwrite.ObjectAttrTokens{Name: keyTokens(id), Value: hclwrite.TokensForObject(attrs)})
		}
		body.SetAttributeRaw("outbound_services", hclwrite.TokensForObject(services))
	}
}

const providerConfig = `terraform {
  required_version = ">= 1.5" # for import blocks

  required_providers {
    s4communication = {
      source = "implemaab/s4communication"
    }
  }
}

# Set S4_HOST, S4_USERNAME and S4_PASSWORD, or configure them here.
provider "s4communication" {}
`

const importsHeader = `# Import blocks for the exported objects. Run "terraform plan" to check that
# nothing changes and "terraform apply" to import them. This file can be
# deleted afterwards.

`

func (g *generator) result() *Result {
	files := map[string][]byte{"provider.tf": []byte(providerConfig)}
	for _, name := range g.order {
		if f := g.files[name]; len(f.Body().Blocks()) > 0 {
			files[name] = hclwrite.Format(f.Bytes())
		}
	}
	if len(g.imports.Body().Blocks()) > 0 {
		files["imports.tf"] = append([]byte(importsHeader), hclwrite.Format(g.imports.Bytes())...)
	}
	return &Result{Files: files, Counts: g.counts}
}
