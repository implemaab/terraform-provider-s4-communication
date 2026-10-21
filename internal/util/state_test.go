package util

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

type testModel struct {
	ID       types.String `api:"UUID,readonly"`
	Name     types.String `api:"Name"`
	Type     types.String `api:"Type,noupdate"`
	Port     types.Int64  `api:"Port"`
	IsLocked types.Bool   `api:"IsLocked"`
	Password types.String `api:"Password,writeonly"`
	Local    types.String
}

func TestCreateBody(t *testing.T) {
	m := testModel{
		ID:       types.StringUnknown(),
		Name:     types.StringValue("n"),
		Type:     types.StringUnknown(),
		Port:     types.Int64Value(443),
		IsLocked: types.BoolNull(),
		Password: types.StringValue("secret"),
		Local:    types.StringValue("not mapped"),
	}
	got := CreateBody(&m)
	want := map[string]interface{}{"Name": "n", "Port": int64(443), "Password": "secret"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestUpdateBody(t *testing.T) {
	state := testModel{
		ID:       types.StringValue("abc"),
		Name:     types.StringValue("old"),
		Type:     types.StringValue("A"),
		Port:     types.Int64Value(443),
		IsLocked: types.BoolValue(false),
		Password: types.StringValue("secret"),
	}
	plan := state
	plan.Name = types.StringValue("new")
	plan.Type = types.StringValue("B") // not updatable, never sent
	plan.IsLocked = types.BoolValue(true)

	got := UpdateBody(&plan, &state)
	want := map[string]interface{}{"Name": "new", "IsLocked": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestApplyResponse(t *testing.T) {
	m := testModel{
		Password: types.StringValue("kept"),
		Name:     types.StringValue("kept too"),
	}
	ApplyResponse(&m, map[string]interface{}{
		"UUID":     "abc",
		"Type":     nil,
		"Port":     float64(8443),
		"IsLocked": true,
		"Password": "",
	})

	if m.ID.ValueString() != "abc" || m.Port.ValueInt64() != 8443 || !m.IsLocked.ValueBool() {
		t.Errorf("unexpected values: %+v", m)
	}
	if m.Type.IsNull() || m.Type.ValueString() != "" {
		t.Errorf("null string from API should become empty string, got %s", m.Type)
	}
	if m.Password.ValueString() != "kept" {
		t.Errorf("writeonly field must not be overwritten, got %s", m.Password)
	}
	if m.Name.ValueString() != "kept too" {
		t.Errorf("fields missing from the response must be kept, got %s", m.Name)
	}
}

func TestResolveUnknowns(t *testing.T) {
	planned := testModel{
		ID:       types.StringUnknown(),
		Name:     types.StringValue("configured"),
		Type:     types.StringUnknown(),
		Port:     types.Int64Unknown(),
		IsLocked: types.BoolValue(true),
	}

	var remote testModel
	ApplyResponse(&remote, map[string]interface{}{
		"UUID":     "abc",
		"Name":     "normalized",
		"Type":     "SET_BY_SAP",
		"IsLocked": false,
	})

	ResolveUnknowns(&planned, &remote)

	if planned.ID.ValueString() != "abc" {
		t.Errorf("ID: got %s", planned.ID)
	}
	if planned.Type.ValueString() != "SET_BY_SAP" {
		t.Errorf("Type: got %s", planned.Type)
	}
	if planned.Name.ValueString() != "configured" {
		t.Errorf("known planned Name must be kept, got %s", planned.Name)
	}
	if !planned.IsLocked.ValueBool() {
		t.Errorf("known planned IsLocked must be kept")
	}
	if !planned.Port.IsNull() {
		t.Errorf("Port absent from API should be null, got %s", planned.Port)
	}
}
