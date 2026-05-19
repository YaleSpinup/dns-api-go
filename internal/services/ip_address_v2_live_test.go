package services

// Live integration tests for IpAddressService against BAM-test. Standing
// pattern from Phase 5: read-only tests run by default; mutation tests
// gated behind BLUECAT_V2_ALLOW_MUTATIONS=1.
//
// Run with:
//   go test ./internal/services/ -run V2Live_Ip -v
//   BLUECAT_V2_ALLOW_MUTATIONS=1 go test ./internal/services/ -run V2Live_Ip -v

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"
)

// TestV2Live_IpAddressService_ParentIDFromCIDR exercises the v2 range-filter
// simplification flagged in the plan as the one Phase 5B path that hadn't
// been live-validated. Uses the Spinup Testing block's CIDR.
//
// If this test fails because BAM-test rejects `range:eq(...)` or returns
// no match, fall back to the v1 probe-walk pattern in ParentIDFromCIDR
// rather than blocking the phase — see plan §Risks #2.
func TestV2Live_IpAddressService_ParentIDFromCIDR(t *testing.T) {
	c := newV2Client(t)
	ips := NewIpAddressService(c)

	cidr := "10.5.0.0/24"
	id, err := ips.ParentIDFromCIDR(cidr)
	if err != nil {
		t.Fatalf("ParentIDFromCIDR(%q): %v\n"+
			"If this is the first time range:eq has been tried live, double-check the "+
			"OpenAPI filter grammar — the fallback is the v1 probe-walk.", cidr, err)
	}
	if id == 0 {
		t.Errorf("ParentIDFromCIDR(%q) returned id=0, want positive network ID", cidr)
	}
	t.Logf("network id for %s = %d", cidr, id)
}

// TestV2Live_IpAddressService_GetIpAddress_NotFound confirms the
// empty-collection → ErrEntityNotFound path against the real API.
func TestV2Live_IpAddressService_GetIpAddress_NotFound(t *testing.T) {
	c := newV2Client(t)
	ips := NewIpAddressService(c)

	_, err := ips.GetIpAddress("10.255.255.254")
	if err == nil {
		t.Fatal("expected error for unallocated IP, got nil")
	}
	var notFound *ErrEntityNotFound
	if !errors.As(err, &notFound) {
		t.Errorf("err = %v (type %T), want *ErrEntityNotFound", err, err)
	}
}

// TestV2Live_IpAddressService_AssignAndDelete walks the full v1 wire
// contract through v2 plumbing:
//   - Assign next-available IP + create host record + PTR (single service call)
//   - Verify GetIpAddress finds the new IP
//   - Delete IP (which cascades to host record removal)
//   - Verify GetIpAddress returns NotFound
//
// Gated behind BLUECAT_V2_ALLOW_MUTATIONS=1 — leaves no residue on success;
// on failure the IP/record may linger and need manual cleanup.
func TestV2Live_IpAddressService_AssignAndDelete(t *testing.T) {
	mutationsAllowed(t)

	c := newV2Client(t)
	ips := NewIpAddressService(c)

	if c.viewID == 0 {
		t.Skip("viewId not in test config; host-record creation needs a view")
	}

	parentID, err := ips.ParentIDFromCIDR("10.5.0.0/24")
	if err != nil {
		t.Fatalf("resolve parent for 10.5.0.0/24: %v", err)
	}

	hostname := fmt.Sprintf("claude-5b-%d.spinuptest.internal", time.Now().UnixNano())
	hostInfo := map[string]string{
		"hostname":       hostname,
		"viewId":         strconv.Itoa(c.viewID),
		"reverseFlag":    "true",
		"sameAsZoneFlag": "false",
	}

	entity, err := ips.AssignIpAddress("MAKE_STATIC", "", parentID, hostInfo, map[string]string{"name": hostname})
	if err != nil {
		t.Fatalf("AssignIpAddress: %v", err)
	}
	if entity.ID == 0 || entity.Properties["address"] == "" {
		t.Fatalf("assign returned bad entity: id=%d address=%q", entity.ID, entity.Properties["address"])
	}
	allocatedIP := entity.Properties["address"]
	allocatedID := entity.ID
	t.Logf("allocated id=%d address=%s host=%s", allocatedID, allocatedIP, hostname)

	t.Cleanup(func() {
		// Belt-and-suspenders cleanup: if the in-body delete already ran,
		// this returns ErrEntityNotFound which is fine.
		if err := ips.DeleteIpAddress(allocatedIP); err != nil {
			var notFound *ErrEntityNotFound
			if errors.As(err, &notFound) {
				return
			}
			t.Logf("cleanup DeleteIpAddress(%s): %v", allocatedIP, err)
		}
	})

	// Round-trip lookup confirms the address landed.
	if got, err := ips.GetIpAddress(allocatedIP); err != nil {
		t.Errorf("GetIpAddress(%s) after assign: %v", allocatedIP, err)
	} else if got.ID != allocatedID {
		t.Errorf("GetIpAddress round-trip id = %d, want %d", got.ID, allocatedID)
	}

	if err := ips.DeleteIpAddress(allocatedIP); err != nil {
		t.Fatalf("DeleteIpAddress(%s): %v", allocatedIP, err)
	}

	if _, err := ips.GetIpAddress(allocatedIP); err == nil {
		t.Errorf("GetIpAddress(%s) succeeded after delete; expected ErrEntityNotFound", allocatedIP)
	} else {
		var notFound *ErrEntityNotFound
		if !errors.As(err, &notFound) {
			t.Errorf("post-delete GetIpAddress err = %v (%T), want ErrEntityNotFound", err, err)
		}
	}
}
