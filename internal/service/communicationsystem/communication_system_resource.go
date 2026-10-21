package communicationsystem

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	s4types "terraform-provider-s4-communication/internal/types"
	"terraform-provider-s4-communication/internal/util"
)

const service = s4types.ServiceCommunicationSystem

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &CommunicationSystemResource{}
var _ resource.ResourceWithImportState = &CommunicationSystemResource{}
var _ resource.ResourceWithIdentity = &CommunicationSystemResource{}

func NewCommunicationSystemResource() resource.Resource {
	return &CommunicationSystemResource{}
}

// CommunicationSystemResource defines the resource implementation.
type CommunicationSystemResource struct {
	client *s4types.Client
}

// CommunicationSystemResourceModel describes the resource data model. The api
// tags map each attribute to its SAP property (see package util).
type CommunicationSystemResourceModel struct {
	ID                                                    types.String `tfsdk:"id" api:"UUID,readonly"`
	CommunicationSystemID                                 types.String `tfsdk:"communication_system_id" api:"ID"`
	Name                                                  types.String `tfsdk:"name" api:"Name"`
	Type                                                  types.String `tfsdk:"type" api:"Type"`
	Description                                           types.String `tfsdk:"description" api:"Description"`
	TCPPort                                               types.Int64  `tfsdk:"tcp_port" api:"TCPPort"`
	LogicalSystemID                                       types.String `tfsdk:"logical_system_id" api:"LogicalSystemID"`
	BusinessSystemID                                      types.String `tfsdk:"business_system_id" api:"BusinessSystemID"`
	SAPClient                                             types.String `tfsdk:"sap_client" api:"SAPClient"`
	HostName                                              types.String `tfsdk:"host_name" api:"HostName"`
	HostNameUI                                            types.String `tfsdk:"host_name_ui" api:"HostNameUI"`
	OwnerContactPersonName                                types.String `tfsdk:"owner_contact_person_name" api:"OwnerContactPersonName"`
	OwnerContactPersonPhone                               types.String `tfsdk:"owner_contact_person_phone" api:"OwnerContactPersonPhone"`
	OwnerContactPersonEMail                               types.String `tfsdk:"owner_contact_person_email" api:"OwnerContactPersonEMail"`
	IsOwnSystem                                           types.Bool   `tfsdk:"is_own_system" api:"IsOwnSystem"`
	CipherSuites                                          types.String `tfsdk:"cipher_suites" api:"CipherSuites"`
	IsOAuth2IdentityProviderActive                        types.Bool   `tfsdk:"is_oauth2_identity_provider_active" api:"IsOAuth2IdentityProviderActive"`
	OAuth2IdentityProviderName                            types.String `tfsdk:"oauth2_identity_provider_name" api:"OAuth2IdentityProviderName"`
	OAuth2IdentityProviderUserLogonType                   types.String `tfsdk:"oauth2_identity_provider_user_logon_type" api:"OAuth2IdentityProviderUserLogonType"`
	OAuth2IdentityProviderCertificatePublicKeyBase64      types.String `tfsdk:"oauth2_identity_provider_certificate_public_key_base64" api:"OAuth2IdentityProviderCertificatePublicKeyBase64"`
	OAuth2AuthorizationEndpoint                           types.String `tfsdk:"oauth2_authorization_endpoint" api:"OAuth2AuthorizationEndpoint"`
	OAuth2TokenEndpoint                                   types.String `tfsdk:"oauth2_token_endpoint" api:"OAuth2TokenEndpoint"`
	OAuth2Audience                                        types.String `tfsdk:"oauth2_audience" api:"OAuth2Audience"`
	OAuth2RedirectURI                                     types.String `tfsdk:"oauth2_redirect_uri" api:"OAuth2RedirectURI"`
	IsSAPCloudConnectorActive                             types.Bool   `tfsdk:"is_sap_cloud_connector_active" api:"IsSAPCloudConnectorActive"`
	SAPCloudConnectorLocationID                           types.String `tfsdk:"sap_cloud_connector_location_id" api:"SAPCloudConnectorLocationID"`
	IsRFCLoadBalancingActive                              types.Bool   `tfsdk:"is_rfc_load_balancing_active" api:"IsRFCLoadBalancingActive"`
	RFCSAPSystemID                                        types.String `tfsdk:"rfc_sap_system_id" api:"RFCSAPSystemID"`
	RFCSAPSystemNumber                                    types.String `tfsdk:"rfc_sap_system_number" api:"RFCSAPSystemNumber"`
	RFCLogonGroup                                         types.String `tfsdk:"rfc_logon_group" api:"RFCLogonGroup"`
	RFCMessageServerTargetHost                            types.String `tfsdk:"rfc_message_server_target_host" api:"RFCMessageServerTargetHost"`
	IsRFCFastSerializationActive                          types.Bool   `tfsdk:"is_rfc_fast_serialization_active" api:"IsRFCFastSerializationActive"`
	DestinationServiceUUID                                types.String `tfsdk:"destination_service_uuid" api:"DestinationServiceUUID"`
	DestinationServiceName                                types.String `tfsdk:"destination_service_name" api:"DestinationServiceName"`
	IsDefaultDestinationServiceActive                     types.Bool   `tfsdk:"is_default_destination_service_active" api:"IsDefaultDestinationServiceActive"`
	IsHubSystem                                           types.Bool   `tfsdk:"is_hub_system" api:"IsHubSystem"`
	IsInboundOnly                                         types.Bool   `tfsdk:"is_inbound_only" api:"IsInboundOnly"`
	IsSAMLBearerAssertionProviderActive                   types.Bool   `tfsdk:"is_saml_bearer_assertion_provider_active" api:"IsSAMLBearerAssertionProviderActive"`
	SAMLBearerAssertionProviderName                       types.String `tfsdk:"saml_bearer_assertion_provider_name" api:"SAMLBearerAssertionProviderName"`
	SAMLBearerAssertionProviderCertificatePublicKeyBase64 types.String `tfsdk:"saml_bearer_assertion_provider_certificate_public_key_base64" api:"SAMLBearerAssertionProviderCertificatePublicKeyBase64"`
	SAMLBearerAssertionProviderUserLogonType              types.String `tfsdk:"saml_bearer_assertion_provider_user_logon_type" api:"SAMLBearerAssertionProviderUserLogonType"`
}

func (r *CommunicationSystemResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_communication_system"
}

func (r *CommunicationSystemResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	// SAP rejects a system without a type with "User does not have the sufficient authorizations"
	systemType := util.OptionalString("Communication System Type, defaults to `customer_managed`")
	systemType.Default = stringdefault.StaticString("customer_managed")

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an SAP S/4HANA Cloud Communication System. Attributes that are not configured are set by SAP and kept as computed values.",
		Attributes: map[string]schema.Attribute{
			"id": util.IDAttribute("Communication System UUID (internal identifier)"),
			"communication_system_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Communication System ID (user-provided identifier)",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Communication System Name",
			},
			"type":                                     systemType,
			"description":                              util.OptionalString("Communication System Description"),
			"tcp_port":                                 util.OptionalInt64("TCP Port"),
			"logical_system_id":                        util.OptionalString("Logical System ID"),
			"business_system_id":                       util.OptionalString("Business System ID"),
			"sap_client":                               util.OptionalString("SAP Client"),
			"host_name":                                util.OptionalString("Host Name"),
			"host_name_ui":                             util.OptionalString("Host Name UI"),
			"owner_contact_person_name":                util.OptionalString("Owner Contact Person Name"),
			"owner_contact_person_phone":               util.OptionalString("Owner Contact Person Phone"),
			"owner_contact_person_email":               util.OptionalString("Owner Contact Person Email"),
			"is_own_system":                            util.OptionalBool("Is Own System"),
			"cipher_suites":                            util.OptionalString("Cipher Suites"),
			"is_oauth2_identity_provider_active":       util.OptionalBool("Is OAuth2 Identity Provider Active"),
			"oauth2_identity_provider_name":            util.OptionalString("OAuth2 Identity Provider Name"),
			"oauth2_identity_provider_user_logon_type": util.OptionalString("OAuth2 Identity Provider User Logon Type"),
			"oauth2_identity_provider_certificate_public_key_base64":       util.OptionalString("OAuth2 Identity Provider Certificate Public Key Base64"),
			"oauth2_authorization_endpoint":                                util.OptionalString("OAuth2 Authorization Endpoint"),
			"oauth2_token_endpoint":                                        util.OptionalString("OAuth2 Token Endpoint"),
			"oauth2_audience":                                              util.OptionalString("OAuth2 Audience"),
			"oauth2_redirect_uri":                                          util.OptionalString("OAuth2 Redirect URI"),
			"is_sap_cloud_connector_active":                                util.OptionalBool("Is SAP Cloud Connector Active"),
			"sap_cloud_connector_location_id":                              util.OptionalString("SAP Cloud Connector Location ID"),
			"is_rfc_load_balancing_active":                                 util.OptionalBool("Is RFC Load Balancing Active"),
			"rfc_sap_system_id":                                            util.OptionalString("RFC SAP System ID"),
			"rfc_sap_system_number":                                        util.OptionalString("RFC SAP System Number"),
			"rfc_logon_group":                                              util.OptionalString("RFC Logon Group"),
			"rfc_message_server_target_host":                               util.OptionalString("RFC Message Server Target Host"),
			"is_rfc_fast_serialization_active":                             util.OptionalBool("Is RFC Fast Serialization Active"),
			"destination_service_uuid":                                     util.OptionalString("Destination Service UUID"),
			"destination_service_name":                                     util.OptionalString("Destination Service Name"),
			"is_default_destination_service_active":                        util.OptionalBool("Is Default Destination Service Active"),
			"is_hub_system":                                                util.OptionalBool("Is Hub System"),
			"is_inbound_only":                                              util.OptionalBool("Is Inbound Only"),
			"is_saml_bearer_assertion_provider_active":                     util.OptionalBool("Is SAML Bearer Assertion Provider Active"),
			"saml_bearer_assertion_provider_name":                          util.OptionalString("SAML Bearer Assertion Provider Name"),
			"saml_bearer_assertion_provider_certificate_public_key_base64": util.OptionalString("SAML Bearer Assertion Provider Certificate Public Key Base64"),
			"saml_bearer_assertion_provider_user_logon_type":               util.OptionalString("SAML Bearer Assertion Provider User Logon Type"),
		},
	}
}

// IdentitySchema defines the resource identity used by import and list.
func (r *CommunicationSystemResource) IdentitySchema(ctx context.Context, req resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = util.IdentitySchema("Communication system UUID")
}

func (r *CommunicationSystemResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	return fmt.Sprintf("CommunicationSystems/%s", uuid)
}

func (r *CommunicationSystemResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CommunicationSystemResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only configured values are sent; SAP fills in the rest
	created, err := util.CreateEntity(ctx, r.client, service, "CommunicationSystems", util.CreateBody(&data))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create communication system, got error: %s", err))
		return
	}

	// Collect the values SAP assigned, re-reading the entity to pick up any
	// fields not included in the create response
	var remote CommunicationSystemResourceModel
	util.ApplyResponse(&remote, created)
	if remote.ID.ValueString() == "" {
		resp.Diagnostics.AddError("API Error", "SAP did not return a UUID for the created communication system")
		return
	}
	if entity, found, err := util.GetEntity(ctx, r.client, service, entityPath(remote.ID.ValueString())); err == nil && found {
		util.ApplyResponse(&remote, entity)
	}
	util.ResolveUnknowns(&data, &remote)

	tflog.Trace(ctx, "created a communication system resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	resp.Diagnostics.Append(util.SetIdentity(ctx, resp.Identity, data.ID)...)
}

func (r *CommunicationSystemResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CommunicationSystemResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	entity, found, err := util.GetEntity(ctx, r.client, service, entityPath(data.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read communication system, got error: %s", err))
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

func (r *CommunicationSystemResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state CommunicationSystemResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// PATCH only the attributes that changed
	endpoint := entityPath(state.ID.ValueString())
	if err := util.PatchEntity(ctx, r.client, service, endpoint, util.UpdateBody(&plan, &state)); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update communication system, got error: %s", err))
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

func (r *CommunicationSystemResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CommunicationSystemResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := util.DeleteEntity(ctx, r.client, service, entityPath(data.ID.ValueString())); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete communication system, got error: %s", err))
	}
}

// ImportState imports a communication system by its UUID.
func (r *CommunicationSystemResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root("id"), path.Root("id"), req, resp)
}
