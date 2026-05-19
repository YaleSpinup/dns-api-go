package api

// Wire-contract snapshot test. This is the single test that protects the
// cross-repo contract with server-api/lib/dns/proteus.rb. It hits
// GET /v2/dns/{acct}/records?type=HostRecord&hint=... through the full
// router → middleware → handler → service → MakeRequest → BAM stack,
// then asserts the JSON response shape matches what server-api parses:
//
//   [i].id                       int     — record ID
//   [i].properties.addresses     string  — comma-separated IPs
//
// If either field drifts (int → string, addresses missing or wrapped),
// this test fails and the cross-repo contract is broken. Skip-guarded
// like every other V2Live test.

import (
	"dns-api-go/internal/services"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

func TestV2Live_RecordsWireContract(t *testing.T) {
	bcat := loadLiveBluecatConfig(t)
	if bcat.account == "" {
		t.Skip("bluecat.account missing from test config; AccountValidationMiddleware would reject")
	}

	s := &server{
		router:  mux.NewRouter(),
		bluecat: bcat,
	}
	s.services = Services{
		RecordService:    services.NewRecordService(s),
		IpAddressService: services.NewIpAddressService(s),
	}
	s.routes()
	t.Cleanup(func() { _ = s.logout() })

	url := "/v2/dns/" + bcat.account + "/records?type=HostRecord&hint=spinuptest&limit=10"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET %s → status %d, want 200; body: %s", url, w.Code, w.Body.String())
	}

	var entities []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &entities); err != nil {
		t.Fatalf("response body is not a JSON array (server-api expects []):\n%v\nbody: %s", err, w.Body.String())
	}

	if len(entities) == 0 {
		t.Skip("no HostRecord results matching hint=spinuptest; can't validate response shape — " +
			"adjust the hint or seed a record under spinuptest.internal")
	}

	for i, e := range entities {
		// id: server-api reads this as an int (proteus.rb host_id, host_ip).
		// JSON-decoded into map[string]interface{}, numbers become float64.
		if _, ok := e["id"].(float64); !ok {
			t.Errorf("entities[%d].id = %v (%T), want number — server-api won't be able to parse this as record ID",
				i, e["id"], e["id"])
		}

		// properties: must be a JSON object so server-api can walk into it.
		props, ok := e["properties"].(map[string]interface{})
		if !ok {
			t.Errorf("entities[%d].properties = %v (%T), want object", i, e["properties"], e["properties"])
			continue
		}

		// properties.addresses: comma-separated IPs as a single string.
		// server-api splits on ',' to get individual IPs (host_ip flow).
		addrs, ok := props["addresses"].(string)
		if !ok {
			t.Errorf("entities[%d].properties.addresses = %v (%T), want string (comma-separated IPs)",
				i, props["addresses"], props["addresses"])
			continue
		}
		if addrs == "" {
			t.Errorf("entities[%d].properties.addresses is empty string; HostRecord must surface ≥1 IP", i)
		}
	}

	t.Logf("validated wire contract on %d HostRecord(s): id (int) + properties.addresses (comma-joined string)", len(entities))
}
