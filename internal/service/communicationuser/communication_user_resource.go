package communicationuser

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	s4types "terraform-provider-s4communication/internal/types"
	"terraform-provider-s4communication/internal/util"
)

const service = s4types.ServiceCommunicationUser

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &CommunicationUserResource{}
var _ resource.ResourceWithImportState = &CommunicationUserResource{}
var _ resource.ResourceWithIdentity = &CommunicationUserResource{}

func NewCommunicationUserResource() resource.Resource {
	return &CommunicationUserResource{}
}

// CommunicationUserResource defines the resource implementation.
type CommunicationUserResource struct {
	client *s4types.Client
}

// CommunicationUserResourceModel describes the resource data model. The api
// tags map each attribute to its SAP property (see package util).
type CommunicationUserResourceModel struct {
	ID          types.String `tfsdk:"id" api:"ID,readonly"`
	Name        types.String `tfsdk:"name" api:"Name"`
	Type        types.String `tfsdk:"type" api:"Type,noupdate"`
	Description types.String `tfsdk:"description" api:"Description"`
	IsLocked    types.Bool   `tfsdk:"is_locked" api:"IsLocked"`
	Password    types.String `tfsdk:"password" api:"Password,writeonly"`
}

func (r *CommunicationUserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_communication_user"
}

func (r *CommunicationUserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	userType := util.OptionalString("Communication User Type, defaults to `customer_communication`. Changing this forces a new user to be created.")
	userType.Default = stringdefault.StaticString("customer_communication")
	userType.PlanModifiers = append(userType.PlanModifiers, stringplanmodifier.RequiresReplace())

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an SAP S/4HANA Cloud Communication User.",

		Attributes: map[string]schema.Attribute{
			"id": util.IDAttribute("Communication User ID, assigned by SAP"),
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Communication User Name",
			},
			"type":        userType,
			"description": util.OptionalString("Communication User Description"),
			"is_locked":   util.OptionalBool("Is the user locked"),
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Communication User Password. SAP requires more than 20 characters with at least one uppercase letter, one lowercase letter and one special character. SAP never returns the password, so changes made outside Terraform are not detected and it is not imported.",
			},
		},
	}
}

// IdentitySchema defines the resource identity used by import and list.
func (r *CommunicationUserResource) IdentitySchema(ctx context.Context, req resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = util.IdentitySchema("Communication user ID, assigned by SAP")
}

func (r *CommunicationUserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func entityPath(id string) string {
	return fmt.Sprintf("CommunicationUsers/%s", id)
}

func (r *CommunicationUserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CommunicationUserResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only configured values are sent; SAP fills in the rest
	created, err := util.CreateEntity(ctx, r.client, service, "CommunicationUsers", util.CreateBody(&data))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create communication user, got error: %s", err))
		return
	}

	// Collect the values SAP assigned, re-reading the entity to pick up any
	// fields not included in the create response
	var remote CommunicationUserResourceModel
	util.ApplyResponse(&remote, created)
	if remote.ID.ValueString() == "" {
		resp.Diagnostics.AddError("API Error", "SAP did not return an ID for the created communication user")
		return
	}
	if entity, found, err := util.GetEntity(ctx, r.client, service, entityPath(remote.ID.ValueString())); err == nil && found {
		util.ApplyResponse(&remote, entity)
	}
	util.ResolveUnknowns(&data, &remote)

	tflog.Trace(ctx, "created a communication user resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	resp.Diagnostics.Append(util.SetIdentity(ctx, resp.Identity, data.ID)...)
}

func (r *CommunicationUserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CommunicationUserResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	entity, found, err := util.GetEntity(ctx, r.client, service, entityPath(data.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read communication user, got error: %s", err))
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

func (r *CommunicationUserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state CommunicationUserResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// PATCH only the attributes that changed; the password is only sent when
	// it was changed in the configuration
	endpoint := entityPath(state.ID.ValueString())
	if err := util.PatchEntity(ctx, r.client, service, endpoint, util.UpdateBody(&plan, &state)); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update communication user, got error: %s", err))
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

func (r *CommunicationUserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CommunicationUserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := util.DeleteEntity(ctx, r.client, service, entityPath(data.ID.ValueString())); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete communication user, got error: %s", err))
	}
}

// ImportState imports a communication user by its ID.
func (r *CommunicationUserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root("id"), path.Root("id"), req, resp)
}
