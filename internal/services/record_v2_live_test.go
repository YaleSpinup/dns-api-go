package services

// Live integration tests for RecordService driving the real v2 transport
// against BAM-test. These complement the unit tests in record_service_test.go
// (which mock MakeRequest) by validating that the v2 endpoints, filter
// predicates, and response shapes RecordService relies on match what BAM
// actually returns.
//
// Skip-guarded — see loadV2Env in v2_validation_test.go for credential
// sources. Read-only by design: a v2 BAM lookup against the production
// instance is fine, but create/delete round-trips against a shared BAM
// (no way to distinguish prod from test at the wire level when the same
// Spinup Testing /26 exists in both) are too risky to leave in the
// committed suite. If you need mutation coverage, run it manually
// against a known-safe sandbox.
//
// Run with:
//   go test ./internal/services/ -run V2Live -v

import (
	"dns-api-go/internal/types"
	"errors"
	"testing"
)

// TestV2Live_RecordService_HostRecordSearch drives GetRecordsByType against
// the spinuptest zone via the global resourceRecords filter path. Validates
// that the v2 hint search returns entities with the Properties["addresses"]
// shape server-api parses (comma-separated IP string).
func TestV2Live_RecordService_HostRecordSearch(t *testing.T) {
	c := newV2Client(t)
	rs := NewRecordService(c)

	entities, err := rs.GetRecordsByType(types.HOSTRECORD, map[string]interface{}{
		"count":   10,
		"start":   0,
		"options": map[string]string{"hint": "spinuptest"},
	}, c.viewID)
	if err != nil {
		t.Fatalf("GetRecordsByType: %v", err)
	}
	if entities == nil || len(*entities) == 0 {
		t.Fatal("expected at least one HostRecord in spinuptest; BAM-test fixture may have changed")
	}

	for _, e := range *entities {
		if e.Type != types.HOSTRECORD {
			t.Errorf("entity %d type = %q, want HostRecord", e.ID, e.Type)
		}
		if e.Properties["addresses"] == "" {
			t.Errorf("entity %d (%s) Properties[addresses] empty — wire contract violation", e.ID, e.Name)
		}
	}
}

// TestV2Live_RecordService_GetEntity_Roundtrip searches for a HostRecord,
// then re-fetches it by ID. Confirms /api/v2/resourceRecords/{id} returns
// the same record found by collection filter.
func TestV2Live_RecordService_GetEntity_Roundtrip(t *testing.T) {
	c := newV2Client(t)
	rs := NewRecordService(c)

	entities, err := rs.GetRecordsByType(types.HOSTRECORD, map[string]interface{}{
		"count":   1,
		"start":   0,
		"options": map[string]string{"hint": "spinuptest"},
	}, c.viewID)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if entities == nil || len(*entities) == 0 {
		t.Skip("no HostRecord under spinuptest to round-trip; skipping")
	}

	want := (*entities)[0]

	got, err := rs.GetEntity(want.ID, false)
	if err != nil {
		t.Fatalf("GetEntity(%d): %v", want.ID, err)
	}
	if got.ID != want.ID {
		t.Errorf("ID mismatch: got %d, want %d", got.ID, want.ID)
	}
	if got.Name != want.Name {
		t.Errorf("Name mismatch: got %q, want %q", got.Name, want.Name)
	}
	if got.Properties["absoluteName"] != want.Properties["absoluteName"] {
		t.Errorf("absoluteName mismatch: got %q, want %q",
			got.Properties["absoluteName"], want.Properties["absoluteName"])
	}
}

// TestV2Live_RecordService_GetEntity_NotFound confirms that a missing record
// surfaces as *ErrEntityNotFound — i.e. the common.IsNotFound path is wired
// correctly through MakeRequest.
func TestV2Live_RecordService_GetEntity_NotFound(t *testing.T) {
	c := newV2Client(t)
	rs := NewRecordService(c)

	_, err := rs.GetEntity(999999999, false)
	if err == nil {
		t.Fatal("expected error for bogus record ID, got nil")
	}
	var notFound *ErrEntityNotFound
	if !errors.As(err, &notFound) {
		t.Errorf("err = %v (type %T), want *ErrEntityNotFound", err, err)
	}
}
