package communicationsystem

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

var _ list.ListResourceWithConfigure = &CommunicationSystemListResource{}

func NewCommunicationSystemListResource() list.ListResource {
	return &CommunicationSystemListResource{}
}

// CommunicationSystemListResource lists communication systems for terraform query.
type CommunicationSystemListResource struct {
	client *s4types.Client
}

// communicationSystemListFilter holds the optional filters of the list configuration.
type communicationSystemListFilter struct {
	CommunicationSystemID types.String `tfsdk:"communication_system_id"`
}

func (r *CommunicationSystemListResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_communication_system"
}

func (r *CommunicationSystemListResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = util.Client(req.ProviderData, &resp.Diagnostics)
}

func (r *CommunicationSystemListResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists communication systems. Use with `terraform query`, optionally with `-generate-config-out` to write configuration with import blocks.",
		Attributes: map[string]schema.Attribute{
			"communication_system_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Only list the communication system with this ID.",
			},
		},
	}
}

func (r *CommunicationSystemListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var filter communicationSystemListFilter
	if diags := req.Config.Get(ctx, &filter); diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	entities, err := util.ListAll(ctx, r.client, service, "CommunicationSystems")
	if err != nil {
		stream.Results = util.ListError("Unable to list communication systems", err)
		return
	}

	var items []util.ListItem
	for _, entity := range entities {
		if !filter.CommunicationSystemID.IsNull() && entity["ID"] != filter.CommunicationSystemID.ValueString() {
			continue
		}
		items = append(items, util.ListItem{
			ID:          str(entity, "UUID"),
			DisplayName: str(entity, "ID") + " (" + str(entity, "Name") + ")",
			Resource: func() (interface{}, diag.Diagnostics) {
				var model CommunicationSystemResourceModel
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
