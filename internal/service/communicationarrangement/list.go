package communicationarrangement

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

var _ list.ListResourceWithConfigure = &CommunicationArrangementListResource{}

func NewCommunicationArrangementListResource() list.ListResource {
	return &CommunicationArrangementListResource{}
}

// CommunicationArrangementListResource lists communication arrangements for terraform query.
type CommunicationArrangementListResource struct {
	client *s4types.Client
}

// communicationArrangementListFilter holds the optional filters of the list configuration.
type communicationArrangementListFilter struct {
	CommunicationSystemID   types.String `tfsdk:"communication_system_id"`
	CommunicationScenarioID types.String `tfsdk:"communication_scenario_id"`
}

func (r *CommunicationArrangementListResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_communication_arrangement"
}

func (r *CommunicationArrangementListResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = util.Client(req.ProviderData, &resp.Diagnostics)
}

func (r *CommunicationArrangementListResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists communication arrangements. Use with `terraform query`, optionally with `-generate-config-out` to write configuration with import blocks; the generated configuration includes the services and properties that are imported.",
		Attributes: map[string]schema.Attribute{
			"communication_system_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Only list arrangements of the communication system with this ID.",
			},
			"communication_scenario_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Only list arrangements of this communication scenario, e.g. `SAP_COM_0008`.",
			},
		},
	}
}

func (r *CommunicationArrangementListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var filter communicationArrangementListFilter
	if diags := req.Config.Get(ctx, &filter); diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	entities, err := util.ListAll(ctx, r.client, service, "CommunicationArrangements?$expand="+expandAll)
	if err != nil {
		stream.Results = util.ListError("Unable to list communication arrangements", err)
		return
	}

	var items []util.ListItem
	for _, entity := range entities {
		if !filter.CommunicationSystemID.IsNull() && entity["CommunicationSystemID"] != filter.CommunicationSystemID.ValueString() {
			continue
		}
		if !filter.CommunicationScenarioID.IsNull() && entity["CommunicationScenarioID"] != filter.CommunicationScenarioID.ValueString() {
			continue
		}
		items = append(items, util.ListItem{
			ID:          stringField(entity, "UUID"),
			DisplayName: stringField(entity, "Name"),
			Resource: func() (interface{}, diag.Diagnostics) {
				var model CommunicationArrangementResourceModel
				applyResponse(&model, entity)
				diags := importServices(ctx, &model, entity)
				return model, diags
			},
		})
	}
	stream.Results = util.ListResults(ctx, req, items)
}
