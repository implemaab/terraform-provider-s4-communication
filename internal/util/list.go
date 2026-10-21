package util

import (
	"context"
	"iter"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/path"

	s4types "terraform-provider-s4-communication/internal/types"
)

// ListAll reads every page of an entity set.
func ListAll(ctx context.Context, client *s4types.Client, service s4types.Service, endpoint string) ([]map[string]interface{}, error) {
	var all []map[string]interface{}
	for endpoint != "" {
		page, _, err := GetEntity(ctx, client, service, endpoint)
		if err != nil {
			return nil, err
		}
		items, _ := page["value"].([]interface{})
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				all = append(all, m)
			}
		}
		next, _ := page["@odata.nextLink"].(string)
		endpoint = nextEndpoint(next, service)
	}
	return all, nil
}

// nextEndpoint turns an @odata.nextLink, which may be relative to the
// service root or an absolute URL, into an endpoint relative to the service.
func nextEndpoint(next string, service s4types.Service) string {
	if next == "" {
		return ""
	}
	if u, err := url.Parse(next); err == nil && u.IsAbs() {
		next = strings.TrimPrefix(u.RequestURI(), string(service))
	} else if strings.HasPrefix(next, string(service)) {
		next = strings.TrimPrefix(next, string(service))
	}
	return strings.TrimPrefix(next, "/")
}

// ListItem is one entity returned by a list resource.
type ListItem struct {
	ID          string // the resource identity, as used for import
	DisplayName string
	// Resource returns the resource model of the entity; only called when
	// Terraform asks for the full resource (e.g. -generate-config-out).
	Resource func() (interface{}, diag.Diagnostics)
}

// ListResults streams items as list results, honouring the request's limit.
func ListResults(ctx context.Context, req list.ListRequest, items []ListItem) iter.Seq[list.ListResult] {
	return func(push func(list.ListResult) bool) {
		for i, item := range items {
			if req.Limit > 0 && int64(i) >= req.Limit {
				return
			}
			result := req.NewListResult(ctx)
			result.DisplayName = item.DisplayName
			result.Diagnostics.Append(result.Identity.SetAttribute(ctx, path.Root("id"), item.ID)...)
			if req.IncludeResource && !result.Diagnostics.HasError() {
				model, diags := item.Resource()
				result.Diagnostics.Append(diags...)
				if !result.Diagnostics.HasError() {
					result.Diagnostics.Append(result.Resource.Set(ctx, model)...)
				}
			}
			if !push(result) {
				return
			}
		}
	}
}

// ListError streams a single result reporting err.
func ListError(summary string, err error) iter.Seq[list.ListResult] {
	var diags diag.Diagnostics
	diags.AddError(summary, err.Error())
	return list.ListResultsStreamDiagnostics(diags)
}
