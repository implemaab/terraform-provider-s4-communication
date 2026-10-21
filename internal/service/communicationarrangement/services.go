package communicationarrangement

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-s4communication/internal/util"
)

// The services of an arrangement are created by SAP from the communication
// scenario. Only the services and properties listed in the configuration are
// managed; everything else is left untouched so that it cannot cause drift.

// expandAll is the $expand option that returns an arrangement with its users,
// services, properties and property values.
const expandAll = "InboundUser,OutboundUser," +
	"InboundServices($expand=Properties($expand=Values))," +
	"OutboundServices($expand=Properties($expand=Values))," +
	"Properties($expand=Values)"

// propertiesType is the Terraform type of a properties attribute: property
// name to its set of values.
var propertiesType = types.MapType{ElemType: types.SetType{ElemType: types.StringType}}

// propertyKind describes where the properties of an arrangement or service
// and their values live in the API.
type propertyKind struct {
	propertySet string // entity set of the properties
	valueSet    string // entity set of the property values
}

var (
	arrangementProperties     = propertyKind{"Properties", "PropertyValues"}
	inboundServiceProperties  = propertyKind{"InboundServiceProperties", "InboundServicePropertyValues"}
	outboundServiceProperties = propertyKind{"OutboundServiceProperties", "OutboundServicePropertyValues"}
)

// inboundServiceModel describes an inbound service. SAP does not allow
// inbound services to be changed, only their property values.
type inboundServiceModel struct {
	Properties types.Map `tfsdk:"properties"`
}

var inboundServiceType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"properties": propertiesType,
}}

// outboundServiceModel describes an outbound service. The api tags map the
// attributes to the SAP properties (see package util).
type outboundServiceModel struct {
	Status             types.String `tfsdk:"status" api:"Status"`
	URLPath            types.String `tfsdk:"url_path" api:"URLPath"`
	TCPPort            types.Int64  `tfsdk:"tcp_port" api:"TCPPort"`
	SOAPWSRMVersion    types.String `tfsdk:"soap_wsrm_version" api:"SOAPWSRMVersion"`
	JobTimezoneCode    types.String `tfsdk:"job_timezone_code" api:"JobTimezoneCode"`
	JobStartTime       types.String `tfsdk:"job_start_time" api:"JobStartTime"`
	JobPackageSize     types.Int64  `tfsdk:"job_package_size" api:"JobPackageSize"`
	JobRecurrenceValue types.Int64  `tfsdk:"job_recurrence_value" api:"JobRecurrenceValue"`
	JobFrequency       types.String `tfsdk:"job_frequency" api:"JobFrequency"`
	Properties         types.Map    `tfsdk:"properties"`
}

var outboundServiceType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"status":               types.StringType,
	"url_path":             types.StringType,
	"tcp_port":             types.Int64Type,
	"soap_wsrm_version":    types.StringType,
	"job_timezone_code":    types.StringType,
	"job_start_time":       types.StringType,
	"job_package_size":     types.Int64Type,
	"job_recurrence_value": types.Int64Type,
	"job_frequency":        types.StringType,
	"properties":           propertiesType,
}}

func propertiesAttribute(description string) schema.MapAttribute {
	return schema.MapAttribute{
		ElementType:         types.SetType{ElemType: types.StringType},
		Optional:            true,
		MarkdownDescription: description + " Map of property name to its values. Only the listed properties are managed; properties that are not listed keep their values. Set a property to `[]` to remove its values.",
	}
}

func servicesSchema() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"properties": propertiesAttribute("Properties of the communication arrangement."),
		"inbound_services": schema.MapNestedAttribute{
			Optional:            true,
			MarkdownDescription: "Inbound services of the arrangement to manage, keyed by inbound service ID. Services that are not listed are left as SAP created them.",
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"properties": propertiesAttribute("Properties of the inbound service."),
				},
			},
		},
		"outbound_services": schema.MapNestedAttribute{
			Optional:            true,
			MarkdownDescription: "Outbound services of the arrangement to manage, keyed by outbound service ID. Services that are not listed are left as SAP created them. Attributes that are not configured keep the value SAP assigned.",
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"status":               util.NestedOptionalString("Service status: `active` or `inactive`"),
					"url_path":             util.NestedOptionalString("URL path of the service on the communication system"),
					"tcp_port":             util.NestedOptionalInt64("TCP port, if different from the communication system"),
					"soap_wsrm_version":    util.NestedOptionalString("SOAP WS-RM version"),
					"job_timezone_code":    util.NestedOptionalString("Time zone of the scheduled job"),
					"job_start_time":       util.NestedOptionalString("Start time of the scheduled job (`HH:MM:SS`)"),
					"job_package_size":     util.NestedOptionalInt64("Package size of the scheduled job"),
					"job_recurrence_value": util.NestedOptionalInt64("Recurrence value of the scheduled job"),
					"job_frequency":        util.NestedOptionalString("Frequency of the scheduled job"),
					"properties":           propertiesAttribute("Properties of the outbound service."),
				},
			},
		},
	}
}

// --- reading the expanded API response ---

func entityList(entity map[string]interface{}, nav string) []map[string]interface{} {
	items, _ := entity[nav].([]interface{})
	out := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

func indexBy(items []map[string]interface{}, key string) map[string]map[string]interface{} {
	out := map[string]map[string]interface{}{}
	for _, item := range items {
		if k, ok := item[key].(string); ok {
			out[k] = item
		}
	}
	return out
}

func stringField(m map[string]interface{}, key string) string {
	s, _ := m[key].(string)
	return s
}

// readProperties returns the current values of the named properties of an
// expanded arrangement or service. Unknown properties have no values.
func readProperties(ctx context.Context, parent map[string]interface{}, names []string) (types.Map, diag.Diagnostics) {
	remote := indexBy(entityList(parent, "Properties"), "Name")
	values := map[string][]string{}
	for _, name := range names {
		values[name] = []string{}
		for _, v := range entityList(remote[name], "Values") {
			values[name] = append(values[name], stringField(v, "Value"))
		}
	}
	return types.MapValueFrom(ctx, propertiesType.ElemType, values)
}

func mapKeys(m types.Map) []string {
	keys := make([]string, 0, len(m.Elements()))
	for k := range m.Elements() {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// refreshServices updates the properties and services in data from an
// expanded arrangement. Only entries already present in data are read.
// Known values in data are kept when keepKnown is set (after an apply), and
// replaced by the API values otherwise (on refresh).
func refreshServices(ctx context.Context, data *CommunicationArrangementResourceModel, entity map[string]interface{}, keepKnown bool) diag.Diagnostics {
	var diags diag.Diagnostics

	refreshProps := func(current types.Map, parent map[string]interface{}) types.Map {
		if current.IsNull() || (keepKnown && !current.IsUnknown()) {
			return current
		}
		props, d := readProperties(ctx, parent, mapKeys(current))
		diags.Append(d...)
		return props
	}

	data.Properties = refreshProps(data.Properties, entity)

	if !data.InboundServices.IsNull() && !data.InboundServices.IsUnknown() {
		remote := indexBy(entityList(entity, "InboundServices"), "ID")
		services := map[string]inboundServiceModel{}
		diags.Append(data.InboundServices.ElementsAs(ctx, &services, false)...)
		for id, svc := range services {
			svc.Properties = refreshProps(svc.Properties, remote[id])
			services[id] = svc
		}
		var d diag.Diagnostics
		data.InboundServices, d = types.MapValueFrom(ctx, inboundServiceType, services)
		diags.Append(d...)
	}

	if !data.OutboundServices.IsNull() && !data.OutboundServices.IsUnknown() {
		remote := indexBy(entityList(entity, "OutboundServices"), "ID")
		services := map[string]outboundServiceModel{}
		diags.Append(data.OutboundServices.ElementsAs(ctx, &services, false)...)
		for id, svc := range services {
			if keepKnown {
				var fromAPI outboundServiceModel
				util.ApplyResponse(&fromAPI, remote[id])
				fromAPI.Properties = svc.Properties
				util.ResolveUnknowns(&svc, &fromAPI)
			} else {
				util.ApplyResponse(&svc, remote[id])
			}
			svc.Properties = refreshProps(svc.Properties, remote[id])
			services[id] = svc
		}
		var d diag.Diagnostics
		data.OutboundServices, d = types.MapValueFrom(ctx, outboundServiceType, services)
		diags.Append(d...)
	}
	return diags
}

// --- applying the configuration ---

// applyServices makes the configured services and properties of the
// arrangement match plan. Services and properties that are not configured
// are left unchanged.
func (r *CommunicationArrangementResource) applyServices(ctx context.Context, uuid string, plan *CommunicationArrangementResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	planProps, d := propertyValues(ctx, plan.Properties)
	diags.Append(d...)
	if diags.HasError() || (len(planProps) == 0 && plan.InboundServices.IsNull() && plan.OutboundServices.IsNull()) {
		return diags
	}

	entity, _, err := r.readArrangement(ctx, uuid)
	if err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to read communication arrangement, got error: %s", err))
		return diags
	}

	if err := r.applyProperties(ctx, entity, "CommunicationArrangements/"+uuid, arrangementProperties, planProps); err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to update the properties of the communication arrangement, got error: %s", err))
		return diags
	}

	diags.Append(r.applyInboundServices(ctx, uuid, entity, plan)...)
	if diags.HasError() {
		return diags
	}
	diags.Append(r.applyOutboundServices(ctx, uuid, entity, plan)...)
	return diags
}

func propertyValues(ctx context.Context, m types.Map) (map[string][]string, diag.Diagnostics) {
	values := map[string][]string{}
	if m.IsNull() || m.IsUnknown() {
		return values, nil
	}
	diags := m.ElementsAs(ctx, &values, false)
	return values, diags
}

func (r *CommunicationArrangementResource) applyInboundServices(ctx context.Context, uuid string, entity map[string]interface{}, plan *CommunicationArrangementResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	planned := map[string]inboundServiceModel{}
	if !plan.InboundServices.IsNull() {
		diags.Append(plan.InboundServices.ElementsAs(ctx, &planned, false)...)
	}
	if diags.HasError() {
		return diags
	}

	remote := indexBy(entityList(entity, "InboundServices"), "ID")
	for _, id := range sortedKeys(planned, nil) {
		remoteSvc, exists := remote[id]
		if !exists {
			created, err := util.CreateEntity(ctx, r.client, service, fmt.Sprintf("CommunicationArrangements/%s/InboundServices", uuid), map[string]interface{}{"ID": id})
			if err != nil {
				diags.AddError("Client Error", fmt.Sprintf("Unable to add inbound service %s, got error: %s", id, err))
				return diags
			}
			remoteSvc = created
		}

		planProps, d := propertyValues(ctx, planned[id].Properties)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		parent := "InboundServices/" + stringField(remoteSvc, "UUID")
		if err := r.applyProperties(ctx, remoteSvc, parent, inboundServiceProperties, planProps); err != nil {
			diags.AddError("Client Error", fmt.Sprintf("Unable to update the properties of inbound service %s, got error: %s", id, err))
			return diags
		}
	}
	return diags
}

func (r *CommunicationArrangementResource) applyOutboundServices(ctx context.Context, uuid string, entity map[string]interface{}, plan *CommunicationArrangementResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	planned := map[string]outboundServiceModel{}
	if !plan.OutboundServices.IsNull() {
		diags.Append(plan.OutboundServices.ElementsAs(ctx, &planned, false)...)
	}
	if diags.HasError() {
		return diags
	}

	remote := indexBy(entityList(entity, "OutboundServices"), "ID")
	for _, id := range sortedKeys(planned, nil) {
		svc := planned[id]
		remoteSvc, exists := remote[id]
		if !exists {
			body := util.CreateBody(&svc)
			body["ID"] = id
			created, err := util.CreateEntity(ctx, r.client, service, fmt.Sprintf("CommunicationArrangements/%s/OutboundServices", uuid), body)
			if err != nil {
				diags.AddError("Client Error", fmt.Sprintf("Unable to add outbound service %s, got error: %s", id, err))
				return diags
			}
			remoteSvc = created
		} else {
			// PATCH the configured attributes that differ from SAP
			var current outboundServiceModel
			util.ApplyResponse(&current, remoteSvc)
			current.Properties = svc.Properties
			endpoint := "OutboundServices/" + stringField(remoteSvc, "UUID")
			if err := util.PatchEntity(ctx, r.client, service, endpoint, util.UpdateBody(&svc, &current)); err != nil {
				diags.AddError("Client Error", fmt.Sprintf("Unable to update outbound service %s, got error: %s", id, err))
				return diags
			}
		}

		planProps, d := propertyValues(ctx, svc.Properties)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		parent := "OutboundServices/" + stringField(remoteSvc, "UUID")
		if err := r.applyProperties(ctx, remoteSvc, parent, outboundServiceProperties, planProps); err != nil {
			diags.AddError("Client Error", fmt.Sprintf("Unable to update the properties of outbound service %s, got error: %s", id, err))
			return diags
		}
	}
	return diags
}

// outboundServicesBody builds the deep-insert body of the configured outbound
// services and their properties for the create request.
func outboundServicesBody(ctx context.Context, services types.Map) ([]interface{}, diag.Diagnostics) {
	if services.IsNull() || services.IsUnknown() {
		return nil, nil
	}
	planned := map[string]outboundServiceModel{}
	diags := services.ElementsAs(ctx, &planned, false)
	if diags.HasError() {
		return nil, diags
	}
	var list []interface{}
	for _, id := range sortedKeys(planned, nil) {
		svc := planned[id]
		body := util.CreateBody(&svc)
		body["ID"] = id
		props, d := propertyValues(ctx, svc.Properties)
		diags.Append(d...)
		if len(props) > 0 {
			body["Properties"] = propertiesBody(props)
		}
		list = append(list, body)
	}
	return list, diags
}

// propertiesBody builds the deep-insert body of properties and their values.
func propertiesBody(props map[string][]string) []interface{} {
	var list []interface{}
	for _, name := range sortedKeys(props, nil) {
		values := make([]interface{}, 0, len(props[name]))
		for _, v := range props[name] {
			values = append(values, map[string]interface{}{"Value": v})
		}
		list = append(list, map[string]interface{}{"Name": name, "Values": values})
	}
	return list
}

// applyProperties makes the values of the planned properties of parent (an
// expanded arrangement or service, reachable at parentPath) match planned.
// An empty list of values removes all values of a property.
func (r *CommunicationArrangementResource) applyProperties(ctx context.Context, parent map[string]interface{}, parentPath string, kind propertyKind, planned map[string][]string) error {
	remote := indexBy(entityList(parent, "Properties"), "Name")

	for _, name := range sortedKeys(planned, nil) {
		want := planned[name]
		prop, exists := remote[name]

		if !exists {
			if len(want) == 0 {
				continue
			}
			values := make([]interface{}, 0, len(want))
			for _, v := range want {
				values = append(values, map[string]interface{}{"Value": v})
			}
			body := map[string]interface{}{"Name": name, "Values": values}
			if _, err := util.CreateEntity(ctx, r.client, service, parentPath+"/Properties", body); err != nil {
				return fmt.Errorf("property %s: %w", name, err)
			}
			continue
		}

		wanted := map[string]bool{}
		for _, v := range want {
			wanted[v] = true
		}
		have := map[string]bool{}
		for _, v := range entityList(prop, "Values") {
			value := stringField(v, "Value")
			if wanted[value] && !have[value] {
				have[value] = true
				continue
			}
			if err := util.DeleteEntity(ctx, r.client, service, fmt.Sprintf("%s/%s", kind.valueSet, stringField(v, "UUID"))); err != nil {
				return fmt.Errorf("property %s: %w", name, err)
			}
		}
		for _, v := range want {
			if have[v] {
				continue
			}
			endpoint := fmt.Sprintf("%s/%s/Values", kind.propertySet, stringField(prop, "UUID"))
			if _, err := util.CreateEntity(ctx, r.client, service, endpoint, map[string]interface{}{"Value": v}); err != nil {
				return fmt.Errorf("property %s: %w", name, err)
			}
			have[v] = true
		}
	}
	return nil
}

// sortedKeys returns the union of the keys of a and b in a stable order.
func sortedKeys[V any](a, b map[string]V) []string {
	seen := map[string]bool{}
	var keys []string
	for _, m := range []map[string]V{a, b} {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)
	return keys
}

// --- import and export ---

// Snapshot describes the services and properties of an arrangement that are
// taken into state on import, and that the exporter writes to configuration:
// properties that have values, inbound services with such properties, and
// every outbound service. Keeping both on this one definition means exported
// configuration plans without changes after import.
type Snapshot struct {
	Properties       map[string][]string            // property name -> values
	InboundServices  map[string]map[string][]string // service ID -> properties
	OutboundServices map[string]OutboundServiceSnapshot
}

// OutboundServiceSnapshot is an outbound service in a Snapshot.
type OutboundServiceSnapshot struct {
	Attributes []util.ConfigValue  // configurable attributes as read from SAP
	Properties map[string][]string // property name -> values
}

// SnapshotServices builds the Snapshot of an expanded arrangement.
func SnapshotServices(entity map[string]interface{}) Snapshot {
	snapshot := Snapshot{
		Properties:       nonEmptyProperties(entity),
		InboundServices:  map[string]map[string][]string{},
		OutboundServices: map[string]OutboundServiceSnapshot{},
	}
	for _, svc := range entityList(entity, "InboundServices") {
		if props := nonEmptyProperties(svc); len(props) > 0 {
			snapshot.InboundServices[stringField(svc, "ID")] = props
		}
	}
	for _, svc := range entityList(entity, "OutboundServices") {
		var model outboundServiceModel
		util.ApplyResponse(&model, svc)
		snapshot.OutboundServices[stringField(svc, "ID")] = OutboundServiceSnapshot{
			Attributes: util.ConfigValues(&model),
			Properties: nonEmptyProperties(svc),
		}
	}
	return snapshot
}

// ExpandAll is the $expand option that returns an arrangement with everything
// SnapshotServices needs.
const ExpandAll = expandAll

func nonEmptyProperties(parent map[string]interface{}) map[string][]string {
	out := map[string][]string{}
	for _, prop := range entityList(parent, "Properties") {
		var values []string
		for _, v := range entityList(prop, "Values") {
			values = append(values, stringField(v, "Value"))
		}
		if len(values) > 0 {
			sort.Strings(values)
			out[stringField(prop, "Name")] = values
		}
	}
	return out
}

// importServices sets the services and properties of data from the Snapshot
// of an expanded arrangement.
func importServices(ctx context.Context, data *CommunicationArrangementResourceModel, entity map[string]interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	snapshot := SnapshotServices(entity)

	toMap := func(props map[string][]string) types.Map {
		if len(props) == 0 {
			return types.MapNull(propertiesType.ElemType)
		}
		m, d := types.MapValueFrom(ctx, propertiesType.ElemType, props)
		diags.Append(d...)
		return m
	}

	data.Properties = toMap(snapshot.Properties)

	data.InboundServices = types.MapNull(inboundServiceType)
	if len(snapshot.InboundServices) > 0 {
		services := map[string]inboundServiceModel{}
		for id, props := range snapshot.InboundServices {
			services[id] = inboundServiceModel{Properties: toMap(props)}
		}
		var d diag.Diagnostics
		data.InboundServices, d = types.MapValueFrom(ctx, inboundServiceType, services)
		diags.Append(d...)
	}

	data.OutboundServices = types.MapNull(outboundServiceType)
	if len(snapshot.OutboundServices) > 0 {
		remote := indexBy(entityList(entity, "OutboundServices"), "ID")
		services := map[string]outboundServiceModel{}
		for id, svc := range snapshot.OutboundServices {
			var model outboundServiceModel
			util.ApplyResponse(&model, remote[id])
			model.Properties = toMap(svc.Properties)
			services[id] = model
		}
		var d diag.Diagnostics
		data.OutboundServices, d = types.MapValueFrom(ctx, outboundServiceType, services)
		diags.Append(d...)
	}
	return diags
}
