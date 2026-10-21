package util

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	s4types "terraform-provider-s4-communication/internal/types"
)

// IdentityModel is the resource identity of every resource: the key SAP
// assigns, the same value as the resource's id attribute and import ID.
type IdentityModel struct {
	ID types.String `tfsdk:"id"`
}

// IdentitySchema returns the identity schema with the given description of
// the id attribute.
func IdentitySchema(description string) identityschema.Schema {
	return identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"id": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       description,
			},
		},
	}
}

// SetIdentity sets the resource identity, if the resource has one in this
// request (older Terraform versions do not support identities).
func SetIdentity(ctx context.Context, identity *tfsdk.ResourceIdentity, id types.String) diag.Diagnostics {
	if identity == nil {
		return nil
	}
	return identity.Set(ctx, IdentityModel{ID: id})
}

// Client returns the API client passed to resources by the provider.
func Client(providerData interface{}, diags *diag.Diagnostics) *s4types.Client {
	if providerData == nil {
		return nil
	}
	client, ok := providerData.(*s4types.Client)
	if !ok {
		diags.AddError(
			"Unexpected Configure Type",
			fmt.Sprintf("Expected *s4types.Client, got: %T. Please report this issue to the provider developers.", providerData),
		)
	}
	return client
}
