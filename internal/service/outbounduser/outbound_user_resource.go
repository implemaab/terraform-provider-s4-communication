package outbounduser

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
var _ resource.Resource = &OutboundUserResource{}
var _ resource.ResourceWithImportState = &OutboundUserResource{}
var _ resource.ResourceWithIdentity = &OutboundUserResource{}

func NewOutboundUserResource() resource.Resource {
	return &OutboundUserResource{}
}

// OutboundUserResource defines the resource implementation.
type OutboundUserResource struct {
	client *s4types.Client
}

// OutboundUserResourceModel describes the resource data model. The api tags
// map each attribute to its SAP property (see package util).
type OutboundUserResourceModel struct {
	ID                               types.String `tfsdk:"id" api:"UUID,readonly"`
	CommunicationSystemUUID          types.String `tfsdk:"communication_system_uuid" api:"CommunicationSystemUUID,noupdate"`
	Name                             types.String `tfsdk:"name" api:"Name"`
	OAuth2ClientID                   types.String `tfsdk:"oauth2_client_id" api:"OAuth2ClientID"`
	OAuth2ClientAuthenticationMethod types.String `tfsdk:"oauth2_client_authentication_method" api:"OAuth2ClientAuthenticationMethod"`
	CertificateID                    types.String `tfsdk:"certificate_id" api:"CertificateID"`
	AuthenticationMethod             types.String `tfsdk:"authentication_method" api:"AuthenticationMethod"`
	Password                         types.String `tfsdk:"password" api:"Password,writeonly"`
}

func (r *OutboundUserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_outbound_user"
}

func (r *OutboundUserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	// SAP rejects a user without an authentication method
	authMethod := util.OptionalString("Authentication Method, defaults to `basic` (max 20 characters)")
	authMethod.Default = stringdefault.StaticString("basic")

	resp.Schema = schema.Schema{
		MarkdownDescription: "Outbound User of a Communication System",
		Attributes: map[string]schema.Attribute{
			"id":                        util.IDAttribute("Outbound User UUID (internal identifier)"),
			"communication_system_uuid": util.RequiredForceNew("UUID of the Communication System this outbound user belongs to. Changing this forces a new outbound user to be created."),
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "User name used to log on to the remote system (max 254 characters)",
			},
			"oauth2_client_id":                    util.OptionalString("OAuth 2.0 Client ID (max 254 characters)"),
			"oauth2_client_authentication_method": util.OptionalString("OAuth 2.0 Client Authentication Method (max 5 characters)"),
			"certificate_id":                      util.OptionalString("Certificate ID (max 6 characters)"),
			"authentication_method":               authMethod,
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Password for the outbound user (max 109 characters). SAP never returns the password, so changes made outside Terraform are not detected and it is not imported.",
			},
		},
	}
}

// IdentitySchema defines the resource identity used by import and list.
func (r *OutboundUserResource) IdentitySchema(ctx context.Context, req resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = util.IdentitySchema("Outbound user UUID")
}

func (r *OutboundUserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	return fmt.Sprintf("OutboundUsers/%s", uuid)
}

func (r *OutboundUserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OutboundUserResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only configured values are sent; SAP fills in the rest
	created, err := util.CreateEntity(ctx, r.client, service, "OutboundUsers", util.CreateBody(&data))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create outbound user, got error: %s", err))
		return
	}

	// Collect the values SAP assigned, re-reading the entity to pick up any
	// fields not included in the create response
	var remote OutboundUserResourceModel
	util.ApplyResponse(&remote, created)
	if remote.ID.ValueString() == "" {
		resp.Diagnostics.AddError("API Error", "SAP did not return a UUID for the created outbound user")
		return
	}
	if entity, found, err := util.GetEntity(ctx, r.client, service, entityPath(remote.ID.ValueString())); err == nil && found {
		util.ApplyResponse(&remote, entity)
	}
	util.ResolveUnknowns(&data, &remote)

	tflog.Trace(ctx, "created an outbound user resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	resp.Diagnostics.Append(util.SetIdentity(ctx, resp.Identity, data.ID)...)
}

func (r *OutboundUserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OutboundUserResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	entity, found, err := util.GetEntity(ctx, r.client, service, entityPath(data.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read outbound user, got error: %s", err))
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	// Update model with current data from API (the password is kept from state)
	util.ApplyResponse(&data, entity)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	resp.Diagnostics.Append(util.SetIdentity(ctx, resp.Identity, data.ID)...)
}

func (r *OutboundUserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state OutboundUserResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// PATCH only the attributes that changed; the password is only sent when
	// it was changed in the configuration
	endpoint := entityPath(state.ID.ValueString())
	if err := util.PatchEntity(ctx, r.client, service, endpoint, util.UpdateBody(&plan, &state)); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update outbound user, got error: %s", err))
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

func (r *OutboundUserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OutboundUserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := util.DeleteEntity(ctx, r.client, service, entityPath(data.ID.ValueString())); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete outbound user, got error: %s", err))
	}
}

// ImportState imports an outbound user by its UUID.
func (r *OutboundUserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root("id"), path.Root("id"), req, resp)
}
