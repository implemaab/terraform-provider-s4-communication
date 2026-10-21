package outbounduser

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

var _ list.ListResourceWithConfigure = &OutboundUserListResource{}

func NewOutboundUserListResource() list.ListResource {
	return &OutboundUserListResource{}
}

// OutboundUserListResource lists outbound users for terraform query.
type OutboundUserListResource struct {
	client *s4types.Client
}

// outboundUserListFilter holds the optional filters of the list configuration.
type outboundUserListFilter struct {
	CommunicationSystemUUID types.String `tfsdk:"communication_system_uuid"`
}

func (r *OutboundUserListResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_outbound_user"
}

func (r *OutboundUserListResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = util.Client(req.ProviderData, &resp.Diagnostics)
}

func (r *OutboundUserListResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the outbound users of communication systems. Use with `terraform query`, optionally with `-generate-config-out` to write configuration with import blocks. Passwords are never returned by SAP.",
		Attributes: map[string]schema.Attribute{
			"communication_system_uuid": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Only list the outbound users of the communication system with this UUID.",
			},
		},
	}
}

func (r *OutboundUserListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var filter outboundUserListFilter
	if diags := req.Config.Get(ctx, &filter); diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	entities, err := util.ListAll(ctx, r.client, service, "OutboundUsers")
	if err != nil {
		stream.Results = util.ListError("Unable to list outbound users", err)
		return
	}

	var items []util.ListItem
	for _, entity := range entities {
		if !filter.CommunicationSystemUUID.IsNull() && entity["CommunicationSystemUUID"] != filter.CommunicationSystemUUID.ValueString() {
			continue
		}
		items = append(items, util.ListItem{
			ID:          str(entity, "UUID"),
			DisplayName: str(entity, "Name"),
			Resource: func() (interface{}, diag.Diagnostics) {
				var model OutboundUserResourceModel
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
