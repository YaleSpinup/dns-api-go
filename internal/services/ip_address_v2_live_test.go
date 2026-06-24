package services

// Live integration tests for IpAddressService against BAM-test. Read-only
// by design — an assign+delete round-trip would have to write into the
// Spinup Testing /26, and that same CIDR exists in production BAM with no
// way for a test to tell them apart at the wire level. If you need
// mutation coverage, run it manually against a known-safe sandbox.
//
// Run with:
//   go test ./internal/services/ -run V2Live_Ip -v

import (
	"errors"
	"testing"
)

// spinupTestCIDR is the Spinup Testing leaf network in Yale BAM-test
// (10.5.0.0/16 block, /26 leaf). ParentIDFromCIDR is read-only, so this
// is safe to target directly.
const spinupTestCIDR = "10.5.0.0/26"

// TestV2Live_IpAddressService_ParentIDFromCIDR exercises the v2 range-filter
// simplification flagged in the plan as the one Phase 5B path that hadn't
// been live-validated. Targets the Spinup Testing /26 directly.
//
// If this test ever fails because BAM rejects `range:eq(...)`, fall back to
// the v1 probe-walk pattern in ParentIDFromCIDR — see plan §Risks #2.
func TestV2Live_IpAddressService_ParentIDFromCIDR(t *testing.T) {
	c := newV2Client(t)
	ips := NewIpAddressService(c)

	id, err := ips.ParentIDFromCIDR(spinupTestCIDR)
	if err != nil {
		t.Fatalf("ParentIDFromCIDR(%q): %v", spinupTestCIDR, err)
	}
	if id == 0 {
		t.Errorf("ParentIDFromCIDR(%q) = 0, want positive network ID", spinupTestCIDR)
	}
	t.Logf("range:eq('%s') → id=%d ✓", spinupTestCIDR, id)
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
