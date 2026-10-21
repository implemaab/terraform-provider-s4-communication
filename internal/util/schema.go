package util

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// The attributes below are Optional+Computed without a static default: when
// omitted from the configuration, SAP decides the value and Terraform keeps
// whatever is in state instead of planning a change.

// OptionalString returns an optional string attribute that SAP may compute.
func OptionalString(description string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:            true,
		Computed:            true,
		PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		MarkdownDescription: description,
	}
}

// OptionalBool returns an optional bool attribute that SAP may compute.
func OptionalBool(description string) schema.BoolAttribute {
	return schema.BoolAttribute{
		Optional:            true,
		Computed:            true,
		PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
		MarkdownDescription: description,
	}
}

// OptionalInt64 returns an optional int64 attribute that SAP may compute.
func OptionalInt64(description string) schema.Int64Attribute {
	return schema.Int64Attribute{
		Optional:            true,
		Computed:            true,
		PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
		MarkdownDescription: description,
	}
}

// IDAttribute returns the computed identifier attribute.
func IDAttribute(description string) schema.StringAttribute {
	return schema.StringAttribute{
		Computed:            true,
		PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		MarkdownDescription: description,
	}
}

// RequiredForceNew returns a required string attribute that cannot be
// changed in place.
func RequiredForceNew(description string) schema.StringAttribute {
	return schema.StringAttribute{
		Required:            true,
		PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		MarkdownDescription: description,
	}
}

// The Nested variants are for attributes inside nested objects, such as the
// entries of a map. UseStateForUnknown keeps a null prior value since
// framework v1.15.1, so an attribute of a newly added entry would be planned
// as null; UseNonNullStateForUnknown leaves it unknown until SAP sets it.

// NestedOptionalString is OptionalString for attributes of nested objects.
func NestedOptionalString(description string) schema.StringAttribute {
	a := OptionalString(description)
	a.PlanModifiers = []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()}
	return a
}

// NestedOptionalInt64 is OptionalInt64 for attributes of nested objects.
func NestedOptionalInt64(description string) schema.Int64Attribute {
	a := OptionalInt64(description)
	a.PlanModifiers = []planmodifier.Int64{int64planmodifier.UseNonNullStateForUnknown()}
	return a
}
