package bluecat

import (
	"dns-api-go/logger"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	logger.InitializeDefault()
	os.Exit(m.Run())
}

func TestAuthenticate_Success(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/sessions" {
			t.Errorf("expected path /api/v2/sessions, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		// Verify Basic auth
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			t.Errorf("expected Basic auth admin:secret, got %s:%s (ok=%v)", user, pass, ok)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(V2SessionResponse{APIToken: "test-token-123"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "secret")
	client.httpClient = server.Client()

	token, err := client.Authenticate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "test-token-123" {
		t.Errorf("expected token 'test-token-123', got '%s'", token)
	}
}

func TestAuthenticate_Failure(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("invalid credentials"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "wrong")
	client.httpClient = server.Client()

	_, err := client.Authenticate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("expected 'authentication failed' in error, got: %v", err)
	}
}

func TestDo_BearerToken(t *testing.T) {
	var capturedAuth string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/sessions" {
			json.NewEncoder(w).Encode(V2SessionResponse{APIToken: "bearer-tok"})
			return
		}
		capturedAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"id":1,"name":"test","type":"Zone"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "secret")
	client.httpClient = server.Client()

	resp, err := client.Do("GET", "/api/v2/zones/1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedAuth != "Bearer bearer-tok" {
		t.Errorf("expected 'Bearer bearer-tok', got '%s'", capturedAuth)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestDo_TokenRefreshOn401(t *testing.T) {
	callCount := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/sessions" {
			callCount++
			json.NewEncoder(w).Encode(V2SessionResponse{
				APIToken: "token-" + string(rune('0'+callCount)),
			})
			return
		}
		auth := r.Header.Get("Authorization")
		if auth == "Bearer token-1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"id":42}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "secret")
	client.httpClient = server.Client()

	resp, err := client.Do("GET", "/api/v2/entities/42", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 auth calls (initial + refresh), got %d", callCount)
	}
	if resp == nil {
		t.Fatal("expected non-nil response after token refresh")
	}
}

func TestDo_404ReturnsNil(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/sessions" {
			json.NewEncoder(w).Encode(V2SessionResponse{APIToken: "tok"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "secret")
	client.httpClient = server.Client()

	resp, err := client.Do("GET", "/api/v2/entities/999", nil)
	if err != nil {
		t.Fatalf("expected no error for 404, got: %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil response for 404, got: %s", string(resp))
	}
}

func TestDo_ServerError(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/sessions" {
			json.NewEncoder(w).Encode(V2SessionResponse{APIToken: "tok"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "secret")
	client.httpClient = server.Client()

	_, err := client.Do("GET", "/api/v2/entities/1", nil)
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected status code: 500") {
		t.Errorf("expected status code 500 in error, got: %v", err)
	}
}

func TestMakeRequest_QueryParams(t *testing.T) {
	var capturedURL string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/sessions" {
			json.NewEncoder(w).Encode(V2SessionResponse{APIToken: "tok"})
			return
		}
		capturedURL = r.URL.String()
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "secret")
	client.httpClient = server.Client()

	_, err := client.MakeRequest("GET", "/api/v2/zones", "limit=10&offset=0", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedURL != "/api/v2/zones?limit=10&offset=0" {
		t.Errorf("expected '/api/v2/zones?limit=10&offset=0', got '%s'", capturedURL)
	}
}

func TestDo_JSONBody(t *testing.T) {
	var capturedBody string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/sessions" {
			json.NewEncoder(w).Encode(V2SessionResponse{APIToken: "tok"})
			return
		}
		bodyBytes := make([]byte, r.ContentLength)
		r.Body.Read(bodyBytes)
		capturedBody = string(bodyBytes)
		w.Write([]byte(`{"id":1}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "secret")
	client.httpClient = server.Client()

	body := map[string]string{"name": "test-zone"}
	_, err := client.Do("POST", "/api/v2/zones", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(capturedBody, `"name":"test-zone"`) {
		t.Errorf("expected JSON body with name, got: %s", capturedBody)
	}
}

func TestTokenCaching(t *testing.T) {
	authCalls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/sessions" {
			authCalls++
			json.NewEncoder(w).Encode(V2SessionResponse{APIToken: "cached-tok"})
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "secret")
	client.httpClient = server.Client()

	// Make two requests — should only authenticate once
	client.Do("GET", "/api/v2/zones/1", nil)
	client.Do("GET", "/api/v2/zones/2", nil)

	if authCalls != 1 {
		t.Errorf("expected 1 auth call (token should be cached), got %d", authCalls)
	}
}

