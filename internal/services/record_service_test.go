package services

import (
	"dns-api-go/internal/common"
	"dns-api-go/internal/mocks"
	"dns-api-go/internal/types"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// recordedCall captures a single MakeRequest invocation for assertion.
type recordedCall struct {
	Method     string
	Route      string
	QueryParam string
	Body       string
}

// scriptedServer is a MockServer convenience: callers seed a script of
// route-prefix → response pairs, and the server records every call.
type scriptedServer struct {
	mock   *mocks.MockServer
	calls  *[]recordedCall
	script map[string]scriptedResponse
}

type scriptedResponse struct {
	body string
	err  error
}

func newScriptedServer(script map[string]scriptedResponse) *scriptedServer {
	calls := []recordedCall{}
	ss := &scriptedServer{
		calls:  &calls,
		script: script,
	}
	ss.mock = &mocks.MockServer{
		ConfigurationIDFunc: func() (int, bool) { return 100882, true },
		MakeRequestFunc: func(method, route, queryParam string, body io.Reader) ([]byte, error) {
			bodyStr := ""
			if body != nil {
				b, _ := io.ReadAll(body)
				bodyStr = string(b)
			}
			calls = append(calls, recordedCall{
				Method:     method,
				Route:      route,
				QueryParam: queryParam,
				Body:       bodyStr,
			})
			for key, resp := range script {
				if matchesScriptKey(key, method, route, queryParam) {
					if resp.err != nil {
						return nil, resp.err
					}
					return []byte(resp.body), nil
				}
			}
			return nil, errorf("scripted server: no match for %s %s?%s", method, route, queryParam)
		},
	}
	return ss
}

// matchesScriptKey supports two forms:
//   - "METHOD ROUTE"        — matches any querystring
//   - "METHOD ROUTE?Q"      — matches when QueryParam contains Q (substring)
func matchesScriptKey(key, method, route, queryParam string) bool {
	parts := strings.SplitN(key, " ", 2)
	if len(parts) != 2 || parts[0] != method {
		return false
	}
	q := parts[1]
	if i := strings.Index(q, "?"); i >= 0 {
		if q[:i] != route {
			return false
		}
		return strings.Contains(queryParam, q[i+1:])
	}
	return q == route
}

func errorf(format string, args ...interface{}) error {
	return &mockError{msg: format, args: args}
}

type mockError struct {
	msg  string
	args []interface{}
}

func (e *mockError) Error() string {
	return e.msg
}

func TestRecordService_GetEntity_Success(t *testing.T) {
	ss := newScriptedServer(map[string]scriptedResponse{
		"GET /api/v2/resourceRecords/100913": {body: `{
			"id": 100913,
			"type": "HostRecord",
			"name": "example",
			"absoluteName": "example.spinuptest.internal",
			"addresses": [{"id": 1, "type": "IP4Address", "address": "10.5.0.10"}]
		}`},
	})

	rs := NewRecordService(ss.mock)
	entity, err := rs.GetEntity(100913, false)
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if entity.ID != 100913 {
		t.Errorf("ID = %d, want 100913", entity.ID)
	}
	if entity.Properties["addresses"] != "10.5.0.10" {
		t.Errorf("addresses = %q, want 10.5.0.10", entity.Properties["addresses"])
	}
}

func TestRecordService_GetEntity_NotFound(t *testing.T) {
	ss := newScriptedServer(map[string]scriptedResponse{
		"GET /api/v2/resourceRecords/999999": {
			err: &common.BluecatAPIError{StatusCode: http.StatusNotFound, Body: `{"code":"ResourceNotFound"}`},
		},
	})
	rs := NewRecordService(ss.mock)

	_, err := rs.GetEntity(999999, false)
	var notFound *ErrEntityNotFound
	if !errors.As(err, &notFound) {
		t.Errorf("err = %v (type %T), want *ErrEntityNotFound", err, err)
	}
}

func TestRecordService_GetEntity_WrongType(t *testing.T) {
	ss := newScriptedServer(map[string]scriptedResponse{
		"GET /api/v2/resourceRecords/77": {body: `{
			"id": 77, "type": "TXTRecord", "name": "x", "absoluteName": "x.example.com"
		}`},
	})
	rs := NewRecordService(ss.mock)

	_, err := rs.GetEntity(77, false)
	var mismatch *ErrEntityTypeMismatch
	if !errors.As(err, &mismatch) {
		t.Errorf("err = %v (type %T), want *ErrEntityTypeMismatch", err, err)
	}
}

func TestRecordService_GetRecordsByType_BuildsFilterAndPagination(t *testing.T) {
	ss := newScriptedServer(map[string]scriptedResponse{
		"GET /api/v2/resourceRecords": {body: `{"count":1,"data":[{
			"id":100913, "type":"HostRecord", "name":"example",
			"absoluteName":"example.spinuptest.internal",
			"addresses":[{"address":"10.5.0.10"}]
		}]}`},
	})
	rs := NewRecordService(ss.mock)

	entities, err := rs.GetRecordsByType(types.HOSTRECORD, map[string]interface{}{
		"count":   25,
		"start":   5,
		"options": map[string]string{"hint": "example"},
	}, 100902)
	if err != nil {
		t.Fatalf("GetRecordsByType: %v", err)
	}
	if len(*entities) != 1 {
		t.Fatalf("entities len = %d, want 1", len(*entities))
	}

	call := (*ss.calls)[0]
	if call.Method != "GET" || call.Route != "/api/v2/resourceRecords" {
		t.Errorf("call = %+v, want GET /api/v2/resourceRecords", call)
	}
	if !strings.Contains(call.QueryParam, "filter=") {
		t.Errorf("query missing filter=: %s", call.QueryParam)
	}
	if !strings.Contains(call.QueryParam, "limit=25") {
		t.Errorf("query missing limit=25: %s", call.QueryParam)
	}
	if !strings.Contains(call.QueryParam, "offset=5") {
		t.Errorf("query missing offset=5: %s", call.QueryParam)
	}
}

func TestRecordService_GetRecordsByType_InvalidType(t *testing.T) {
	rs := NewRecordService(newScriptedServer(nil).mock)
	if _, err := rs.GetRecordsByType("PTRRecord", map[string]interface{}{"count": 1, "start": 0}, 0); err == nil {
		t.Error("expected error for invalid record type, got nil")
	}
}

func TestRecordService_CreateRecord_AlreadyExists(t *testing.T) {
	ss := newScriptedServer(map[string]scriptedResponse{
		// findByAbsoluteName preflight returns a match → ErrEntityAlreadyExists.
		"GET /api/v2/resourceRecords": {body: `{"count":1,"data":[{
			"id":100913, "type":"HostRecord", "name":"example",
			"absoluteName":"example.spinuptest.internal"
		}]}`},
	})
	rs := NewRecordService(ss.mock)

	_, err := rs.CreateRecord(types.HOSTRECORD, map[string]interface{}{
		"absoluteName": "example.spinuptest.internal",
		"addresses":    []string{"10.5.0.10"},
		"ttl":          300,
	}, 100902)
	var exists *ErrEntityAlreadyExists
	if !errors.As(err, &exists) {
		t.Errorf("err = %v (type %T), want *ErrEntityAlreadyExists", err, err)
	}
}

// CreateRecord HostRecord drives the full v2 sequence:
//  1. pre-flight resourceRecords lookup (empty → proceed)
//  2. resolveZoneIDFromLabels: views/{viewId}/zones name:eq('internal')
//  3. resolveZoneIDFromLabels: zones/{id}/zones name:eq('spinuptest')
//  4. resolveAddressIDs: addresses?filter=address:eq('10.5.0.10')
//  5. POST /api/v2/zones/100913/resourceRecords with discriminated body
func TestRecordService_CreateRecord_Host(t *testing.T) {
	preflightCalls := 0
	ss := newScriptedServer(nil)
	ss.mock.MakeRequestFunc = func(method, route, queryParam string, body io.Reader) ([]byte, error) {
		switch {
		case method == "GET" && route == "/api/v2/resourceRecords":
			preflightCalls++
			return []byte(`{"count":0,"data":[]}`), nil
		case method == "GET" && route == "/api/v2/views/100902/zones":
			return []byte(`{"count":1,"data":[{"id":100911}]}`), nil
		case method == "GET" && route == "/api/v2/zones/100911/zones":
			return []byte(`{"count":1,"data":[{"id":100913}]}`), nil
		case method == "GET" && route == "/api/v2/addresses":
			return []byte(`{"count":1,"data":[{"id":100914,"type":"IP4Address","address":"10.5.0.10"}]}`), nil
		case method == "POST" && route == "/api/v2/zones/100913/resourceRecords":
			bodyBytes, _ := io.ReadAll(body)
			var got map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &got); err != nil {
				return nil, errorf("invalid POST body: %v", err)
			}
			if got["type"] != "HostRecord" {
				t.Errorf("POST body type = %v, want HostRecord", got["type"])
			}
			if got["name"] != "example" {
				t.Errorf("POST body name = %v, want example (local label, not FQDN)", got["name"])
			}
			addrs, ok := got["addresses"].([]interface{})
			if !ok || len(addrs) != 1 {
				t.Errorf("POST body addresses = %v, want one ref", got["addresses"])
			} else {
				ref := addrs[0].(map[string]interface{})
				if ref["id"].(float64) != 100914 {
					t.Errorf("address ref id = %v, want 100914", ref["id"])
				}
				if ref["type"] != "IP4Address" {
					t.Errorf("address ref type = %v, want IP4Address (echoed from lookup)", ref["type"])
				}
			}
			return []byte(`{
				"id": 100920, "type": "HostRecord", "name": "example",
				"absoluteName": "example.spinuptest.internal",
				"addresses": [{"address": "10.5.0.10"}]
			}`), nil
		}
		return nil, errorf("unexpected call %s %s?%s", method, route, queryParam)
	}

	rs := NewRecordService(ss.mock)
	entity, err := rs.CreateRecord(types.HOSTRECORD, map[string]interface{}{
		"absoluteName": "example.spinuptest.internal",
		"addresses":    []string{"10.5.0.10"},
		"ttl":          300,
	}, 100902)
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if entity.ID != 100920 {
		t.Errorf("created entity ID = %d, want 100920", entity.ID)
	}
	if entity.Properties["absoluteName"] != "example.spinuptest.internal" {
		t.Errorf("absoluteName = %q", entity.Properties["absoluteName"])
	}
	if preflightCalls != 1 {
		t.Errorf("preflight existence check called %d times, want 1", preflightCalls)
	}
}

func TestRecordService_CreateRecord_Alias_UsesAbsoluteNameLink(t *testing.T) {
	ss := newScriptedServer(nil)
	ss.mock.MakeRequestFunc = func(method, route, queryParam string, body io.Reader) ([]byte, error) {
		switch {
		case method == "GET" && route == "/api/v2/resourceRecords":
			return []byte(`{"count":0,"data":[]}`), nil
		case method == "GET" && route == "/api/v2/views/100902/zones":
			return []byte(`{"count":1,"data":[{"id":100911}]}`), nil
		case method == "GET" && route == "/api/v2/zones/100911/zones":
			return []byte(`{"count":1,"data":[{"id":100913}]}`), nil
		case method == "POST" && route == "/api/v2/zones/100913/resourceRecords":
			bodyBytes, _ := io.ReadAll(body)
			var got map[string]interface{}
			_ = json.Unmarshal(bodyBytes, &got)
			if got["type"] != "AliasRecord" {
				t.Errorf("type = %v, want AliasRecord", got["type"])
			}
			linked, ok := got["linkedRecord"].(map[string]interface{})
			if !ok || linked["absoluteName"] != "example.spinuptest.internal" {
				t.Errorf("linkedRecord = %v, want {absoluteName: example.spinuptest.internal}", got["linkedRecord"])
			}
			return []byte(`{
				"id": 100930, "type": "AliasRecord", "name": "alias",
				"absoluteName": "alias.spinuptest.internal"
			}`), nil
		}
		return nil, errorf("unexpected %s %s", method, route)
	}

	rs := NewRecordService(ss.mock)
	_, err := rs.CreateRecord(types.CNAMERECORD, map[string]interface{}{
		"absoluteName":     "alias.spinuptest.internal",
		"linkedRecordName": "example.spinuptest.internal",
	}, 100902)
	if err != nil {
		t.Fatalf("CreateRecord alias: %v", err)
	}
}

// ExternalHostRecord creation uses the ExternalHostsZone under the view —
// no FQDN walk, no address resolution.
func TestRecordService_CreateRecord_External_UsesExternalHostsZone(t *testing.T) {
	ss := newScriptedServer(nil)
	ss.mock.MakeRequestFunc = func(method, route, queryParam string, body io.Reader) ([]byte, error) {
		switch {
		case method == "GET" && route == "/api/v2/resourceRecords":
			return []byte(`{"count":0,"data":[]}`), nil
		case method == "GET" && route == "/api/v2/views/100902/zones":
			if !strings.Contains(queryParam, "ExternalHostsZone") {
				t.Errorf("zone lookup query missing ExternalHostsZone filter: %s", queryParam)
			}
			return []byte(`{"count":1,"data":[{"id":200001}]}`), nil
		case method == "POST" && route == "/api/v2/zones/200001/resourceRecords":
			bodyBytes, _ := io.ReadAll(body)
			var got map[string]interface{}
			_ = json.Unmarshal(bodyBytes, &got)
			if got["type"] != "ExternalHostRecord" || got["name"] != "host.external.com" {
				t.Errorf("body = %v, want ExternalHostRecord + full FQDN as name", got)
			}
			return []byte(`{"id": 100940, "type": "ExternalHostRecord", "name": "host.external.com"}`), nil
		}
		return nil, errorf("unexpected %s %s", method, route)
	}

	rs := NewRecordService(ss.mock)
	_, err := rs.CreateRecord(types.EXTERNALHOST, map[string]interface{}{
		"name": "host.external.com",
	}, 100902)
	if err != nil {
		t.Fatalf("CreateRecord external: %v", err)
	}
}

func TestRecordService_DeleteEntity_ChecksTypeBeforeDelete(t *testing.T) {
	ss := newScriptedServer(map[string]scriptedResponse{
		"GET /api/v2/resourceRecords/100913": {body: `{
			"id":100913, "type":"HostRecord", "name":"example", "absoluteName":"example.spinuptest.internal"
		}`},
		"DELETE /api/v2/resourceRecords/100913": {body: ""},
	})
	rs := NewRecordService(ss.mock)

	if err := rs.DeleteEntity(100913); err != nil {
		t.Fatalf("DeleteEntity: %v", err)
	}
	if len(*ss.calls) != 2 {
		t.Fatalf("expected 2 calls (GET then DELETE), got %d", len(*ss.calls))
	}
	if (*ss.calls)[0].Method != "GET" || (*ss.calls)[1].Method != "DELETE" {
		t.Errorf("call order = %v, want GET then DELETE", *ss.calls)
	}
}

func TestRecordService_DeleteEntity_RejectsNonRecordType(t *testing.T) {
	ss := newScriptedServer(map[string]scriptedResponse{
		"GET /api/v2/resourceRecords/100913": {body: `{
			"id":100913, "type":"IP4Address", "name":"10.5.0.10"
		}`},
	})
	rs := NewRecordService(ss.mock)

	err := rs.DeleteEntity(100913)
	var mismatch *ErrEntityTypeMismatch
	if !errors.As(err, &mismatch) {
		t.Errorf("err = %v (type %T), want *ErrEntityTypeMismatch (delete should not bypass type allowlist)", err, err)
	}
}

// HostCreate threads server-api's `properties` map through as
// userDefinedFields on BOTH the Address allocation and the HostRecord
// body. Yale's prod BAM rejects v2 POSTs that omit configured required
// UDFs (e.g. `phone`); core fields like reverseRecord still come out of
// the same map as top-level body keys, not UDFs.
func TestRecordService_CreateRecord_Host_PropagatesUserDefinedFields(t *testing.T) {
	addrBodySeen := false
	recordBodySeen := false
	ss := newScriptedServer(nil)
	ss.mock.MakeRequestFunc = func(method, route, queryParam string, body io.Reader) ([]byte, error) {
		switch {
		case method == "GET" && route == "/api/v2/resourceRecords":
			return []byte(`{"count":0,"data":[]}`), nil
		case method == "GET" && route == "/api/v2/views/100902/zones":
			return []byte(`{"count":1,"data":[{"id":100911}]}`), nil
		case method == "GET" && route == "/api/v2/zones/100911/zones":
			return []byte(`{"count":1,"data":[{"id":100913}]}`), nil
		case method == "GET" && route == "/api/v2/addresses":
			return []byte(`{"count":0,"data":[]}`), nil
		case method == "GET" && route == "/api/v2/networks":
			return []byte(`{"count":1,"data":[{"id":200001}]}`), nil
		case method == "POST" && route == "/api/v2/networks/200001/addresses":
			addrBodySeen = true
			b, _ := io.ReadAll(body)
			var got map[string]interface{}
			_ = json.Unmarshal(b, &got)
			udfs, ok := got["userDefinedFields"].(map[string]interface{})
			if !ok {
				t.Errorf("address body missing userDefinedFields: %s", string(b))
			} else if udfs["phone"] != "555-1212" {
				t.Errorf("address userDefinedFields.phone = %v, want 555-1212", udfs["phone"])
			}
			// reverseRecord must NOT show up as a UDF on the address — it's
			// a HostRecord core field; an address with a stray
			// reverseRecord UDF could trip Yale's BAM schema validator.
			if _, leaked := udfs["reverseRecord"]; leaked {
				t.Errorf("reverseRecord leaked into address userDefinedFields: %v", udfs)
			}
			return []byte(`{"id":3000123,"type":"IPv4Address","address":"10.5.99.99","state":"STATIC"}`), nil
		case method == "POST" && route == "/api/v2/zones/100913/resourceRecords":
			recordBodySeen = true
			b, _ := io.ReadAll(body)
			var got map[string]interface{}
			_ = json.Unmarshal(b, &got)
			if got["reverseRecord"] != true {
				t.Errorf("host record body reverseRecord = %v, want true (top-level, not in UDFs)", got["reverseRecord"])
			}
			udfs, ok := got["userDefinedFields"].(map[string]interface{})
			if !ok {
				t.Errorf("host record body missing userDefinedFields: %s", string(b))
			} else if udfs["phone"] != "555-1212" {
				t.Errorf("host record userDefinedFields.phone = %v, want 555-1212", udfs["phone"])
			}
			return []byte(`{"id":3000200,"type":"HostRecord","name":"example","absoluteName":"example.spinuptest.internal"}`), nil
		}
		return nil, errorf("unexpected %s %s", method, route)
	}

	rs := NewRecordService(ss.mock)
	_, err := rs.CreateRecord(types.HOSTRECORD, map[string]interface{}{
		"absoluteName": "example.spinuptest.internal",
		"addresses":    []string{"10.5.99.99"},
		"properties": map[string]string{
			"phone":         "555-1212",
			"reverseRecord": "true",
		},
	}, 100902)
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if !addrBodySeen {
		t.Error("address POST never happened")
	}
	if !recordBodySeen {
		t.Error("host record POST never happened")
	}
}

// HostCreate auto-allocates a v2 Address when BAM doesn't yet know the IP
// (preserves the v1 addHostRecord behavior server-api depends on). The
// flow: filter by address:eq → empty, range:contains → find network,
// POST /networks/{netId}/addresses → use returned ID in HostRecord body.
func TestRecordService_CreateRecord_Host_AutoAllocatesMissingAddress(t *testing.T) {
	allocatePosted := false
	ss := newScriptedServer(nil)
	ss.mock.MakeRequestFunc = func(method, route, queryParam string, body io.Reader) ([]byte, error) {
		switch {
		case method == "GET" && route == "/api/v2/resourceRecords":
			return []byte(`{"count":0,"data":[]}`), nil
		case method == "GET" && route == "/api/v2/views/100902/zones":
			return []byte(`{"count":1,"data":[{"id":100911}]}`), nil
		case method == "GET" && route == "/api/v2/zones/100911/zones":
			return []byte(`{"count":1,"data":[{"id":100913}]}`), nil
		case method == "GET" && route == "/api/v2/addresses":
			return []byte(`{"count":0,"data":[]}`), nil
		case method == "GET" && route == "/api/v2/networks":
			if !strings.Contains(queryParam, "range%3Acontains") {
				t.Errorf("network lookup query missing range:contains predicate: %s", queryParam)
			}
			return []byte(`{"count":1,"data":[{"id":200001}]}`), nil
		case method == "POST" && route == "/api/v2/networks/200001/addresses":
			allocatePosted = true
			b, _ := io.ReadAll(body)
			var got map[string]interface{}
			_ = json.Unmarshal(b, &got)
			if got["address"] != "10.5.99.99" {
				t.Errorf("allocate body address = %v, want 10.5.99.99 (explicit, not next-available)", got["address"])
			}
			if got["state"] != "STATIC" {
				t.Errorf("allocate body state = %v, want STATIC", got["state"])
			}
			return []byte(`{"id":3000123,"type":"IPv4Address","address":"10.5.99.99","state":"STATIC"}`), nil
		case method == "POST" && route == "/api/v2/zones/100913/resourceRecords":
			b, _ := io.ReadAll(body)
			var got map[string]interface{}
			_ = json.Unmarshal(b, &got)
			addrs := got["addresses"].([]interface{})
			ref := addrs[0].(map[string]interface{})
			if int(ref["id"].(float64)) != 3000123 {
				t.Errorf("host record addresses[0].id = %v, want 3000123 (the newly-allocated address)", ref["id"])
			}
			return []byte(`{"id":3000200,"type":"HostRecord","name":"example","absoluteName":"example.spinuptest.internal"}`), nil
		}
		return nil, errorf("unexpected %s %s", method, route)
	}

	rs := NewRecordService(ss.mock)
	entity, err := rs.CreateRecord(types.HOSTRECORD, map[string]interface{}{
		"absoluteName": "example.spinuptest.internal",
		"addresses":    []string{"10.5.99.99"},
	}, 100902)
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if entity.ID != 3000200 {
		t.Errorf("entity ID = %d, want 3000200", entity.ID)
	}
	if !allocatePosted {
		t.Error("expected POST /networks/{id}/addresses to allocate the missing IP, but it was never called")
	}
}

// When the IP isn't in BAM AND no network contains it, the operation
// fails — but any previously-allocated Address in the same flow must be
// rolled back so retries see a clean slate.
func TestRecordService_CreateRecord_Host_NoContainingNetworkRollsBack(t *testing.T) {
	deletedFirstAlloc := false
	ss := newScriptedServer(nil)
	ss.mock.MakeRequestFunc = func(method, route, queryParam string, body io.Reader) ([]byte, error) {
		switch {
		case method == "GET" && route == "/api/v2/resourceRecords":
			return []byte(`{"count":0,"data":[]}`), nil
		case method == "GET" && route == "/api/v2/views/100902/zones":
			return []byte(`{"count":1,"data":[{"id":100911}]}`), nil
		case method == "GET" && route == "/api/v2/zones/100911/zones":
			return []byte(`{"count":1,"data":[{"id":100913}]}`), nil
		case method == "GET" && route == "/api/v2/addresses":
			return []byte(`{"count":0,"data":[]}`), nil
		case method == "GET" && route == "/api/v2/networks":
			// First IP finds a network; second IP doesn't.
			if strings.Contains(queryParam, "10.5.0.10") {
				return []byte(`{"count":1,"data":[{"id":200001}]}`), nil
			}
			return []byte(`{"count":0,"data":[]}`), nil
		case method == "POST" && route == "/api/v2/networks/200001/addresses":
			return []byte(`{"id":3000123,"type":"IPv4Address","address":"10.5.0.10","state":"STATIC"}`), nil
		case method == "DELETE" && route == "/api/v2/addresses/3000123":
			deletedFirstAlloc = true
			return []byte{}, nil
		}
		return nil, errorf("unexpected %s %s", method, route)
	}

	rs := NewRecordService(ss.mock)
	_, err := rs.CreateRecord(types.HOSTRECORD, map[string]interface{}{
		"absoluteName": "example.spinuptest.internal",
		"addresses":    []string{"10.5.0.10", "10.99.99.99"},
	}, 100902)
	if err == nil {
		t.Fatal("expected error when second IP has no containing network, got nil")
	}
	if !deletedFirstAlloc {
		t.Error("first IP's freshly-allocated Address was not rolled back after the second IP failed")
	}
}

// If the HostRecord POST itself fails after Addresses are allocated,
// the rollback path must still trip so we don't leave orphaned Addresses.
func TestRecordService_CreateRecord_Host_RecordPostFailureRollsBack(t *testing.T) {
	deleted := false
	ss := newScriptedServer(nil)
	ss.mock.MakeRequestFunc = func(method, route, queryParam string, body io.Reader) ([]byte, error) {
		switch {
		case method == "GET" && route == "/api/v2/resourceRecords":
			return []byte(`{"count":0,"data":[]}`), nil
		case method == "GET" && route == "/api/v2/views/100902/zones":
			return []byte(`{"count":1,"data":[{"id":100911}]}`), nil
		case method == "GET" && route == "/api/v2/zones/100911/zones":
			return []byte(`{"count":1,"data":[{"id":100913}]}`), nil
		case method == "GET" && route == "/api/v2/addresses":
			return []byte(`{"count":0,"data":[]}`), nil
		case method == "GET" && route == "/api/v2/networks":
			return []byte(`{"count":1,"data":[{"id":200001}]}`), nil
		case method == "POST" && route == "/api/v2/networks/200001/addresses":
			return []byte(`{"id":3000123,"type":"IPv4Address","address":"10.5.0.10","state":"STATIC"}`), nil
		case method == "POST" && route == "/api/v2/zones/100913/resourceRecords":
			return nil, errorf("simulated BAM 500 on record create")
		case method == "DELETE" && route == "/api/v2/addresses/3000123":
			deleted = true
			return []byte{}, nil
		}
		return nil, errorf("unexpected %s %s", method, route)
	}

	rs := NewRecordService(ss.mock)
	_, err := rs.CreateRecord(types.HOSTRECORD, map[string]interface{}{
		"absoluteName": "example.spinuptest.internal",
		"addresses":    []string{"10.5.0.10"},
	}, 100902)
	if err == nil {
		t.Fatal("expected error from simulated record-create failure")
	}
	if !deleted {
		t.Error("newly-allocated Address was not rolled back after record-create failure")
	}
}
