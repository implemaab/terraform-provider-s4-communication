package inbounduser

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	s4types "terraform-provider-s4communication/internal/types"
	"terraform-provider-s4communication/internal/util"
)

const service = s4types.ServiceCommunicationSystem

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &InboundUserResource{}
var _ resource.ResourceWithImportState = &InboundUserResource{}
var _ resource.ResourceWithIdentity = &InboundUserResource{}

func NewInboundUserResource() resource.Resource {
	return &InboundUserResource{}
}

// InboundUserResource defines the resource implementation.
type InboundUserResource struct {
	client *s4types.Client
}

// InboundUserResourceModel describes the resource data model. The api tags
// map each attribute to its SAP property (see package util).
type InboundUserResourceModel struct {
	ID                            types.String `tfsdk:"id" api:"UUID,readonly"`
	CommunicationSystemUUID       types.String `tfsdk:"communication_system_uuid" api:"CommunicationSystemUUID,noupdate"`
	CommunicationUserID           types.String `tfsdk:"communication_user_id" api:"CommunicationUserID"`
	OAuth2ClientID                types.String `tfsdk:"oauth2_client_id" api:"OAuth2ClientID"`
	OAuth2GrantType               types.String `tfsdk:"oauth2_grant_type" api:"OAuth2GrantType"`
	IsOAuth2RefreshTokenAllowed   types.Bool   `tfsdk:"is_oauth2_refresh_token_allowed" api:"IsOAuth2RefreshTokenAllowed"`
	OAuth2RefreshTokenExpiryValue types.Int64  `tfsdk:"oauth2_refresh_token_expiry_value" api:"OAuth2RefreshTokenExpiryValue"`
	OAuth2RefreshTokenExpiryUnit  types.String `tfsdk:"oauth2_refresh_token_expiry_unit" api:"OAuth2RefreshTokenExpiryUnit"`
	AuthenticationMethod          types.String `tfsdk:"authentication_method" api:"AuthenticationMethod"`
}

func (r *InboundUserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_inbound_user"
}

func (r *InboundUserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	// SAP rejects a user without an authentication method
	authMethod := util.OptionalString("Authentication Method, defaults to `basic` (max 20 characters)")
	authMethod.Default = stringdefault.StaticString("basic")

	resp.Schema = schema.Schema{
		MarkdownDescription: "Inbound User of a Communication System",
		Attributes: map[string]schema.Attribute{
			"id":                        util.IDAttribute("Inbound User UUID (internal identifier)"),
			"communication_system_uuid": util.RequiredForceNew("UUID of the Communication System this inbound user belongs to. Changing this forces a new inbound user to be created."),
			"communication_user_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Communication User ID (max 12 characters)",
			},
			"oauth2_client_id":                  util.OptionalString("OAuth 2.0 Client ID (max 40 characters)"),
			"oauth2_grant_type":                 util.OptionalString("OAuth 2.0 Grant Type (max 20 characters)"),
			"is_oauth2_refresh_token_allowed":   util.OptionalBool("OAuth 2.0 Refresh Token Allowed"),
			"oauth2_refresh_token_expiry_value": util.OptionalInt64("OAuth 2.0 Refresh Token Expiry Value"),
			"oauth2_refresh_token_expiry_unit":  util.OptionalString("OAuth 2.0 Refresh Token Expiry Unit (max 6 characters)"),
			"authentication_method":             authMethod,
		},
	}
}

// IdentitySchema defines the resource identity used by import and list.
func (r *InboundUserResource) IdentitySchema(ctx context.Context, req resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = util.IdentitySchema("Inbound user UUID")
}

func (r *InboundUserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	return fmt.Sprintf("InboundUsers/%s", uuid)
}

func (r *InboundUserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data InboundUserResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only configured values are sent; SAP fills in the rest
	created, err := util.CreateEntity(ctx, r.client, service, "InboundUsers", util.CreateBody(&data))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create inbound user, got error: %s", err))
		return
	}

	// Collect the values SAP assigned, re-reading the entity to pick up any
	// fields not included in the create response
	var remote InboundUserResourceModel
	util.ApplyResponse(&remote, created)
	if remote.ID.ValueString() == "" {
		resp.Diagnostics.AddError("API Error", "SAP did not return a UUID for the created inbound user")
		return
	}
	if entity, found, err := util.GetEntity(ctx, r.client, service, entityPath(remote.ID.ValueString())); err == nil && found {
		util.ApplyResponse(&remote, entity)
	}
	util.ResolveUnknowns(&data, &remote)

	tflog.Trace(ctx, "created an inbound user resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	resp.Diagnostics.Append(util.SetIdentity(ctx, resp.Identity, data.ID)...)
}

func (r *InboundUserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data InboundUserResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	entity, found, err := util.GetEntity(ctx, r.client, service, entityPath(data.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read inbound user, got error: %s", err))
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	// Update model with current data from API
	util.ApplyResponse(&data, entity)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	resp.Diagnostics.Append(util.SetIdentity(ctx, resp.Identity, data.ID)...)
}

func (r *InboundUserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state InboundUserResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// PATCH only the attributes that changed
	endpoint := entityPath(state.ID.ValueString())
	if err := util.PatchEntity(ctx, r.client, service, endpoint, util.UpdateBody(&plan, &state)); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update inbound user, got error: %s", err))
		return
	}

	// Fill in values SAP computed during the update
	remote := state
	if entity, found, err := util.GetEntity(ctx, r.client, service, endpoint); err == nil && found {
		util.ApplyResponse(&remote, entity)
	}
	util.ResolveUnknowns(&plan, &remote)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	resp.Diagnostics.Append(util.SetIdentity(ctx, resp.Identity, plan.ID)...)
}

func (r *InboundUserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data InboundUserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := util.DeleteEntity(ctx, r.client, service, entityPath(data.ID.ValueString())); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete inbound user, got error: %s", err))
	}
}

// ImportState imports an inbound user by its UUID.
func (r *InboundUserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root("id"), path.Root("id"), req, resp)
}
