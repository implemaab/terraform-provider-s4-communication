package communicationuser

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	s4types "terraform-provider-s4-communication/internal/types"
	"terraform-provider-s4-communication/internal/util"
)

var _ list.ListResourceWithConfigure = &CommunicationUserListResource{}

func NewCommunicationUserListResource() list.ListResource {
	return &CommunicationUserListResource{}
}

// CommunicationUserListResource lists communication users for terraform query.
type CommunicationUserListResource struct {
	client *s4types.Client
}

// communicationUserListFilter holds the optional filters of the list configuration.
type communicationUserListFilter struct {
	Name types.String `tfsdk:"name"`
}

func (r *CommunicationUserListResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_communication_user"
}

func (r *CommunicationUserListResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = util.Client(req.ProviderData, &resp.Diagnostics)
}

func (r *CommunicationUserListResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists communication users. Use with `terraform query`, optionally with `-generate-config-out` to write configuration with import blocks. Passwords are never returned by SAP.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Only list communication users with this name.",
			},
		},
	}
}

func (r *CommunicationUserListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var filter communicationUserListFilter
	if diags := req.Config.Get(ctx, &filter); diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	entities, err := util.ListAll(ctx, r.client, service, "CommunicationUsers")
	if err != nil {
		stream.Results = util.ListError("Unable to list communication users", err)
		return
	}

	var items []util.ListItem
	for _, entity := range entities {
		if !filter.Name.IsNull() && entity["Name"] != filter.Name.ValueString() {
			continue
		}
		items = append(items, util.ListItem{
			ID:          str(entity, "ID"),
			DisplayName: str(entity, "Name"),
			Resource: func() (interface{}, diag.Diagnostics) {
				var model CommunicationUserResourceModel
				util.ApplyResponse(&model, entity)
				return model, nil
			},
		})
	}
	stream.Results = util.ListResults(ctx, req, items)
}

func str(m map[string]interface{}, key string) string {
	s, _ := m[key].(string)
	return s
}
