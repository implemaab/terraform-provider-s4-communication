package communicationarrangement

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	s4types "terraform-provider-s4-communication/internal/types"
	"terraform-provider-s4-communication/internal/util"
)

const service = s4types.ServiceCommunicationArrangement

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &CommunicationArrangementResource{}
var _ resource.ResourceWithImportState = &CommunicationArrangementResource{}
var _ resource.ResourceWithIdentity = &CommunicationArrangementResource{}

func NewCommunicationArrangementResource() resource.Resource {
	return &CommunicationArrangementResource{}
}

// CommunicationArrangementResource defines the resource implementation.
type CommunicationArrangementResource struct {
	client *s4types.Client
}

// CommunicationArrangementResourceModel describes the resource data model.
// The api tags map each attribute to its SAP property (see package util); the
// users, services and properties are nested entities and are mapped by hand.
type CommunicationArrangementResourceModel struct {
	ID                      types.String `tfsdk:"id" api:"UUID,readonly"`
	CommunicationScenarioID types.String `tfsdk:"communication_scenario_id" api:"CommunicationScenarioID,noupdate"`
	CommunicationSystemID   types.String `tfsdk:"communication_system_id" api:"CommunicationSystemID"`
	Name                    types.String `tfsdk:"name" api:"Name"`
	Description             types.String `tfsdk:"description" api:"Description"`
	InboundUser             types.String `tfsdk:"inbound_user"`
	OutboundUser            types.String `tfsdk:"outbound_user"`
	Properties              types.Map    `tfsdk:"properties"`
	InboundServices         types.Map    `tfsdk:"inbound_services"`
	OutboundServices        types.Map    `tfsdk:"outbound_services"`
}

// nestedUser describes one of the arrangement's nested user entities.
type nestedUser struct {
	navigation string // navigation property on the arrangement
	key        string // property holding the communication system user UUID
}

var (
	inboundUser  = nestedUser{"InboundUser", "CommunicationSystemInboundUserUUID"}
	outboundUser = nestedUser{"OutboundUser", "CommunicationSystemOutboundUserUUID"}
)

func (r *CommunicationArrangementResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_communication_arrangement"
}

// requiresReplaceWhenAddedOrRemoved forces a new arrangement when a nested
// user is added or removed: SAP only allows them to be created together with
// the arrangement, while an existing one can be switched in place.
var requiresReplaceWhenAddedOrRemoved = stringplanmodifier.RequiresReplaceIf(
	func(ctx context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
		resp.RequiresReplace = req.StateValue.IsNull() != req.PlanValue.IsNull()
	},
	"Adding or removing the user forces a new arrangement to be created.",
	"Adding or removing the user forces a new arrangement to be created.",
)

func (r *CommunicationArrangementResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := servicesSchema()
	for name, attribute := range map[string]schema.Attribute{
		"id":                        util.IDAttribute("Communication Arrangement UUID"),
		"communication_scenario_id": util.RequiredForceNew("Communication Scenario ID, e.g. `SAP_COM_0008`. Changing this forces a new arrangement to be created."),
		"communication_system_id": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "Communication System ID (the `communication_system_id` of the system, not its UUID)",
		},
		"name": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "Communication Arrangement Name",
		},
		"description": util.OptionalString("Communication Arrangement Description"),
		"inbound_user": schema.StringAttribute{
			Optional:            true,
			PlanModifiers:       []planmodifier.String{requiresReplaceWhenAddedOrRemoved},
			MarkdownDescription: "UUID of the communication system's inbound user (`s4communication_inbound_user.id`). Adding or removing it forces a new arrangement to be created.",
		},
		"outbound_user": schema.StringAttribute{
			Optional:            true,
			PlanModifiers:       []planmodifier.String{requiresReplaceWhenAddedOrRemoved},
			MarkdownDescription: "UUID of the communication system's outbound user (`s4communication_outbound_user.id`). Adding or removing it forces a new arrangement to be created.",
		},
	} {
		attributes[name] = attribute
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an SAP S/4HANA Cloud Communication Arrangement, including the properties of its services.",
		Attributes:          attributes,
	}
}

// IdentitySchema defines the resource identity used by import and list.
func (r *CommunicationArrangementResource) IdentitySchema(ctx context.Context, req resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = util.IdentitySchema("Communication arrangement UUID")
}

func (r *CommunicationArrangementResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*s4types.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *s4types.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func entityPath(uuid string) string {
	return fmt.Sprintf("CommunicationArrangements/%s", uuid)
}

// readArrangement fetches an arrangement together with its nested users,
// services and properties.
func (r *CommunicationArrangementResource) readArrangement(ctx context.Context, uuid string) (map[string]interface{}, bool, error) {
	return util.GetEntity(ctx, r.client, service, entityPath(uuid)+"?$expand="+expandAll)
}

// applyResponse copies an expanded arrangement response into the model.
func applyResponse(data *CommunicationArrangementResourceModel, entity map[string]interface{}) {
	util.ApplyResponse(data, entity)
	data.InboundUser = nestedUserValue(entity, inboundUser)
	data.OutboundUser = nestedUserValue(entity, outboundUser)
}

func nestedUserValue(entity map[string]interface{}, u nestedUser) types.String {
	if nested, ok := entity[u.navigation].(map[string]interface{}); ok {
		if id, ok := nested[u.key].(string); ok && id != "" {
			return types.StringValue(id)
		}
	}
	return types.StringNull()
}

func nestedUserUUID(entity map[string]interface{}, u nestedUser) string {
	if nested, ok := entity[u.navigation].(map[string]interface{}); ok {
		if id, ok := nested["UUID"].(string); ok {
			return id
		}
	}
	return ""
}

func (r *CommunicationArrangementResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CommunicationArrangementResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only configured values are sent; the users are created as nested entities
	body := util.CreateBody(&data)
	if util.IsSet(data.InboundUser) {
		body[inboundUser.navigation] = map[string]interface{}{inboundUser.key: data.InboundUser.ValueString()}
	}
	if util.IsSet(data.OutboundUser) {
		body[outboundUser.navigation] = map[string]interface{}{outboundUser.key: data.OutboundUser.ValueString()}
	}

	// Configured outbound services are sent with the create request: SAP
	// validates the services it creates from the scenario (e.g. SAP_COM_0008
	// requires replication settings on active services), so a service that
	// is configured inactive must be inactive from the start.
	outbound, d := outboundServicesBody(ctx, data.OutboundServices)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(outbound) > 0 {
		body["OutboundServices"] = outbound
	}

	created, err := util.CreateEntity(ctx, r.client, service, "CommunicationArrangements", body)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create communication arrangement, got error: %s", err))
		return
	}

	// Collect the values SAP assigned, re-reading the entity to pick up any
	// fields not included in the create response
	var remote CommunicationArrangementResourceModel
	util.ApplyResponse(&remote, created)
	if remote.ID.ValueString() == "" {
		resp.Diagnostics.AddError("API Error", "SAP did not return a UUID for the created communication arrangement")
		return
	}

	// Configure the services and properties SAP created from the scenario.
	// The arrangement is saved to state even if this fails, so that it is
	// not orphaned.
	data.ID = remote.ID
	servicesDiags := r.applyServices(ctx, remote.ID.ValueString(), &data)

	if entity, found, err := r.readArrangement(ctx, remote.ID.ValueString()); err == nil && found {
		applyResponse(&remote, entity)
		resp.Diagnostics.Append(refreshServices(ctx, &data, entity, true)...)
	}
	util.ResolveUnknowns(&data, &remote)
	resp.Diagnostics.Append(servicesDiags...)

	tflog.Trace(ctx, "created a communication arrangement resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	resp.Diagnostics.Append(util.SetIdentity(ctx, resp.Identity, data.ID)...)
}

func (r *CommunicationArrangementResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CommunicationArrangementResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	entity, found, err := r.readArrangement(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read communication arrangement, got error: %s", err))
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	// Update model with current data from API. Right after an import, all
	// services and properties are taken into state (see Snapshot); otherwise
	// only the ones already in state are refreshed.
	applyResponse(&data, entity)
	imported, diags := req.Private.GetKey(ctx, importedKey)
	resp.Diagnostics.Append(diags...)
	if string(imported) == "true" {
		resp.Diagnostics.Append(importServices(ctx, &data, entity)...)
		resp.Diagnostics.Append(resp.Private.SetKey(ctx, importedKey, nil)...)
	} else {
		resp.Diagnostics.Append(refreshServices(ctx, &data, entity, false)...)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	resp.Diagnostics.Append(util.SetIdentity(ctx, resp.Identity, data.ID)...)
}

func (r *CommunicationArrangementResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state CommunicationArrangementResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// PATCH only the attributes that changed
	uuid := state.ID.ValueString()
	if err := util.PatchEntity(ctx, r.client, service, entityPath(uuid), util.UpdateBody(&plan, &state)); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update communication arrangement, got error: %s", err))
		return
	}

	// Switching to another inbound/outbound user is a PATCH on the nested
	// entity (adding or removing one forces replacement, see the schema)
	if !plan.InboundUser.Equal(state.InboundUser) || !plan.OutboundUser.Equal(state.OutboundUser) {
		current, _, err := r.readArrangement(ctx, uuid)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read communication arrangement, got error: %s", err))
			return
		}
		for _, change := range []struct {
			user        nestedUser
			plan, state types.String
		}{
			{inboundUser, plan.InboundUser, state.InboundUser},
			{outboundUser, plan.OutboundUser, state.OutboundUser},
		} {
			if change.plan.Equal(change.state) || !util.IsSet(change.plan) {
				continue
			}
			nestedUUID := nestedUserUUID(current, change.user)
			if nestedUUID == "" {
				resp.Diagnostics.AddError("API Error", fmt.Sprintf("Communication arrangement %s has no %s to update", uuid, change.user.navigation))
				return
			}
			nestedPath := fmt.Sprintf("%ss/%s", change.user.navigation, nestedUUID)
			body := map[string]interface{}{change.user.key: change.plan.ValueString()}
			if err := util.PatchEntity(ctx, r.client, service, nestedPath, body); err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update the %s of the communication arrangement, got error: %s", change.user.navigation, err))
				return
			}
		}
	}

	resp.Diagnostics.Append(r.applyServices(ctx, uuid, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fill in values SAP computed during the update
	remote := state
	if entity, found, err := r.readArrangement(ctx, uuid); err == nil && found {
		applyResponse(&remote, entity)
		resp.Diagnostics.Append(refreshServices(ctx, &plan, entity, true)...)
	}
	util.ResolveUnknowns(&plan, &remote)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	resp.Diagnostics.Append(util.SetIdentity(ctx, resp.Identity, plan.ID)...)
}

func (r *CommunicationArrangementResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CommunicationArrangementResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := util.DeleteEntity(ctx, r.client, service, entityPath(data.ID.ValueString())); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete communication arrangement, got error: %s", err))
	}
}

// importedKey marks, in private state, an arrangement that was just imported.
const importedKey = "imported"

// ImportState imports a communication arrangement by its UUID, including all
// of its services and properties.
func (r *CommunicationArrangementResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root("id"), path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, importedKey, []byte("true"))...)
}
