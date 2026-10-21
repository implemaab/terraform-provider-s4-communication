package sapmock

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Services and properties of communication arrangements. SAP creates the
// services and properties of the communication scenario together with the
// arrangement; the mock creates one of each for any scenario:
//
//	inbound service  <scenario>_IN   with property IN_PROP (no values)
//	outbound service <scenario>_OUT  with property OUT_PROP = ["default"]
//	arrangement property ARR_PROP = ["default"]

// node is a nested arrangement entity reachable by its UUID.
type node struct {
	kind   string                 // entity set name
	entity map[string]interface{} // the entity itself
	parent map[string]interface{} // the entity holding it in a list
	list   string                 // name of that list on the parent
}

// propertyValueKind maps a property entity set to its value entity set.
var propertyValueKind = map[string]string{
	"Properties":                "PropertyValues",
	"InboundServiceProperties":  "InboundServicePropertyValues",
	"OutboundServiceProperties": "OutboundServicePropertyValues",
}

// servicePropertyKind maps a service entity set to its property entity set.
var servicePropertyKind = map[string]string{
	"InboundServices":  "InboundServiceProperties",
	"OutboundServices": "OutboundServiceProperties",
}

var outboundServiceUpdatable = []string{"Status", "URLPath", "TCPPort", "SOAPWSRMVersion", "JobTimezoneCode",
	"JobStartTime", "JobPackageSize", "JobRecurrenceValue", "JobFrequency"}

func (s *Server) addChild(kind string, parent map[string]interface{}, list string, entity map[string]interface{}) map[string]interface{} {
	entity["UUID"] = s.nextKey("UUID")
	items, _ := parent[list].([]interface{})
	parent[list] = append(items, entity)
	s.nodes[entity["UUID"].(string)] = &node{kind: kind, entity: entity, parent: parent, list: list}
	return entity
}

func (s *Server) addProperty(kind string, parent map[string]interface{}, name string, values []string) map[string]interface{} {
	prop := s.addChild(kind, parent, "Properties", map[string]interface{}{"Name": name, "Values": []interface{}{}})
	for _, v := range values {
		s.addChild(propertyValueKind[kind], prop, "Values", map[string]interface{}{"Value": v})
	}
	return prop
}

func (s *Server) addOutboundService(arrangement map[string]interface{}, fields map[string]interface{}) map[string]interface{} {
	svc := map[string]interface{}{"Properties": []interface{}{}, "Status": "", "URLPath": "", "TCPPort": 0, "SOAPWSRMVersion": "",
		"JobTimezoneCode": "", "JobStartTime": "", "JobPackageSize": 0, "JobRecurrenceValue": 0, "JobFrequency": "", "Type": "SOAP"}
	for k, v := range fields {
		svc[k] = v
	}
	return s.addChild("OutboundServices", arrangement, "OutboundServices", svc)
}

// seedServices creates the services and properties of the scenario.
func (s *Server) seedServices(arrangement map[string]interface{}) {
	scenario, _ := arrangement["CommunicationScenarioID"].(string)
	arrangement["InboundServices"] = []interface{}{}
	arrangement["OutboundServices"] = []interface{}{}
	arrangement["Properties"] = []interface{}{}

	in := s.addChild("InboundServices", arrangement, "InboundServices", map[string]interface{}{"ID": scenario + "_IN", "Type": "SOAP", "Properties": []interface{}{}})
	s.addProperty("InboundServiceProperties", in, "IN_PROP", nil)

	out := s.addOutboundService(arrangement, map[string]interface{}{"ID": scenario + "_OUT", "Status": "ACTIVE", "URLPath": "/default"})
	s.addProperty("OutboundServiceProperties", out, "OUT_PROP", []string{"default"})

	s.addProperty("Properties", arrangement, "ARR_PROP", []string{"default"})
}

// deepInsertOutboundServices applies outbound services sent with the create
// request to the services created from the scenario, like SAP does. It
// reports whether the body was valid.
func (s *Server) deepInsertOutboundServices(w http.ResponseWriter, arrangement map[string]interface{}, list []interface{}) bool {
	for _, item := range list {
		fields, _ := item.(map[string]interface{})
		id, _ := fields["ID"].(string)
		if id == "" {
			odataError(w, http.StatusBadRequest, "ID is required")
			return false
		}
		var svc map[string]interface{}
		for _, existing := range arrangement["OutboundServices"].([]interface{}) {
			if e := existing.(map[string]interface{}); e["ID"] == id {
				svc = e
			}
		}
		if svc == nil {
			svc = s.addOutboundService(arrangement, map[string]interface{}{"ID": id})
		}
		for k, v := range fields {
			if k != "ID" && k != "Properties" {
				svc[k] = v
			}
		}
		props, _ := fields["Properties"].([]interface{})
		for _, p := range props {
			body, _ := p.(map[string]interface{})
			name, _ := body["Name"].(string)
			var values []string
			if vals, ok := body["Values"].([]interface{}); ok {
				for _, v := range vals {
					if m, ok := v.(map[string]interface{}); ok {
						value, _ := m["Value"].(string)
						values = append(values, value)
					}
				}
			}
			replaced := false
			for _, existing := range svc["Properties"].([]interface{}) {
				if e := existing.(map[string]interface{}); e["Name"] == name {
					e["Values"] = []interface{}{}
					for _, v := range values {
						s.addChild("OutboundServicePropertyValues", e, "Values", map[string]interface{}{"Value": v})
					}
					replaced = true
				}
			}
			if !replaced {
				s.addProperty("OutboundServiceProperties", svc, name, values)
			}
		}
	}
	return true
}

// handleArrangementChild serves the nested arrangement entities. It reports
// whether the request was handled.
func (s *Server) handleArrangementChild(w http.ResponseWriter, r *http.Request, parts []string) bool {
	switch {
	// POST CommunicationArrangements/{UUID}/{InboundServices|OutboundServices|Properties}
	case len(parts) == 3 && parts[0] == "CommunicationArrangements":
		arrangement, ok := s.data["CommunicationArrangements"][parts[1]]
		if !ok {
			odataError(w, http.StatusNotFound, "arrangement %s not found", parts[1])
			return true
		}
		if r.Method != http.MethodPost {
			odataError(w, http.StatusMethodNotAllowed, "method %s not allowed", r.Method)
			return true
		}
		body, ok := decodeBody(w, r)
		if !ok {
			return true
		}
		switch parts[2] {
		case "InboundServices":
			if !s.checkUnique(w, arrangement, "InboundServices", "ID", body["ID"]) {
				return true
			}
			svc := s.addChild("InboundServices", arrangement, "InboundServices", map[string]interface{}{"ID": body["ID"], "Type": "SOAP", "Properties": []interface{}{}})
			writeJSON(w, http.StatusCreated, renderNode(svc, nil))
		case "OutboundServices":
			if !s.checkUnique(w, arrangement, "OutboundServices", "ID", body["ID"]) {
				return true
			}
			writeJSON(w, http.StatusCreated, renderNode(s.addOutboundService(arrangement, body), nil))
		case "Properties":
			s.createProperty(w, "Properties", arrangement, body)
		default:
			odataError(w, http.StatusNotFound, "unknown navigation %s", parts[2])
		}
		return true

	// POST {InboundServices|OutboundServices}/{UUID}/Properties and
	// POST {...Properties}/{UUID}/Values
	case len(parts) == 3:
		n, ok := s.nodes[parts[1]]
		if !ok || n.kind != parts[0] {
			return false
		}
		if r.Method != http.MethodPost {
			odataError(w, http.StatusMethodNotAllowed, "method %s not allowed", r.Method)
			return true
		}
		body, ok := decodeBody(w, r)
		if !ok {
			return true
		}
		if propKind, isService := servicePropertyKind[n.kind]; isService && parts[2] == "Properties" {
			s.createProperty(w, propKind, n.entity, body)
		} else if valueKind, isProperty := propertyValueKind[n.kind]; isProperty && parts[2] == "Values" {
			value := s.addChild(valueKind, n.entity, "Values", map[string]interface{}{"Value": body["Value"]})
			writeJSON(w, http.StatusCreated, value)
		} else {
			odataError(w, http.StatusNotFound, "unknown navigation %s", parts[2])
		}
		return true

	// {kind}/{UUID}
	case len(parts) == 2:
		n, ok := s.nodes[parts[1]]
		if !ok || n.kind != parts[0] {
			return false
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, renderNode(n.entity, nil))
		case http.MethodPatch:
			if n.kind != "OutboundServices" {
				odataError(w, http.StatusMethodNotAllowed, "%s cannot be updated", n.kind)
				return true
			}
			body, ok := decodeBody(w, r)
			if !ok {
				return true
			}
			for k, v := range body {
				if !contains(outboundServiceUpdatable, k) {
					odataError(w, http.StatusBadRequest, "property %s cannot be updated", k)
					return true
				}
				n.entity[k] = v
			}
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			if !strings.HasSuffix(n.kind, "PropertyValues") {
				odataError(w, http.StatusMethodNotAllowed, "%s cannot be deleted", n.kind)
				return true
			}
			items, _ := n.parent[n.list].([]interface{})
			kept := []interface{}{}
			for _, item := range items {
				if item.(map[string]interface{})["UUID"] != parts[1] {
					kept = append(kept, item)
				}
			}
			n.parent[n.list] = kept
			delete(s.nodes, parts[1])
			w.WriteHeader(http.StatusNoContent)
		default:
			odataError(w, http.StatusMethodNotAllowed, "method %s not allowed", r.Method)
		}
		return true
	}
	return false
}

func (s *Server) createProperty(w http.ResponseWriter, kind string, parent map[string]interface{}, body map[string]interface{}) {
	if !s.checkUnique(w, parent, "Properties", "Name", body["Name"]) {
		return
	}
	var values []string
	if list, ok := body["Values"].([]interface{}); ok {
		for _, v := range list {
			if m, ok := v.(map[string]interface{}); ok {
				value, _ := m["Value"].(string)
				values = append(values, value)
			}
		}
	}
	name, _ := body["Name"].(string)
	prop := s.addProperty(kind, parent, name, values)
	writeJSON(w, http.StatusCreated, renderNode(prop, map[string]string{"Values": ""}))
}

func (s *Server) checkUnique(w http.ResponseWriter, parent map[string]interface{}, list, key string, value interface{}) bool {
	if v, ok := value.(string); !ok || v == "" {
		odataError(w, http.StatusBadRequest, "%s is required", key)
		return false
	}
	items, _ := parent[list].([]interface{})
	for _, item := range items {
		if item.(map[string]interface{})[key] == value {
			odataError(w, http.StatusBadRequest, "%s %v already exists", key, value)
			return false
		}
	}
	return true
}

func decodeBody(w http.ResponseWriter, r *http.Request) (map[string]interface{}, bool) {
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		odataError(w, http.StatusBadRequest, "invalid body: %s", err)
		return nil, false
	}
	return body, true
}

// parseExpand parses an OData $expand option into navigation property names
// and their nested $expand options, e.g.
// "A,B($expand=C($expand=D))" -> {"A": "", "B": "C($expand=D)"}.
func parseExpand(expand string) map[string]string {
	out := map[string]string{}
	depth, start := 0, 0
	add := func(item string) {
		item = strings.TrimSpace(item)
		if item == "" {
			return
		}
		name, inner, _ := strings.Cut(item, "(")
		inner = strings.TrimSuffix(inner, ")")
		out[name] = strings.TrimPrefix(inner, "$expand=")
	}
	for i, c := range expand {
		switch c {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				add(expand[start:i])
				start = i + 1
			}
		}
	}
	add(expand[start:])
	return out
}

// navigationProperties are dropped from responses unless expanded.
var navigationProperties = []string{"InboundUser", "OutboundUser", "InboundServices", "OutboundServices", "Properties", "Values"}

// renderNode copies an entity, keeping only the expanded navigation
// properties (recursively).
func renderNode(entity map[string]interface{}, expand map[string]string) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range entity {
		if !contains(navigationProperties, k) {
			out[k] = v
			continue
		}
		inner, expanded := expand[k]
		if !expanded {
			continue
		}
		switch nav := v.(type) {
		case map[string]interface{}:
			out[k] = renderNode(nav, parseExpand(inner))
		case []interface{}:
			items := []interface{}{}
			for _, item := range nav {
				items = append(items, renderNode(item.(map[string]interface{}), parseExpand(inner)))
			}
			out[k] = items
		default:
			out[k] = v
		}
	}
	return out
}
