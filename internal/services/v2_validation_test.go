package services

// BlueCat v2 endpoint validation harness. These tests hit a live BAM and
// are skipped unless creds are reachable.
//
// Sources, in priority order:
//   1. BLUECAT_V2_URL + BLUECAT_V2_USER + BLUECAT_V2_PASS env vars
//   2. BLUECAT_V2_CONFIG env var pointing at a config.json
//   3. ../../docker/config.json relative to this file
//
// Run with: go test ./internal/services/ -run V2 -v
//
// Goal: document v2 response shapes against Yale's BAM-test instance,
// scoped to the Spinup Testing block (10.5.0.0/16). No mutating calls.

import (
	"crypto/tls"
	"dns-api-go/internal/common"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type v2EnvSource struct {
	BaseURL  string
	Username string
	Password string
	ViewID   int
}

type v2Client struct {
	baseURL   string
	authBasic string
	sessionID int
	viewID    int
	http      *http.Client
	t         *testing.T
}

func loadV2Env(t *testing.T) v2EnvSource {
	t.Helper()

	if u := os.Getenv("BLUECAT_V2_URL"); u != "" {
		return v2EnvSource{
			BaseURL:  u,
			Username: os.Getenv("BLUECAT_V2_USER"),
			Password: os.Getenv("BLUECAT_V2_PASS"),
		}
	}

	cfgPath := os.Getenv("BLUECAT_V2_CONFIG")
	if cfgPath == "" {
		cfgPath = filepath.Join("..", "..", "docker", "config.json")
	}

	f, err := os.Open(cfgPath)
	if err != nil {
		t.Skipf("v2 creds unavailable: env unset and config not readable at %s: %v", cfgPath, err)
	}
	defer f.Close()

	var cfg struct {
		Bluecat struct {
			BaseUrl  string `json:"baseUrl"`
			Username string `json:"username"`
			Password string `json:"password"`
			ViewId   string `json:"viewId"`
		} `json:"bluecat"`
	}
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		t.Skipf("could not decode %s: %v", cfgPath, err)
	}
	if cfg.Bluecat.BaseUrl == "" || cfg.Bluecat.Username == "" || cfg.Bluecat.Password == "" {
		t.Skipf("bluecat creds incomplete in %s", cfgPath)
	}
	viewID, _ := strconv.Atoi(cfg.Bluecat.ViewId)
	return v2EnvSource{
		BaseURL:  cfg.Bluecat.BaseUrl,
		Username: cfg.Bluecat.Username,
		Password: cfg.Bluecat.Password,
		ViewID:   viewID,
	}
}

func newV2Client(t *testing.T) *v2Client {
	t.Helper()
	env := loadV2Env(t)

	c := &v2Client{
		baseURL: strings.TrimRight(env.BaseURL, "/"),
		viewID:  env.ViewID,
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
		t: t,
	}
	c.login(env.Username, env.Password)
	t.Cleanup(c.logout)
	return c
}

// MakeRequest lets v2Client double as a interfaces.ServerInterface so live
// tests in this package can drive services (RecordService, IpAddressService)
// against BAM-test without standing up the full *api.server. Non-2xx
// statuses surface as *common.BluecatAPIError so callers' IsNotFound paths
// behave identically to the production transport.
func (c *v2Client) MakeRequest(method, route, queryParam string, body io.Reader) ([]byte, error) {
	fullURL := c.baseURL + route
	if queryParam != "" {
		fullURL += "?" + queryParam
	}

	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authBasic)
	req.Header.Set("Accept", "application/hal+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNoContent {
		return []byte{}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &common.BluecatAPIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	return respBody, nil
}

// GetCIDRFile satisfies interfaces.ServerInterface. The /ips/cidrs route
// reads a local file in production and is not exercised by live tests.
func (c *v2Client) GetCIDRFile() (string, error) { return "", nil }

// ConfigurationID satisfies interfaces.ServerInterface. v2Client doesn't
// cache a configuration ID; callers that need one (e.g. IpAddressService)
// will hit the v2 fallback path in GetConfigID.
func (c *v2Client) ConfigurationID() (int, bool) { return 0, false }

func (c *v2Client) login(user, pass string) {
	c.t.Helper()

	payload, err := json.Marshal(map[string]string{
		"type":     "UserSession",
		"username": user,
		"password": pass,
	})
	if err != nil {
		c.t.Fatalf("marshal login payload: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/v2/sessions", strings.NewReader(string(payload)))
	if err != nil {
		c.t.Fatalf("build login request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/hal+json")

	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Skipf("BlueCat v2 unreachable at %s: %v", c.baseURL, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		c.t.Fatalf("login: status %d, body %s", resp.StatusCode, string(body))
	}

	var out struct {
		ID                            int    `json:"id"`
		APIToken                      string `json:"apiToken"`
		BasicAuthenticationCredentials string `json:"basicAuthenticationCredentials"`
		State                         string `json:"state"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		c.t.Fatalf("decode login: %v\n%s", err, string(body))
	}
	if out.BasicAuthenticationCredentials == "" {
		c.t.Fatalf("login: empty basicAuthenticationCredentials")
	}

	c.authBasic = "Basic " + out.BasicAuthenticationCredentials
	c.sessionID = out.ID
	c.t.Logf("logged in: sessionID=%d state=%s", out.ID, out.State)
}

func (c *v2Client) logout() {
	if c.sessionID == 0 {
		return
	}
	req, _ := http.NewRequest(http.MethodPatch, c.baseURL+"/api/v2/sessions/current", strings.NewReader(`{"state":"LOGGED_OUT"}`))
	req.Header.Set("Authorization", c.authBasic)
	req.Header.Set("Content-Type", "application/merge-patch+json")
	req.Header.Set("Accept", "application/hal+json")
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Logf("logout failed: %v", err)
		return
	}
	resp.Body.Close()
	c.t.Logf("logged out: sessionID=%d status=%d", c.sessionID, resp.StatusCode)
}

func (c *v2Client) get(path string) []byte {
	c.t.Helper()

	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		c.t.Fatalf("build GET %s: %v", path, err)
	}
	req.Header.Set("Authorization", c.authBasic)
	req.Header.Set("Accept", "application/hal+json")

	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.t.Fatalf("GET %s: status %d, body %s", path, resp.StatusCode, string(body))
	}
	return body
}

type v2NamedEntity struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

func decodeNamedCollection(t *testing.T, body []byte) []v2NamedEntity {
	t.Helper()
	var wrap struct {
		Data []v2NamedEntity `json:"data"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		t.Fatalf("decode collection: %v\n%s", err, string(body))
	}
	return wrap.Data
}

func findByName(ents []v2NamedEntity, name string) *v2NamedEntity {
	for i := range ents {
		if ents[i].Name == name {
			return &ents[i]
		}
	}
	return nil
}

func findByNameAndType(ents []v2NamedEntity, name, entityType string) *v2NamedEntity {
	for i := range ents {
		if ents[i].Name == name && ents[i].Type == entityType {
			return &ents[i]
		}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…(truncated)"
}

// TestV2_Auth confirms login + /sessions/current round-trip.
func TestV2_Auth(t *testing.T) {
	c := newV2Client(t)
	body := c.get("/api/v2/sessions/current")
	var sess struct {
		ID    int    `json:"id"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(body, &sess); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if sess.State != "LOGGED_IN" {
		t.Errorf("expected LOGGED_IN, got %q", sess.State)
	}
	t.Logf("/sessions/current → %s", truncate(string(body), 600))
}

// TestV2_DiscoverFixtures walks from configuration → view → spinuptest zone,
// and exercises the endpoints the migration will rely on. Scoped to the
// Spinup Testing block (10.5.0.0/16). Read-only.
func TestV2_DiscoverFixtures(t *testing.T) {
	c := newV2Client(t)

	cfg := pickFirst(t, c, "/api/v2/configurations?limit=5", "configurations")
	t.Logf("configuration: id=%d type=%q name=%q", cfg.ID, cfg.Type, cfg.Name)

	internalView := requireByName(t, c,
		fmt.Sprintf("/api/v2/configurations/%d/views?filter=%s", cfg.ID, url.QueryEscape("name:eq('internal')")),
		"internal", "internal view")
	t.Logf("view: id=%d", internalView.ID)

	block := requireByName(t, c,
		"/api/v2/blocks?filter="+url.QueryEscape("name:eq('Spinup Testing')")+"&limit=5",
		"Spinup Testing", "Spinup Testing block")
	t.Logf("block: id=%d", block.ID)

	net := pickFirst(t, c, fmt.Sprintf("/api/v2/blocks/%d/networks?limit=1", block.ID), "Spinup Testing networks")
	t.Logf("network: id=%d name=%q", net.ID, net.Name)

	// View 100902 has two zones named "internal" (ExternalHostsZone + Zone);
	// pick the regular Zone so we can drill into spinuptest beneath it.
	intZoneBody := c.get(fmt.Sprintf("/api/v2/views/%d/zones?filter=%s", internalView.ID, url.QueryEscape("name:eq('internal')")))
	intZoneCandidates := decodeNamedCollection(t, intZoneBody)
	intZone := findByNameAndType(intZoneCandidates, "internal", "Zone")
	if intZone == nil {
		t.Fatalf("top-level internal Zone not found among: %+v", intZoneCandidates)
	}
	t.Logf("internal zone: id=%d", intZone.ID)

	spinupZone := requireByName(t, c,
		fmt.Sprintf("/api/v2/zones/%d/zones?filter=%s", intZone.ID, url.QueryEscape("name:eq('spinuptest')")),
		"spinuptest", "spinuptest sub-zone")
	t.Logf("spinuptest zone: id=%d", spinupZone.ID)

	// Records in spinuptest — validates HostRecord shape (addresses is array-of-objects)
	recBody := c.get(fmt.Sprintf("/api/v2/zones/%d/resourceRecords?limit=5", spinupZone.ID))
	t.Logf("zones/%d/resourceRecords:\n%s", spinupZone.ID, truncate(string(recBody), 1200))

	var recCol struct {
		Data []struct {
			ID           int    `json:"id"`
			Type         string `json:"type"`
			Name         string `json:"name"`
			AbsoluteName string `json:"absoluteName"`
			Addresses    []struct {
				ID      int    `json:"id"`
				Type    string `json:"type"`
				Address string `json:"address"`
			} `json:"addresses"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recBody, &recCol); err != nil {
		t.Fatalf("decode records: %v", err)
	}
	for _, rec := range recCol.Data {
		if rec.Type != "HostRecord" {
			continue
		}
		if len(rec.Addresses) == 0 {
			t.Errorf("HostRecord id=%d (%s) has empty addresses[] — contract violation", rec.ID, rec.AbsoluteName)
			continue
		}
		for _, a := range rec.Addresses {
			if a.Address == "" {
				t.Errorf("HostRecord id=%d address entry missing 'address' string", rec.ID)
			}
		}
	}

	// Filter records globally by absoluteName (v1 getHostRecordsByHint analog)
	filterBody := c.get("/api/v2/resourceRecords?filter=" + url.QueryEscape("absoluteName:contains('spinuptest')") + "&limit=3")
	t.Logf("filter absoluteName contains 'spinuptest':\n%s", truncate(string(filterBody), 800))

	// Addresses under the testing network
	addrBody := c.get(fmt.Sprintf("/api/v2/networks/%d/addresses?limit=5", net.ID))
	t.Logf("networks/%d/addresses:\n%s", net.ID, truncate(string(addrBody), 800))

	// Filter address by value (v1 getIP4Address analog) — pick a STATIC address from the list above
	var addrCol struct {
		Data []struct {
			Address string `json:"address"`
			State   string `json:"state"`
		} `json:"data"`
	}
	if err := json.Unmarshal(addrBody, &addrCol); err == nil {
		for _, a := range addrCol.Data {
			if a.State != "STATIC" {
				continue
			}
			vb := c.get("/api/v2/addresses?filter=" + url.QueryEscape(fmt.Sprintf("address:eq('%s')", a.Address)))
			t.Logf("addresses?filter=address:eq('%s'):\n%s", a.Address, truncate(string(vb), 600))
			break
		}
	}

	// MAC addresses under the configuration
	macBody := c.get(fmt.Sprintf("/api/v2/configurations/%d/macAddresses?limit=3", cfg.ID))
	t.Logf("configurations/%d/macAddresses:\n%s", cfg.ID, truncate(string(macBody), 800))

	// Pagination shape — same listing with offset
	pageBody := c.get(fmt.Sprintf("/api/v2/zones/%d/resourceRecords?offset=1&limit=1", spinupZone.ID))
	t.Logf("pagination (offset=1 limit=1):\n%s", truncate(string(pageBody), 600))
}

func pickFirst(t *testing.T, c *v2Client, path, label string) v2NamedEntity {
	t.Helper()
	body := c.get(path)
	ents := decodeNamedCollection(t, body)
	if len(ents) == 0 {
		t.Fatalf("%s: no entries returned\n%s", label, string(body))
	}
	return ents[0]
}

func requireByName(t *testing.T, c *v2Client, path, name, label string) v2NamedEntity {
	t.Helper()
	body := c.get(path)
	ents := decodeNamedCollection(t, body)
	hit := findByName(ents, name)
	if hit == nil {
		t.Fatalf("%s: did not find %q in response\n%s", label, name, truncate(string(body), 600))
	}
	return *hit
}
