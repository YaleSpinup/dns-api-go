package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServerWithBluecat(handler http.HandlerFunc) (*httptest.Server, *server) {
	ts := httptest.NewServer(handler)
	s := &server{
		bluecat: &bluecat{
			baseUrl:  ts.URL,
			user:     "alice",
			password: "s3cret",
		},
	}
	return ts, s
}

func TestGenerateAuthToken_Success(t *testing.T) {
	var capturedBody map[string]string
	var capturedMethod, capturedPath, capturedCT, capturedAccept string

	ts, s := newTestServerWithBluecat(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedCT = r.Header.Get("Content-Type")
		capturedAccept = r.Header.Get("Accept")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &capturedBody)

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{
			"id": 42,
			"apiToken": "raw-token-abc",
			"basicAuthenticationCredentials": "YWxpY2U6cmF3LXRva2VuLWFiYw==",
			"state": "LOGGED_IN"
		}`))
	})
	defer ts.Close()

	token, err := s.generateAuthToken("alice", "s3cret")
	if err != nil {
		t.Fatalf("generateAuthToken: %v", err)
	}

	if token != "YWxpY2U6cmF3LXRva2VuLWFiYw==" {
		t.Errorf("token = %q, want pre-encoded basicAuthenticationCredentials", token)
	}
	if s.bluecat.sessionID != 42 {
		t.Errorf("sessionID = %d, want 42", s.bluecat.sessionID)
	}
	if capturedMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", capturedMethod)
	}
	if capturedPath != "/api/v2/sessions" {
		t.Errorf("path = %s, want /api/v2/sessions", capturedPath)
	}
	if capturedCT != "application/json" {
		t.Errorf("Content-Type = %s, want application/json", capturedCT)
	}
	if capturedAccept != "application/hal+json" {
		t.Errorf("Accept = %s, want application/hal+json", capturedAccept)
	}
	if capturedBody["type"] != "UserSession" {
		t.Errorf("body type = %s, want UserSession", capturedBody["type"])
	}
	if capturedBody["username"] != "alice" || capturedBody["password"] != "s3cret" {
		t.Errorf("body creds = %v, want alice/s3cret", capturedBody)
	}
}

func TestGenerateAuthToken_NonCreatedStatus(t *testing.T) {
	ts, s := newTestServerWithBluecat(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"bad creds"}`))
	})
	defer ts.Close()

	_, err := s.generateAuthToken("alice", "wrong")
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
	if !strings.Contains(err.Error(), "status 401") {
		t.Errorf("error = %v, want status 401 mention", err)
	}
}

func TestGenerateAuthToken_MissingCredentialsField(t *testing.T) {
	ts, s := newTestServerWithBluecat(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":1,"apiToken":"t","state":"LOGGED_IN"}`))
	})
	defer ts.Close()

	_, err := s.generateAuthToken("alice", "s3cret")
	if err == nil {
		t.Fatal("expected error when basicAuthenticationCredentials is empty")
	}
}

func TestLogout_Success(t *testing.T) {
	var capturedMethod, capturedPath, capturedAuth, capturedCT string
	var capturedBody string

	ts, s := newTestServerWithBluecat(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		capturedCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		capturedBody = string(b)
		w.WriteHeader(http.StatusNoContent)
	})
	defer ts.Close()

	s.bluecat.token = "YWxpY2U6dG9rZW4="
	s.bluecat.sessionID = 7

	if err := s.logout(); err != nil {
		t.Fatalf("logout: %v", err)
	}

	if capturedMethod != http.MethodPatch {
		t.Errorf("method = %s, want PATCH", capturedMethod)
	}
	if capturedPath != "/api/v2/sessions/current" {
		t.Errorf("path = %s, want /api/v2/sessions/current", capturedPath)
	}
	if capturedAuth != "Basic YWxpY2U6dG9rZW4=" {
		t.Errorf("Authorization = %q, want Basic YWxpY2U6dG9rZW4=", capturedAuth)
	}
	if capturedCT != "application/merge-patch+json" {
		t.Errorf("Content-Type = %s, want application/merge-patch+json", capturedCT)
	}
	if !strings.Contains(capturedBody, `"state":"LOGGED_OUT"`) {
		t.Errorf("body = %s, want state LOGGED_OUT", capturedBody)
	}
	if s.bluecat.token != "" || s.bluecat.sessionID != 0 {
		t.Errorf("post-logout state: token=%q sessionID=%d, want zeroed", s.bluecat.token, s.bluecat.sessionID)
	}
}

func TestLogout_NoActiveSession(t *testing.T) {
	called := false
	ts, s := newTestServerWithBluecat(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	defer ts.Close()

	if err := s.logout(); err != nil {
		t.Fatalf("logout (no session): %v", err)
	}
	if called {
		t.Error("logout sent HTTP request despite no active session")
	}
}

func TestLogout_NonSuccessSwallowed(t *testing.T) {
	ts, s := newTestServerWithBluecat(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer ts.Close()

	s.bluecat.token = "creds"
	s.bluecat.sessionID = 1

	if err := s.logout(); err != nil {
		t.Errorf("logout returned err on server 500, want nil (best-effort): %v", err)
	}
	if s.bluecat.token != "" || s.bluecat.sessionID != 0 {
		t.Error("logout should clear local state even on server error")
	}
}

// makeRequestRouter routes the v2 session POST (called by getToken on the
// first MakeRequest attempt) to a fixed 201 login response, and routes
// everything else to apiHandler. It returns the count of API hits.
func makeRequestRouter(t *testing.T, apiHandler http.HandlerFunc) (*httptest.Server, *server, *int) {
	t.Helper()
	hits := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/sessions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":1,"apiToken":"t","basicAuthenticationCredentials":"YWxpY2U6dA==","state":"LOGGED_IN"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		hits++
		apiHandler(w, r)
	})
	ts := httptest.NewServer(mux)
	s := &server{
		bluecat: &bluecat{
			baseUrl:  ts.URL,
			user:     "alice",
			password: "s3cret",
		},
	}
	return ts, s, &hits
}

func TestMakeRequest_SuccessHeadersAndBody(t *testing.T) {
	var capturedAuth, capturedAccept, capturedCT string
	var capturedBody string
	ts, s, _ := makeRequestRouter(t, func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		capturedAccept = r.Header.Get("Accept")
		capturedCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		capturedBody = string(b)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	})
	defer ts.Close()

	resp, err := s.MakeRequest("POST", "/api/v2/zones", "", strings.NewReader(`{"name":"z"}`))
	if err != nil {
		t.Fatalf("MakeRequest: %v", err)
	}
	if string(resp) != `{"ok":true}` {
		t.Errorf("body = %s, want {\"ok\":true}", string(resp))
	}
	if capturedAuth != "Basic YWxpY2U6dA==" {
		t.Errorf("Authorization = %q, want Basic YWxpY2U6dA==", capturedAuth)
	}
	if capturedAccept != "application/hal+json" {
		t.Errorf("Accept = %s, want application/hal+json", capturedAccept)
	}
	if capturedCT != "application/json" {
		t.Errorf("Content-Type = %s, want application/json", capturedCT)
	}
	if capturedBody != `{"name":"z"}` {
		t.Errorf("body received by server = %s, want {\"name\":\"z\"}", capturedBody)
	}
}

func TestMakeRequest_201Created(t *testing.T) {
	ts, s, _ := makeRequestRouter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":99}`))
	})
	defer ts.Close()

	resp, err := s.MakeRequest("POST", "/api/v2/zones", "", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("MakeRequest: %v", err)
	}
	if string(resp) != `{"id":99}` {
		t.Errorf("body = %s, want {\"id\":99}", string(resp))
	}
}

func TestMakeRequest_204NoContent(t *testing.T) {
	ts, s, _ := makeRequestRouter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	defer ts.Close()

	resp, err := s.MakeRequest("DELETE", "/api/v2/zones/5", "", nil)
	if err != nil {
		t.Fatalf("MakeRequest: %v", err)
	}
	if resp == nil || len(resp) != 0 {
		t.Errorf("body = %v, want empty []byte", resp)
	}
}

func TestMakeRequest_404ReturnsBluecatAPIError(t *testing.T) {
	ts, s, _ := makeRequestRouter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"missing"}`))
	})
	defer ts.Close()

	_, err := s.MakeRequest("GET", "/api/v2/zones/999", "", nil)
	if err == nil {
		t.Fatal("expected error on 404")
	}
	var apiErr *BluecatAPIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *BluecatAPIError", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
	}
	if !IsNotFound(err) {
		t.Error("IsNotFound should be true for 404 response")
	}
}

func TestMakeRequest_500ReturnsBluecatAPIError(t *testing.T) {
	ts, s, _ := makeRequestRouter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`server boom`))
	})
	defer ts.Close()

	_, err := s.MakeRequest("GET", "/api/v2/zones", "", nil)
	var apiErr *BluecatAPIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *BluecatAPIError", err)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", apiErr.StatusCode)
	}
	if IsNotFound(err) {
		t.Error("IsNotFound should be false for 500 response")
	}
}

// Verifies the body-retry fix: a 401 followed by a 200 must replay the body
// to the second attempt, not deliver an empty reader.
func TestMakeRequest_401RetryReplaysBody(t *testing.T) {
	var bodiesSeen []string
	calls := 0
	ts, s, _ := makeRequestRouter(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		b, _ := io.ReadAll(r.Body)
		bodiesSeen = append(bodiesSeen, string(b))
		if calls == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"retried":true}`))
	})
	defer ts.Close()

	resp, err := s.MakeRequest("POST", "/api/v2/zones", "", strings.NewReader(`{"name":"replay"}`))
	if err != nil {
		t.Fatalf("MakeRequest: %v", err)
	}
	if string(resp) != `{"retried":true}` {
		t.Errorf("body = %s, want retried payload", string(resp))
	}
	if calls != 2 {
		t.Errorf("API call count = %d, want 2", calls)
	}
	if len(bodiesSeen) != 2 || bodiesSeen[0] != `{"name":"replay"}` || bodiesSeen[1] != `{"name":"replay"}` {
		t.Errorf("bodies seen = %v, want both to be the original payload", bodiesSeen)
	}
}

// Verifies the retry is bounded: two consecutive 401s surface as an error
// instead of recursing forever.
func TestMakeRequest_Persistent401Bounded(t *testing.T) {
	calls := 0
	ts, s, _ := makeRequestRouter(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"nope"}`))
	})
	defer ts.Close()

	_, err := s.MakeRequest("GET", "/api/v2/zones", "", nil)
	if err == nil {
		t.Fatal("expected error after persistent 401")
	}
	var apiErr *BluecatAPIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *BluecatAPIError on second 401", err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", apiErr.StatusCode)
	}
	if calls != 2 {
		t.Errorf("API call count = %d, want exactly 2 (one retry)", calls)
	}
}

// MakeRequest with a nil body should not panic and should not send a body.
func TestMakeRequest_NilBody(t *testing.T) {
	ts, s, _ := makeRequestRouter(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if len(b) != 0 {
			t.Errorf("expected empty body, got %q", string(b))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	})
	defer ts.Close()

	if _, err := s.MakeRequest("GET", "/api/v2/zones", "", nil); err != nil {
		t.Fatalf("MakeRequest with nil body: %v", err)
	}
}

// QueryParam should be appended as ?...
func TestMakeRequest_QueryParam(t *testing.T) {
	var capturedQuery string
	ts, s, _ := makeRequestRouter(t, func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	})
	defer ts.Close()

	_, err := s.MakeRequest("GET", "/api/v2/zones", "filter=name:eq('foo')&limit=1", nil)
	if err != nil {
		t.Fatalf("MakeRequest: %v", err)
	}
	want := "filter=name:eq('foo')&limit=1"
	if !strings.Contains(capturedQuery, "filter=name:eq") || !strings.Contains(capturedQuery, "limit=1") {
		t.Errorf("query = %q, want substring %q", capturedQuery, want)
	}
}

