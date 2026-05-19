package api

import (
	"encoding/json"
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
