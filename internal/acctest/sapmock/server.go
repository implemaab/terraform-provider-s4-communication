// Package sapmock implements an in-memory stand-in for the SAP S/4HANA Cloud
// communication management OData APIs, used by the provider's tests.
//
// It follows the published API definitions (sap-s4-CE_APS_COM_*_ODATA_0001):
// single entities only support GET, PATCH and DELETE; PATCH rejects
// properties outside the update schema; writes require a CSRF token; keys
// are assigned by the server; and some properties are filled in by the
// server when the client leaves them empty.
package sapmock

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	csService = "/sap/opu/odata4/sap/aps_com_cs_a4c_odata/srvd_a2x/sap/aps_com_cs_a4c_odata/0001"
	cuService = "/sap/opu/odata4/sap/aps_com_cu_a4c_odata/srvd_a2x/sap/aps_com_cu_a4c_odata/0001"
	caService = "/sap/opu/odata4/sap/aps_com_ca_a4c_odata/srvd_a2x/sap/aps_com_ca_a4c_odata/0001"

	Username  = "mockuser"
	Password  = "mockpass"
	csrfToken = "mock-csrf-token"
)

type entitySet struct {
	key        string                 // key property, assigned by the server
	properties map[string]interface{} // every property with its empty value
	updatable  []string               // properties accepted by PATCH
	writeOnly  []string               // properties never returned
	defaults   func(e map[string]interface{})
	validate   func(e map[string]interface{}) error // rejects a create like SAP would
}

func props(strs, bools, ints []string) map[string]interface{} {
	m := map[string]interface{}{}
	for _, p := range strs {
		m[p] = ""
	}
	for _, p := range bools {
		m[p] = false
	}
	for _, p := range ints {
		m[p] = 0
	}
	return m
}

var systemStrings = []string{"ID", "Name", "Type", "Description", "LogicalSystemID", "BusinessSystemID", "SAPClient", "HostName", "HostNameUI",
	"OwnerContactPersonName", "OwnerContactPersonPhone", "OwnerContactPersonEMail", "CipherSuites", "OAuth2IdentityProviderName",
	"OAuth2IdentityProviderUserLogonType", "OAuth2IdentityProviderCertificatePublicKeyBase64", "OAuth2AuthorizationEndpoint",
	"OAuth2TokenEndpoint", "OAuth2Audience", "OAuth2RedirectURI", "SAPCloudConnectorLocationID", "RFCSAPSystemID", "RFCSAPSystemNumber",
	"RFCLogonGroup", "RFCMessageServerTargetHost", "DestinationServiceUUID", "DestinationServiceName", "SAMLBearerAssertionProviderName",
	"SAMLBearerAssertionProviderCertificatePublicKeyBase64", "SAMLBearerAssertionProviderUserLogonType"}
var systemBools = []string{"IsOwnSystem", "IsOAuth2IdentityProviderActive", "IsSAPCloudConnectorActive", "IsRFCLoadBalancingActive",
	"IsRFCFastSerializationActive", "IsDefaultDestinationServiceActive", "IsHubSystem", "IsInboundOnly", "IsSAMLBearerAssertionProviderActive"}

func entitySets() map[string]map[string]*entitySet {
	return map[string]map[string]*entitySet{
		csService: {
			"CommunicationSystems": {
				key:        "UUID",
				properties: props(systemStrings, systemBools, []string{"TCPPort"}),
				updatable:  append(append(append([]string{}, systemStrings...), systemBools...), "TCPPort"),
				defaults: func(e map[string]interface{}) {
					setIfEmpty(e, "Type", "customer_managed")
					setIfEmpty(e, "TCPPort", 443)
					setIfEmpty(e, "LogicalSystemID", e["ID"])
					setIfEmpty(e, "CipherSuites", "DEFAULT")
				},
			},
			"InboundUsers": {
				key: "UUID",
				properties: props([]string{"CommunicationSystemUUID", "CommunicationUserID", "OAuth2ClientID", "OAuth2GrantType",
					"OAuth2RefreshTokenExpiryUnit", "AuthenticationMethod"}, []string{"IsOAuth2RefreshTokenAllowed"}, []string{"OAuth2RefreshTokenExpiryValue"}),
				updatable: []string{"CommunicationUserID", "OAuth2ClientID", "OAuth2GrantType", "IsOAuth2RefreshTokenAllowed",
					"OAuth2RefreshTokenExpiryValue", "OAuth2RefreshTokenExpiryUnit", "AuthenticationMethod"},
				defaults: func(e map[string]interface{}) { setIfEmpty(e, "AuthenticationMethod", "basic") },
			},
			"OutboundUsers": {
				key: "UUID",
				properties: props([]string{"CommunicationSystemUUID", "Name", "OAuth2ClientID", "OAuth2ClientAuthenticationMethod",
					"CertificateID", "AuthenticationMethod", "Password"}, nil, nil),
				updatable: []string{"Name", "OAuth2ClientID", "OAuth2ClientAuthenticationMethod", "CertificateID", "AuthenticationMethod", "Password"},
				writeOnly: []string{"Password"},
				defaults:  func(e map[string]interface{}) { setIfEmpty(e, "AuthenticationMethod", "basic") },
			},
		},
		cuService: {
			"CommunicationUsers": {
				key:        "ID",
				properties: props([]string{"Name", "Type", "Description", "Password"}, []string{"IsLocked"}, nil),
				updatable:  []string{"Name", "Description", "IsLocked", "Password"},
				writeOnly:  []string{"Password"},
				// SAP rejects a user without a type ("Invalid User category")
				validate: func(e map[string]interface{}) error {
					if e["Type"] == nil || e["Type"] == "" {
						return fmt.Errorf("Invalid User category for Communication User")
					}
					return nil
				},
			},
		},
		caService: {
			"CommunicationArrangements": {
				key:        "UUID",
				properties: props([]string{"CommunicationScenarioID", "CommunicationSystemID", "Name", "Description"}, nil, nil),
				updatable:  []string{"CommunicationScenarioID", "CommunicationSystemID", "Name", "Description"},
				defaults:   func(e map[string]interface{}) { setIfEmpty(e, "Description", e["CommunicationScenarioID"]) },
			},
		},
	}
}

func setIfEmpty(e map[string]interface{}, key string, value interface{}) {
	switch v := e[key].(type) {
	case string:
		if v != "" {
			return
		}
	case float64:
		if v != 0 {
			return
		}
	case int:
		if v != 0 {
			return
		}
	}
	e[key] = value
}

// arrangement navigation properties and their nested key property
var nestedUsers = map[string]string{
	"InboundUser":  "CommunicationSystemInboundUserUUID",
	"OutboundUser": "CommunicationSystemOutboundUserUUID",
}

// Server is a running mock SAP API.
type Server struct {
	*httptest.Server

	mu      sync.Mutex
	sets    map[string]map[string]*entitySet
	data    map[string]map[string]map[string]interface{} // set name -> key -> entity
	nodes   map[string]*node                             // nested arrangement entities by UUID
	counter int

	// PageSize limits the number of entities returned per page when listing
	// an entity set; further pages are linked with @odata.nextLink.
	PageSize int

	// certificate authority of a server started with NewTLS
	ca    *x509.Certificate
	caKey *ecdsa.PrivateKey
}

// New starts a mock SAP API server. Close it when done.
func New() *Server {
	s := newServer()
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	return s
}

func newServer() *Server {
	return &Server{sets: entitySets(), data: map[string]map[string]map[string]interface{}{}, nodes: map[string]*node{}}
}

// Entities returns a copy of all stored entities of an entity set.
func (s *Server) Entities(set string) []map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []map[string]interface{}
	for _, e := range s.data[set] {
		out = append(out, copyMap(e))
	}
	return out
}

func (s *Server) nextKey(prefix string) string {
	s.counter++
	if prefix == "ID" {
		return fmt.Sprintf("CC%010d", s.counter)
	}
	return fmt.Sprintf("00000000-0000-0000-0000-%012d", s.counter)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func odataError(w http.ResponseWriter, status int, format string, args ...interface{}) {
	writeJSON(w, status, map[string]interface{}{"error": map[string]interface{}{"message": fmt.Sprintf(format, args...)}})
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	if user, ok := clientCertificateUser(r); ok {
		if user != Username {
			odataError(w, http.StatusUnauthorized, "unknown communication user %s", user)
			return
		}
	} else if user, pass, ok := r.BasicAuth(); !ok || user != Username || pass != Password {
		odataError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var svc string
	for _, candidate := range []string{csService, cuService, caService} {
		if strings.HasPrefix(r.URL.Path, candidate) {
			svc = candidate
		}
	}
	if svc == "" {
		odataError(w, http.StatusNotFound, "unknown service")
		return
	}

	if r.Method == http.MethodHead {
		if r.Header.Get("X-CSRF-Token") == "Fetch" {
			w.Header().Set("X-CSRF-Token", csrfToken)
		}
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet && r.Header.Get("X-CSRF-Token") != csrfToken {
		w.Header().Set("X-CSRF-Token", "Required")
		odataError(w, http.StatusForbidden, "CSRF token validation failed")
		return
	}

	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, svc), "/"), "/")
	s.mu.Lock()
	defer s.mu.Unlock()

	// Nested arrangement users: PATCH {InboundUsers|OutboundUsers}/{UUID} on the arrangement service
	if svc == caService && len(parts) == 2 && (parts[0] == "InboundUsers" || parts[0] == "OutboundUsers") {
		s.handleNestedUser(w, r, strings.TrimSuffix(parts[0], "s"), parts[1])
		return
	}
	// Services, properties and property values of arrangements
	if svc == caService && s.handleArrangementChild(w, r, parts) {
		return
	}

	set, ok := s.sets[svc][parts[0]]
	if !ok || len(parts) > 2 {
		odataError(w, http.StatusNotFound, "unknown resource %s", r.URL.Path)
		return
	}
	name := parts[0]

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodPost:
			s.create(w, r, name, set)
		case http.MethodGet:
			s.list(w, r, name, set)
		default:
			odataError(w, http.StatusMethodNotAllowed, "method %s not allowed on collection", r.Method)
		}
		return
	}

	entity, ok := s.data[name][parts[1]]
	if !ok {
		odataError(w, http.StatusNotFound, "%s(%s) not found", name, parts[1])
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.render(name, set, entity, r.URL.Query().Get("$expand")))
	case http.MethodPatch:
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			odataError(w, http.StatusBadRequest, "invalid body: %s", err)
			return
		}
		for k, v := range body {
			if !contains(set.updatable, k) {
				odataError(w, http.StatusBadRequest, "property %s cannot be updated", k)
				return
			}
			entity[k] = v
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		delete(s.data[name], parts[1])
		w.WriteHeader(http.StatusNoContent)
	default:
		odataError(w, http.StatusMethodNotAllowed, "method %s not allowed on entity", r.Method)
	}
}

func (s *Server) create(w http.ResponseWriter, r *http.Request, name string, set *entitySet) {
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		odataError(w, http.StatusBadRequest, "invalid body: %s", err)
		return
	}

	entity := copyMap(set.properties)
	for k, v := range body {
		if _, nested := nestedUsers[k]; name == "CommunicationArrangements" && (nested || k == "OutboundServices") {
			continue
		}
		if _, known := set.properties[k]; !known {
			odataError(w, http.StatusBadRequest, "unknown property %s", k)
			return
		}
		if k == set.key {
			odataError(w, http.StatusBadRequest, "key %s is assigned by the server", k)
			return
		}
		entity[k] = v
	}
	if set.defaults != nil {
		set.defaults(entity)
	}
	if set.validate != nil {
		if err := set.validate(entity); err != nil {
			odataError(w, http.StatusBadRequest, "%s", err)
			return
		}
	}
	key := s.nextKey(set.key)
	entity[set.key] = key

	// Deep insert of the arrangement's users
	if name == "CommunicationArrangements" {
		for nav, nestedKey := range nestedUsers {
			nested, ok := body[nav].(map[string]interface{})
			if !ok {
				entity[nav] = nil
				continue
			}
			if _, ok := nested[nestedKey].(string); !ok {
				odataError(w, http.StatusBadRequest, "%s requires %s", nav, nestedKey)
				return
			}
			entity[nav] = map[string]interface{}{"UUID": s.nextKey("UUID"), "CommunicationArrangementUUID": key, nestedKey: nested[nestedKey]}
		}
		s.seedServices(entity)
		if list, ok := body["OutboundServices"].([]interface{}); ok {
			if !s.deepInsertOutboundServices(w, entity, list) {
				return
			}
		}
	}

	if s.data[name] == nil {
		s.data[name] = map[string]map[string]interface{}{}
	}
	s.data[name][key] = entity
	writeJSON(w, http.StatusCreated, s.render(name, set, entity, ""))
}

func (s *Server) handleNestedUser(w http.ResponseWriter, r *http.Request, nav, uuid string) {
	for _, arrangement := range s.data["CommunicationArrangements"] {
		nested, ok := arrangement[nav].(map[string]interface{})
		if !ok || nested["UUID"] != uuid {
			continue
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, nested)
		case http.MethodPatch:
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				odataError(w, http.StatusBadRequest, "invalid body: %s", err)
				return
			}
			for k, v := range body {
				if k != nestedUsers[nav] {
					odataError(w, http.StatusBadRequest, "property %s cannot be updated", k)
					return
				}
				nested[k] = v
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			odataError(w, http.StatusMethodNotAllowed, "method %s not allowed", r.Method)
		}
		return
	}
	odataError(w, http.StatusNotFound, "%s(%s) not found", nav, uuid)
}

// render returns the entity as the API would, hiding write-only properties
// and navigation properties that were not expanded.
func (s *Server) render(name string, set *entitySet, entity map[string]interface{}, expand string) map[string]interface{} {
	out := copyMap(entity)
	for _, p := range set.writeOnly {
		out[p] = ""
	}
	if name == "CommunicationArrangements" {
		return renderNode(out, parseExpand(expand))
	}
	return out
}

func copyMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		if nested, ok := v.(map[string]interface{}); ok {
			v = copyMap(nested)
		}
		out[k] = v
	}
	return out
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// Modify runs fn with exclusive access to the stored entities (entity set
// name -> key -> entity), to simulate changes made outside Terraform.
func (s *Server) Modify(fn func(data map[string]map[string]map[string]interface{})) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s.data)
}

// list returns a page of an entity set, ordered by key.
func (s *Server) list(w http.ResponseWriter, r *http.Request, name string, set *entitySet) {
	keys := make([]string, 0, len(s.data[name]))
	for k := range s.data[name] {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	query := r.URL.Query()
	start, _ := strconv.Atoi(query.Get("$skiptoken"))
	end := len(keys)
	if s.PageSize > 0 && start+s.PageSize < end {
		end = start + s.PageSize
	}

	values := []interface{}{}
	for _, k := range keys[start:end] {
		values = append(values, s.render(name, set, s.data[name][k], query.Get("$expand")))
	}
	page := map[string]interface{}{"value": values}
	if end < len(keys) {
		query.Set("$skiptoken", strconv.Itoa(end))
		page["@odata.nextLink"] = name + "?" + query.Encode()
	}
	writeJSON(w, http.StatusOK, page)
}
