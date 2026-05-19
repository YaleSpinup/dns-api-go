package services

// Live integration tests for RecordService driving the real v2 transport
// against BAM-test. These complement the unit tests in record_service_test.go
// (which mock MakeRequest) by validating that the v2 endpoints, filter
// predicates, and response shapes RecordService relies on match what BAM
// actually returns.
//
// Skip-guarded — see loadV2Env in v2_validation_test.go for credential
// sources. Read-only tests run by default; mutation tests are gated behind
// BLUECAT_V2_ALLOW_MUTATIONS=1 so CI / casual runs don't write to the
// shared BAM-test instance.
//
// Run with:
//   go test ./internal/services/ -run V2Live -v
//   BLUECAT_V2_ALLOW_MUTATIONS=1 go test ./internal/services/ -run V2Live -v

import (
	"dns-api-go/internal/common"
	"dns-api-go/internal/types"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

func mutationsAllowed(t *testing.T) {
	t.Helper()
	if os.Getenv("BLUECAT_V2_ALLOW_MUTATIONS") != "1" {
		t.Skip("mutation tests gated; set BLUECAT_V2_ALLOW_MUTATIONS=1 to run")
	}
}

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

// TestV2Live_RecordService_CreateAndDeleteExternalHost exercises the
// mutation path that doesn't require IP allocation. ExternalHostRecord
// creates land in the view's ExternalHostsZone — a single POST + DELETE
// round-trip. Uses a timestamped name so re-runs and concurrent runs don't
// collide on the AlreadyExists preflight.
//
// If the test fails between create and delete, the record will linger in
// BAM-test until manually cleaned up — accept that risk for the mutation
// gate's narrow blast radius.
func TestV2Live_RecordService_CreateAndDeleteExternalHost(t *testing.T) {
	mutationsAllowed(t)

	c := newV2Client(t)
	rs := NewRecordService(c)

	if c.viewID == 0 {
		t.Skip("viewId not in test config; ExternalHostRecord create needs a view")
	}

	name := fmt.Sprintf("claude-5a-%d.test.invalid", time.Now().UnixNano())

	created, err := rs.CreateRecord(types.EXTERNALHOST, map[string]interface{}{
		"name": name,
	}, c.viewID)
	if err != nil {
		t.Fatalf("CreateRecord ExternalHost: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("created record has zero ID; cannot delete safely")
	}
	t.Logf("created ExternalHostRecord id=%d name=%s", created.ID, name)

	t.Cleanup(func() {
		// Best-effort cleanup. If the in-body delete below already succeeded,
		// this returns ErrEntityNotFound which is fine to swallow.
		if err := rs.DeleteEntity(created.ID); err != nil {
			var notFound *ErrEntityNotFound
			if errors.As(err, &notFound) {
				return
			}
			t.Logf("cleanup DeleteEntity(%d): %v", created.ID, err)
		}
	})

	if err := rs.DeleteEntity(created.ID); err != nil {
		t.Fatalf("DeleteEntity(%d): %v", created.ID, err)
	}

	// Verify gone.
	if _, err := rs.GetEntity(created.ID, false); err == nil {
		t.Errorf("GetEntity(%d) succeeded after delete; expected ErrEntityNotFound", created.ID)
	} else {
		var notFound *ErrEntityNotFound
		var bcErr *common.BluecatAPIError
		if !errors.As(err, &notFound) && !errors.As(err, &bcErr) {
			t.Errorf("post-delete GetEntity err = %v (%T), want ErrEntityNotFound or 404", err, err)
		}
	}
}
