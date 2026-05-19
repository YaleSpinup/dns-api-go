package api

// Live integration tests for the BlueCat v2 transport rewrite (Phase 2/3).
// These exercise the real (s *server).MakeRequest, generateAuthToken, and
// logout against a configured BAM instance — not the private v2Client in
// internal/services/v2_validation_test.go.
//
// Credential sources, in priority order:
//   1. BLUECAT_V2_URL + BLUECAT_V2_USER + BLUECAT_V2_PASS env vars
//   2. BLUECAT_V2_CONFIG env var pointing at a config.json
//   3. ../../docker/config.json relative to this file
//
// Tests are skip-guarded: when creds are unreachable they Skip rather than
// fail, so the package's offline `go test ./internal/api/...` stays green.
//
// Run with: go test ./internal/api/ -run V2Live -v
//
// All calls are read-only GETs scoped to existing resources on BAM-test.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func loadLiveBluecatConfig(t *testing.T) *bluecat {
	t.Helper()

	if u := os.Getenv("BLUECAT_V2_URL"); u != "" {
		user := os.Getenv("BLUECAT_V2_USER")
		pass := os.Getenv("BLUECAT_V2_PASS")
		if user == "" || pass == "" {
			t.Skipf("BLUECAT_V2_URL set but BLUECAT_V2_USER/PASS missing")
		}
		return &bluecat{baseUrl: u, user: user, password: pass}
	}

	cfgPath := os.Getenv("BLUECAT_V2_CONFIG")
	if cfgPath == "" {
		cfgPath = filepath.Join("..", "..", "docker", "config.json")
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Skipf("v2 creds unavailable: env unset and config not readable at %s: %v", cfgPath, err)
	}

	var cfg struct {
		Bluecat struct {
			Account  string `json:"account"`
			BaseUrl  string `json:"baseUrl"`
			Username string `json:"username"`
			Password string `json:"password"`
			ViewId   string `json:"viewId"`
		} `json:"bluecat"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Skipf("could not decode %s: %v", cfgPath, err)
	}
	if cfg.Bluecat.BaseUrl == "" || cfg.Bluecat.Username == "" || cfg.Bluecat.Password == "" {
		t.Skipf("bluecat creds incomplete in %s", cfgPath)
	}

	return &bluecat{
		account:   cfg.Bluecat.Account,
		baseUrl:   strings.TrimRight(cfg.Bluecat.BaseUrl, "/"),
		user:      cfg.Bluecat.Username,
		password:  cfg.Bluecat.Password,
		viewId:    cfg.Bluecat.ViewId,
		tokenLock: sync.Mutex{},
	}
}

func newLiveServer(t *testing.T) *server {
	t.Helper()
	s := &server{bluecat: loadLiveBluecatConfig(t)}
	t.Cleanup(func() { _ = s.logout() })
	return s
}

// First-call path: empty token → generateAuthToken → MakeRequest succeeds
// against /api/v2/sessions/current. Confirms the entire auth chain
// (POST login → Basic header → 200) works end-to-end.
func TestV2Live_SessionCurrent(t *testing.T) {
	s := newLiveServer(t)

	body, err := s.MakeRequest("GET", "/api/v2/sessions/current", "", nil)
	if err != nil {
		t.Fatalf("MakeRequest /api/v2/sessions/current: %v", err)
	}

	var session map[string]interface{}
	if err := json.Unmarshal(body, &session); err != nil {
		t.Fatalf("decode session response: %v\nbody: %s", err, string(body))
	}
	if state, _ := session["state"].(string); state != "LOGGED_IN" {
		t.Errorf("session state = %v, want LOGGED_IN; body: %s", session["state"], string(body))
	}
	if s.bluecat.sessionID == 0 {
		t.Error("sessionID was not stored on bluecat struct after login")
	}
	if s.bluecat.token == "" {
		t.Error("token was not stored on bluecat struct after login")
	}
}

// Collection fetch: verifies the v2 HAL+JSON envelope (`count`, `data[]`)
// is reachable through the new transport.
func TestV2Live_ListConfigurations(t *testing.T) {
	s := newLiveServer(t)

	body, err := s.MakeRequest("GET", "/api/v2/configurations", "limit=1", nil)
	if err != nil {
		t.Fatalf("MakeRequest /api/v2/configurations: %v", err)
	}

	var page struct {
		Count int                      `json:"count"`
		Data  []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("decode configurations response: %v\nbody: %s", err, string(body))
	}
	if len(page.Data) == 0 {
		t.Errorf("expected at least one configuration, got count=%d body=%s", page.Count, string(body))
	}
}

// 404 path: a deliberately bogus configuration ID must surface as
// *BluecatAPIError with StatusCode=404, and IsNotFound must recognize it.
// This validates that v2 actually returns 404 for missing entities (the
// behavior Phase 4/5 callers will depend on).
func TestV2Live_NotFoundError(t *testing.T) {
	s := newLiveServer(t)

	_, err := s.MakeRequest("GET", "/api/v2/configurations/999999999", "", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent configuration, got nil")
	}
	if !IsNotFound(err) {
		t.Errorf("IsNotFound(%v) = false, want true (err type: %T)", err, err)
	}
}

// 401 rotation path: poison the cached token with garbage but keep
// sessionID non-zero so the code thinks a session is in flight. The next
// MakeRequest must hit 401, clear the cached creds, re-authenticate, and
// succeed on the second attempt — all transparently.
func TestV2Live_401TriggersRotation(t *testing.T) {
	s := newLiveServer(t)

	// Prime the cache with a valid session.
	if _, err := s.MakeRequest("GET", "/api/v2/sessions/current", "", nil); err != nil {
		t.Fatalf("prime auth: %v", err)
	}
	originalSessionID := s.bluecat.sessionID
	if originalSessionID == 0 {
		t.Fatal("priming did not establish a session")
	}

	// Poison the credentials. We keep the struct in a "session looks live"
	// shape (non-zero sessionID, non-empty token) so MakeRequest doesn't
	// short-circuit through getToken's empty-token fast path.
	s.bluecat.tokenLock.Lock()
	s.bluecat.token = "deliberatelyInvalidCredentials=="
	s.bluecat.tokenLock.Unlock()

	body, err := s.MakeRequest("GET", "/api/v2/sessions/current", "", nil)
	if err != nil {
		t.Fatalf("MakeRequest after poisoning token: expected transparent retry, got %v", err)
	}

	var session map[string]interface{}
	if err := json.Unmarshal(body, &session); err != nil {
		t.Fatalf("decode session response after rotation: %v\nbody: %s", err, string(body))
	}
	if state, _ := session["state"].(string); state != "LOGGED_IN" {
		t.Errorf("post-rotation session state = %v, want LOGGED_IN", session["state"])
	}
	if s.bluecat.sessionID == 0 {
		t.Error("sessionID was zeroed after rotation but never refilled")
	}
}

// SystemInfoHandler: drives the real handler against BAM-test and asserts
// it returns a populated map containing the SystemSettings fields we expect
// to see from any BlueCat instance (version, hostname, address).
func TestV2Live_SystemInfoHandler(t *testing.T) {
	s := newLiveServer(t)

	req, _ := http.NewRequest("GET", "/v2/dns/systeminfo", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(s.SystemInfoHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var info map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &info); err != nil {
		t.Fatalf("decode response: %v\nbody: %s", err, rr.Body.String())
	}
	for _, key := range []string{"hostname", "version", "address"} {
		if info[key] == "" {
			t.Errorf("info[%q] is empty; full response: %v", key, info)
		}
	}
	if info["type"] != "SystemSettings" {
		t.Errorf("type = %q, want SystemSettings", info["type"])
	}
	if _, ok := info["_links"]; ok {
		t.Error("_links leaked into response")
	}
}

// Logout path: confirms PATCH /api/v2/sessions/current with LOGGED_OUT
// reaches BAM, and that the subsequent MakeRequest transparently opens a
// new session (proving local state was cleared properly).
func TestV2Live_LogoutAndReauth(t *testing.T) {
	s := newLiveServer(t)

	// Open a session.
	if _, err := s.MakeRequest("GET", "/api/v2/sessions/current", "", nil); err != nil {
		t.Fatalf("initial auth: %v", err)
	}
	if s.bluecat.token == "" || s.bluecat.sessionID == 0 {
		t.Fatal("session not established")
	}

	if err := s.logout(); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if s.bluecat.token != "" || s.bluecat.sessionID != 0 {
		t.Errorf("logout left state: token=%q sessionID=%d", s.bluecat.token, s.bluecat.sessionID)
	}

	// Next call must transparently re-auth via the empty-token path in
	// getToken.
	if _, err := s.MakeRequest("GET", "/api/v2/sessions/current", "", nil); err != nil {
		t.Fatalf("post-logout MakeRequest: %v", err)
	}
	if s.bluecat.sessionID == 0 {
		t.Error("re-auth after logout did not establish a new session")
	}
}
