package services

import (
	"dns-api-go/internal/common"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestIpAddressService_GetIpAddress_Success drives the happy path through
// the v2 addresses filter lookup.
func TestIpAddressService_GetIpAddress_Success(t *testing.T) {
	ss := newScriptedServer(map[string]scriptedResponse{
		"GET /api/v2/addresses": {body: `{"count":1,"data":[{
			"id": 100914, "type": "IPv4Address", "address": "10.5.0.10", "name": "host.example"
		}]}`},
	})
	ips := NewIpAddressService(ss.mock)

	entity, err := ips.GetIpAddress("10.5.0.10")
	if err != nil {
		t.Fatalf("GetIpAddress: %v", err)
	}
	if entity.ID != 100914 {
		t.Errorf("ID = %d, want 100914", entity.ID)
	}
	if entity.Properties["address"] != "10.5.0.10" {
		t.Errorf("Properties[address] = %q, want 10.5.0.10", entity.Properties["address"])
	}

	call := (*ss.calls)[0]
	if !strings.Contains(call.QueryParam, "address%3Aeq%28%2710.5.0.10%27%29") {
		t.Errorf("query missing url-encoded address:eq filter: %s", call.QueryParam)
	}
}

func TestIpAddressService_GetIpAddress_NotFound(t *testing.T) {
	ss := newScriptedServer(map[string]scriptedResponse{
		"GET /api/v2/addresses": {body: `{"count":0,"data":[]}`},
	})
	ips := NewIpAddressService(ss.mock)

	_, err := ips.GetIpAddress("10.5.99.99")
	var notFound *ErrEntityNotFound
	if !errors.As(err, &notFound) {
		t.Errorf("err = %v (type %T), want *ErrEntityNotFound", err, err)
	}
}

// TestIpAddressService_DeleteIpAddress_DeletesHostRecordsThenAddress walks
// the full v2 delete sequence: filter lookup, list owning records,
// DELETE each, DELETE the address itself.
func TestIpAddressService_DeleteIpAddress_DeletesHostRecordsThenAddress(t *testing.T) {
	ss := newScriptedServer(nil)
	ss.mock.MakeRequestFunc = func(method, route, queryParam string, body io.Reader) ([]byte, error) {
		switch {
		case method == "GET" && route == "/api/v2/addresses":
			return []byte(`{"count":1,"data":[{"id":100914,"type":"IPv4Address","address":"10.5.0.10"}]}`), nil
		case method == "GET" && route == "/api/v2/addresses/100914/resourceRecords":
			return []byte(`{"count":2,"data":[{"id":100920},{"id":100921}]}`), nil
		case method == "DELETE" && route == "/api/v2/resourceRecords/100920":
			return []byte{}, nil
		case method == "DELETE" && route == "/api/v2/resourceRecords/100921":
			return []byte{}, nil
		case method == "DELETE" && route == "/api/v2/addresses/100914":
			return []byte{}, nil
		}
		return nil, errorf("unexpected %s %s", method, route)
	}
	ips := NewIpAddressService(ss.mock)

	if err := ips.DeleteIpAddress("10.5.0.10"); err != nil {
		t.Fatalf("DeleteIpAddress: %v", err)
	}
}

func TestIpAddressService_DeleteIpAddress_NotFound(t *testing.T) {
	ss := newScriptedServer(map[string]scriptedResponse{
		"GET /api/v2/addresses": {body: `{"count":0,"data":[]}`},
	})
	ips := NewIpAddressService(ss.mock)

	err := ips.DeleteIpAddress("10.5.99.99")
	var notFound *ErrEntityNotFound
	if !errors.As(err, &notFound) {
		t.Errorf("err = %v (type %T), want *ErrEntityNotFound", err, err)
	}
}

// TestIpAddressService_AssignIpAddress_TwoCallSequence verifies the two-POST
// sequence: address allocation then HostRecord creation. The first POST's
// id+address must thread into the HostRecord body as an addresses[].id ref.
func TestIpAddressService_AssignIpAddress_TwoCallSequence(t *testing.T) {
	ss := newScriptedServer(nil)
	ss.mock.MakeRequestFunc = func(method, route, queryParam string, body io.Reader) ([]byte, error) {
		switch {
		case method == "POST" && route == "/api/v2/networks/200001/addresses":
			b, _ := io.ReadAll(body)
			var got map[string]interface{}
			_ = json.Unmarshal(b, &got)
			if got["type"] != "IPv4Address" {
				t.Errorf("address POST type = %v, want IPv4Address", got["type"])
			}
			if got["state"] != "STATIC" {
				t.Errorf("address POST state = %v, want STATIC", got["state"])
			}
			if mac, ok := got["macAddress"].(map[string]interface{}); !ok || mac["address"] != "02:00:5e:00:00:01" {
				t.Errorf("address POST macAddress = %v, want nested {address: ...}", got["macAddress"])
			}
			if !strings.Contains(queryParam, "x-bcn-create-reverse-record=true") {
				t.Errorf("address POST query missing reverse-record header param: %s", queryParam)
			}
			return []byte(`{"id":100914,"type":"IPv4Address","address":"10.5.0.10","name":"host.spinuptest.internal","state":"STATIC"}`), nil
		case method == "GET" && route == "/api/v2/views/100902/zones":
			return []byte(`{"count":1,"data":[{"id":100911}]}`), nil
		case method == "GET" && route == "/api/v2/zones/100911/zones":
			return []byte(`{"count":1,"data":[{"id":100913}]}`), nil
		case method == "POST" && route == "/api/v2/zones/100913/resourceRecords":
			b, _ := io.ReadAll(body)
			var got map[string]interface{}
			_ = json.Unmarshal(b, &got)
			if got["type"] != "HostRecord" || got["name"] != "host" {
				t.Errorf("host record body type/name = %v/%v, want HostRecord/host", got["type"], got["name"])
			}
			addrs, _ := got["addresses"].([]interface{})
			if len(addrs) != 1 {
				t.Fatalf("addresses len = %d, want 1", len(addrs))
			}
			ref := addrs[0].(map[string]interface{})
			if int(ref["id"].(float64)) != 100914 {
				t.Errorf("host record addresses[0].id = %v, want 100914", ref["id"])
			}
			return []byte(`{"id":100920,"type":"HostRecord","name":"host","absoluteName":"host.spinuptest.internal"}`), nil
		}
		return nil, errorf("unexpected %s %s", method, route)
	}
	ips := NewIpAddressService(ss.mock)

	entity, err := ips.AssignIpAddress("MAKE_STATIC", "02:00:5e:00:00:01", 200001,
		map[string]string{
			"hostname":       "host.spinuptest.internal",
			"viewId":         "100902",
			"reverseFlag":    "true",
			"sameAsZoneFlag": "false",
		},
		map[string]string{"name": "host.spinuptest.internal"})
	if err != nil {
		t.Fatalf("AssignIpAddress: %v", err)
	}
	if entity.ID != 100914 {
		t.Errorf("returned entity ID = %d, want 100914 (the address id, not the record id)", entity.ID)
	}
	if entity.Properties["address"] != "10.5.0.10" {
		t.Errorf("Properties[address] = %q, want 10.5.0.10", entity.Properties["address"])
	}
}

// TestIpAddressService_AssignIpAddress_RollsBackAddressOnRecordFailure
// verifies the cleanup semantics: if the host-record POST fails after
// address allocation, the address is DELETEd so retries see a clean slate.
func TestIpAddressService_AssignIpAddress_RollsBackAddressOnRecordFailure(t *testing.T) {
	deleted := false
	ss := newScriptedServer(nil)
	ss.mock.MakeRequestFunc = func(method, route, queryParam string, body io.Reader) ([]byte, error) {
		switch {
		case method == "POST" && route == "/api/v2/networks/200001/addresses":
			return []byte(`{"id":100914,"type":"IPv4Address","address":"10.5.0.10"}`), nil
		case method == "GET" && route == "/api/v2/views/100902/zones":
			return []byte(`{"count":1,"data":[{"id":100911}]}`), nil
		case method == "GET" && route == "/api/v2/zones/100911/zones":
			// Pretend the zone walk fails — host record can't be created.
			return []byte(`{"count":0,"data":[]}`), nil
		case method == "DELETE" && route == "/api/v2/addresses/100914":
			deleted = true
			return []byte{}, nil
		}
		return nil, errorf("unexpected %s %s", method, route)
	}
	ips := NewIpAddressService(ss.mock)

	_, err := ips.AssignIpAddress("MAKE_STATIC", "", 200001,
		map[string]string{"hostname": "host.spinuptest.internal", "viewId": "100902", "reverseFlag": "false"},
		nil)
	if err == nil {
		t.Fatal("expected error when host record creation fails, got nil")
	}
	if !deleted {
		t.Error("address was not rolled back after host record failure")
	}
}

func TestIpAddressService_AssignIpAddress_RejectsMissingHostnameOrView(t *testing.T) {
	ips := NewIpAddressService(newScriptedServer(nil).mock)

	if _, err := ips.AssignIpAddress("MAKE_STATIC", "", 200001,
		map[string]string{"viewId": "100902", "reverseFlag": "true"}, nil); err == nil {
		t.Error("expected error for missing hostname")
	}

	if _, err := ips.AssignIpAddress("MAKE_STATIC", "", 200001,
		map[string]string{"hostname": "x.y", "viewId": "abc", "reverseFlag": "true"}, nil); err == nil {
		t.Error("expected error for non-integer viewId")
	}
}

// TestIpAddressService_ParentIDFromCIDR_Success verifies the v2 simplification
// — replacing the v1 probe-and-walk with a single range filter.
func TestIpAddressService_ParentIDFromCIDR_Success(t *testing.T) {
	ss := newScriptedServer(map[string]scriptedResponse{
		"GET /api/v2/networks": {body: `{"count":1,"data":[{"id":200001}]}`},
	})
	ips := NewIpAddressService(ss.mock)

	id, err := ips.ParentIDFromCIDR("10.5.0.0/24")
	if err != nil {
		t.Fatalf("ParentIDFromCIDR: %v", err)
	}
	if id != 200001 {
		t.Errorf("id = %d, want 200001", id)
	}

	call := (*ss.calls)[0]
	if !strings.Contains(call.QueryParam, "range%3Aeq") || !strings.Contains(call.QueryParam, "10.5.0.0%2F24") {
		t.Errorf("query missing range:eq('10.5.0.0/24'): %s", call.QueryParam)
	}
}

func TestIpAddressService_ParentIDFromCIDR_NoMatch(t *testing.T) {
	ss := newScriptedServer(map[string]scriptedResponse{
		"GET /api/v2/networks": {body: `{"count":0,"data":[]}`},
	})
	ips := NewIpAddressService(ss.mock)

	if _, err := ips.ParentIDFromCIDR("10.99.0.0/24"); err == nil {
		t.Error("expected error when no network matches CIDR")
	}
}

func TestIpAddressService_ParentIDFromCIDR_EmptyCIDR(t *testing.T) {
	ips := NewIpAddressService(newScriptedServer(nil).mock)

	if _, err := ips.ParentIDFromCIDR(""); err == nil {
		t.Error("expected error for empty CIDR")
	}
}

// Sanity check on the IsNotFound path through DeleteIpAddress when the
// final DELETE 404s (concurrent delete by another caller).
func TestIpAddressService_DeleteIpAddress_404OnFinalDelete(t *testing.T) {
	ss := newScriptedServer(nil)
	ss.mock.MakeRequestFunc = func(method, route, queryParam string, body io.Reader) ([]byte, error) {
		switch {
		case method == "GET" && route == "/api/v2/addresses":
			return []byte(`{"count":1,"data":[{"id":100914,"type":"IPv4Address","address":"10.5.0.10"}]}`), nil
		case method == "GET" && route == "/api/v2/addresses/100914/resourceRecords":
			return []byte(`{"count":0,"data":[]}`), nil
		case method == "DELETE" && route == "/api/v2/addresses/100914":
			return nil, &common.BluecatAPIError{StatusCode: http.StatusNotFound, Body: `{"code":"ResourceNotFound"}`}
		}
		return nil, errorf("unexpected %s %s", method, route)
	}
	ips := NewIpAddressService(ss.mock)

	err := ips.DeleteIpAddress("10.5.0.10")
	var notFound *ErrEntityNotFound
	if !errors.As(err, &notFound) {
		t.Errorf("err = %v (type %T), want *ErrEntityNotFound", err, err)
	}
}
