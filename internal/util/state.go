// Package util contains helpers shared by the resources for talking to the
// SAP API and mapping its responses into Terraform state.
//
// Resource model fields are mapped to SAP properties with an `api` struct tag:
//
//	api:"Name"            read from responses, sent on create and update
//	api:"Name,noupdate"   not accepted by the update endpoint (requires replace)
//	api:"Name,readonly"   only read from responses, never sent
//	api:"Name,writeonly"  sent, but never returned by the API (e.g. passwords)
package util

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	s4types "terraform-provider-s4communication/internal/types"
)

type apiField struct {
	index                         int
	key                           string
	noUpdate, readOnly, writeOnly bool
}

func apiFields(t reflect.Type) []apiField {
	var fields []apiField
	for i := 0; i < t.NumField(); i++ {
		tag, ok := t.Field(i).Tag.Lookup("api")
		if !ok {
			continue
		}
		parts := strings.Split(tag, ",")
		f := apiField{index: i, key: parts[0]}
		for _, opt := range parts[1:] {
			switch opt {
			case "noupdate":
				f.noUpdate = true
			case "readonly":
				f.readOnly = true
			case "writeonly":
				f.writeOnly = true
			}
		}
		fields = append(fields, f)
	}
	return fields
}

// apiValue converts a known framework value into its JSON representation.
func apiValue(v attr.Value) (interface{}, bool) {
	if v.IsNull() || v.IsUnknown() {
		return nil, false
	}
	switch val := v.(type) {
	case types.String:
		return val.ValueString(), true
	case types.Bool:
		return val.ValueBool(), true
	case types.Int64:
		return val.ValueInt64(), true
	}
	return nil, false
}

// CreateBody builds a request body from every known, non-null mapped field
// of model (a pointer to a resource model struct).
func CreateBody(model interface{}) map[string]interface{} {
	v := reflect.ValueOf(model).Elem()
	body := map[string]interface{}{}
	for _, f := range apiFields(v.Type()) {
		if f.readOnly {
			continue
		}
		if val, ok := apiValue(v.Field(f.index).Interface().(attr.Value)); ok {
			body[f.key] = val
		}
	}
	return body
}

// UpdateBody builds a PATCH body containing the updatable mapped fields whose
// planned value differs from the prior state.
func UpdateBody(plan, state interface{}) map[string]interface{} {
	pv := reflect.ValueOf(plan).Elem()
	sv := reflect.ValueOf(state).Elem()
	body := map[string]interface{}{}
	for _, f := range apiFields(pv.Type()) {
		if f.readOnly || f.noUpdate {
			continue
		}
		planned := pv.Field(f.index).Interface().(attr.Value)
		if planned.Equal(sv.Field(f.index).Interface().(attr.Value)) {
			continue
		}
		if val, ok := apiValue(planned); ok {
			body[f.key] = val
		}
	}
	return body
}

// ApplyResponse copies the mapped properties present in an API response into
// model (a pointer to a resource model struct).
func ApplyResponse(model interface{}, data map[string]interface{}) {
	v := reflect.ValueOf(model).Elem()
	for _, f := range apiFields(v.Type()) {
		if f.writeOnly {
			continue
		}
		raw, present := data[f.key]
		if !present {
			continue
		}
		field := v.Field(f.index)
		switch field.Interface().(type) {
		case types.String:
			if s, ok := raw.(string); ok {
				field.Set(reflect.ValueOf(types.StringValue(s)))
			} else if raw == nil {
				field.Set(reflect.ValueOf(types.StringValue("")))
			}
		case types.Bool:
			if b, ok := raw.(bool); ok {
				field.Set(reflect.ValueOf(types.BoolValue(b)))
			}
		case types.Int64:
			if n, ok := raw.(float64); ok {
				field.Set(reflect.ValueOf(types.Int64Value(int64(n))))
			}
		}
	}
}

// IsSet reports whether a planned value is known and not null.
func IsSet(v attr.Value) bool {
	return !v.IsNull() && !v.IsUnknown()
}

// ResolveUnknowns replaces every unknown attribute in planned (a pointer to a
// resource model struct) with the corresponding value from remote, which must
// be the same struct type populated from the API. Attributes that are still
// unknown afterwards become null. Known planned values are left untouched, as
// Terraform requires the applied state to match them.
func ResolveUnknowns(planned, remote interface{}) {
	pv := reflect.ValueOf(planned).Elem()
	rv := reflect.ValueOf(remote).Elem()
	for i := 0; i < pv.NumField(); i++ {
		field := pv.Field(i)
		val, ok := field.Interface().(attr.Value)
		if !ok || !val.IsUnknown() {
			continue
		}
		remoteField := rv.Field(i)
		if remoteVal, ok := remoteField.Interface().(attr.Value); ok && !remoteVal.IsUnknown() {
			field.Set(remoteField)
		} else {
			// The zero value of the framework value types is null.
			field.Set(reflect.Zero(field.Type()))
		}
	}
}

// GetEntity fetches a single entity and decodes it. found is false on 404.
func GetEntity(ctx context.Context, client *s4types.Client, service s4types.Service, endpoint string) (data map[string]interface{}, found bool, err error) {
	httpResp, err := client.Get(ctx, service, endpoint)
	if err != nil {
		return nil, false, err
	}
	if httpResp.StatusCode == http.StatusNotFound {
		httpResp.Body.Close()
		return nil, false, nil
	}
	data, err = decode(httpResp)
	return data, true, err
}

// CreateEntity POSTs body to an entity set and returns the created entity.
func CreateEntity(ctx context.Context, client *s4types.Client, service s4types.Service, entitySet string, body map[string]interface{}) (map[string]interface{}, error) {
	httpResp, err := client.Post(ctx, service, entitySet, body)
	if err != nil {
		return nil, err
	}
	return decode(httpResp)
}

// PatchEntity sends a PATCH request. An empty body is not sent.
func PatchEntity(ctx context.Context, client *s4types.Client, service s4types.Service, endpoint string, body map[string]interface{}) error {
	if len(body) == 0 {
		return nil
	}
	httpResp, err := client.Patch(ctx, service, endpoint, body)
	if err != nil {
		return err
	}
	return checkResponse(httpResp)
}

// DeleteEntity sends a DELETE request. A missing entity is not an error.
func DeleteEntity(ctx context.Context, client *s4types.Client, service s4types.Service, endpoint string) error {
	httpResp, err := client.Delete(ctx, service, endpoint)
	if err != nil {
		return err
	}
	if httpResp.StatusCode == http.StatusNotFound {
		httpResp.Body.Close()
		return nil
	}
	return checkResponse(httpResp)
}

func checkResponse(httpResp *http.Response) error {
	body, err := s4types.ReadResponseBody(httpResp)
	if err != nil {
		return err
	}
	if s4types.IsErrorResponse(httpResp) {
		return fmt.Errorf("status: %d, response: %s", httpResp.StatusCode, string(body))
	}
	return nil
}

func decode(httpResp *http.Response) (map[string]interface{}, error) {
	body, err := s4types.ReadResponseBody(httpResp)
	if err != nil {
		return nil, err
	}
	if s4types.IsErrorResponse(httpResp) {
		return nil, fmt.Errorf("status: %d, response: %s", httpResp.StatusCode, string(body))
	}
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("unable to parse response: %w", err)
	}
	return data, nil
}

// ConfigValue is a configurable attribute of a model and its value.
type ConfigValue struct {
	Name  string // Terraform attribute name
	Value attr.Value
}

// ConfigValues returns the configurable mapped attributes of model (a
// pointer to a resource model struct) in field order. Read-only keys and
// write-only values such as passwords are left out.
func ConfigValues(model interface{}) []ConfigValue {
	v := reflect.ValueOf(model).Elem()
	var out []ConfigValue
	for _, f := range apiFields(v.Type()) {
		if f.readOnly || f.writeOnly {
			continue
		}
		name, _, _ := strings.Cut(v.Type().Field(f.index).Tag.Get("tfsdk"), ",")
		out = append(out, ConfigValue{Name: name, Value: v.Field(f.index).Interface().(attr.Value)})
	}
	return out
}
