package inbounduser

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	s4types "terraform-provider-s4communication/internal/types"
	"terraform-provider-s4communication/internal/util"
)

var _ list.ListResourceWithConfigure = &InboundUserListResource{}

func NewInboundUserListResource() list.ListResource {
	return &InboundUserListResource{}
}

// InboundUserListResource lists inbound users for terraform query.
type InboundUserListResource struct {
	client *s4types.Client
}

// inboundUserListFilter holds the optional filters of the list configuration.
type inboundUserListFilter struct {
	CommunicationSystemUUID types.String `tfsdk:"communication_system_uuid"`
}

func (r *InboundUserListResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_inbound_user"
}

func (r *InboundUserListResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = util.Client(req.ProviderData, &resp.Diagnostics)
}

func (r *InboundUserListResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the inbound users of communication systems. Use with `terraform query`, optionally with `-generate-config-out` to write configuration with import blocks.",
		Attributes: map[string]schema.Attribute{
			"communication_system_uuid": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Only list the inbound users of the communication system with this UUID.",
			},
		},
	}
}

func (r *InboundUserListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var filter inboundUserListFilter
	if diags := req.Config.Get(ctx, &filter); diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	entities, err := util.ListAll(ctx, r.client, service, "InboundUsers")
	if err != nil {
		stream.Results = util.ListError("Unable to list inbound users", err)
		return
	}

	var items []util.ListItem
	for _, entity := range entities {
		if !filter.CommunicationSystemUUID.IsNull() && entity["CommunicationSystemUUID"] != filter.CommunicationSystemUUID.ValueString() {
			continue
		}
		items = append(items, util.ListItem{
			ID:          str(entity, "UUID"),
			DisplayName: str(entity, "CommunicationUserID"),
			Resource: func() (interface{}, diag.Diagnostics) {
				var model InboundUserResourceModel
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
