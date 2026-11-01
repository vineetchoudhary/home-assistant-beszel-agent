package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func respond(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func TestHubBasePathPaginationAndSessionRefresh(t *testing.T) {
	var logins, refreshes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/beszel/api/") {
			t.Errorf("lost configured base path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		switch r.URL.Path {
		case "/beszel/api/collections/users/auth-with-password":
			logins.Add(1)
			var credentials map[string]string
			_ = json.NewDecoder(r.Body).Decode(&credentials)
			if credentials["identity"] != "reader@example.test" || credentials["password"] != "secret-do-not-log" {
				t.Error("normal account credentials were not used")
			}
			respond(w, map[string]string{"token": "old-session"})
		case "/beszel/api/collections/users/auth-refresh":
			refreshes.Add(1)
			respond(w, map[string]string{"token": "new-session"})
		case "/beszel/api/beszel/info":
			if r.Header.Get("Authorization") != "new-session" {
				w.WriteHeader(401)
				return
			}
			respond(w, HubInfo{Version: "0.20.0", Key: "ssh-ed25519 fixture"})
		case "/beszel/api/collections/systems/records":
			if r.URL.Query().Get("page") == "1" {
				respond(w, page[System]{Page: 1, TotalPages: 2, TotalItems: 2, Items: []System{{ID: "sys000000000001"}}})
			} else {
				respond(w, page[System]{Page: 2, TotalPages: 2, TotalItems: 2, Items: []System{{ID: "sys000000000002"}}})
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := testConfig()
	c.HubURL = server.URL + "/beszel"
	hub, err := newHub(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = hub.info(context.Background()); err != nil {
		t.Fatal(err)
	}
	items, err := hub.systems(context.Background())
	if err != nil || len(items.Systems) != 2 || !items.Complete {
		t.Fatalf("pagination: %v, %d", err, len(items.Systems))
	}
	if logins.Load() != 1 || refreshes.Load() != 1 {
		t.Errorf("unexpected authentication attempts: %d/%d", logins.Load(), refreshes.Load())
	}
}

func TestHubKeepsPartialInventoryAndAcceptsNewVersions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/info") {
			respond(w, HubInfo{Version: "0.99.0", Key: "ssh-ed25519 fixture"})
			return
		}
		if r.URL.Query().Get("page") == "1" {
			respond(w, page[System]{Page: 1, TotalPages: 2, TotalItems: 2, Items: []System{{ID: "sys000000000001"}}})
			return
		}
		w.WriteHeader(503)
	}))
	defer server.Close()
	c := testConfig()
	c.HubURL = server.URL
	hub, _ := newHub(c)
	hub.token = "fixture"
	if _, err := hub.info(context.Background()); err != nil {
		t.Error("new Hub version was rejected before examining its data")
	}
	items, err := hub.systems(context.Background())
	if err != nil || items.Complete || len(items.Systems) != 1 {
		t.Error("valid partial inventory was discarded or treated as authoritative")
	}
}

func TestHubDoesNotForwardSessionOutsideAPI(t *testing.T) {
	var leaked atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Store(true); respond(w, map[string]any{}) }))
	defer other.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, other.URL+"/api/beszel/info", 302) }))
	defer server.Close()
	c := testConfig()
	c.HubURL = server.URL
	hub, _ := newHub(c)
	hub.token = "private-session"
	if _, err := hub.info(context.Background()); err == nil {
		t.Error("cross-origin redirect accepted")
	}
	if leaked.Load() {
		t.Error("session request reached another server")
	}
}

func TestHubTLSValidationAndCustomCA(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respond(w, HubInfo{Version: "0.21.0", Key: "ssh-ed25519 tls-fixture"})
	}))
	defer server.Close()
	c := testConfig()
	c.HubURL = server.URL
	hub, _ := newHub(c)
	hub.token = "fixture"
	if _, err := hub.info(context.Background()); err == nil {
		t.Error("untrusted certificate accepted")
	}
	certificate := server.Certificate()
	if certificate == nil {
		t.Fatal("missing fixture certificate")
	}
	ca := filepath.Join(t.TempDir(), "ca.pem")
	_ = os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw}), 0600)
	transport, err := tlsConfig(ca)
	if err != nil {
		t.Fatal(err)
	}
	hub.client.Transport.(*http.Transport).TLSClientConfig = transport
	if _, err = hub.info(context.Background()); err != nil {
		t.Fatalf("trusted private CA rejected: %v", err)
	}
	wrong := x509.NewCertPool()
	hub.client.Transport.(*http.Transport).TLSClientConfig.RootCAs = wrong
	hub.client.CloseIdleConnections()
	if _, err = hub.info(context.Background()); err == nil {
		t.Error("unrelated CA accepted")
	}
}

func TestAuthenticationErrorsDoNotEchoSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"message":"secret-do-not-log"}`))
	}))
	defer server.Close()
	c := testConfig()
	c.HubURL = server.URL
	hub, _ := newHub(c)
	_, err := hub.info(context.Background())
	if err == nil || strings.Contains(err.Error(), c.HubPassword) {
		t.Fatalf("unsafe authentication diagnostic: %v", err)
	}
}

func TestRevokedSessionFallsBackToPasswordLogin(t *testing.T) {
	var logins, refreshes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/collections/users/auth-refresh":
			refreshes.Add(1)
			w.WriteHeader(http.StatusUnauthorized)
		case "/api/collections/users/auth-with-password":
			logins.Add(1)
			respond(w, map[string]string{"token": "replacement-session"})
		case "/api/beszel/info":
			if r.Header.Get("Authorization") != "replacement-session" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			respond(w, HubInfo{Version: "0.20.0", Key: "ssh-ed25519 fixture"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := testConfig()
	c.HubURL = server.URL
	hub, _ := newHub(c)
	hub.token = "revoked-session"
	if _, err := hub.info(context.Background()); err != nil {
		t.Fatal(err)
	}
	if logins.Load() != 1 || refreshes.Load() != 1 {
		t.Error("revoked session did not renew through one bounded refresh and login")
	}
}

func TestAuthenticationIgnoresUnneededMetadataChanges(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respond(w, map[string]any{"token": "valid-session", "mfaId": map[string]any{"format": "changed"}, "new_field": []any{false, nil}})
	}))
	defer server.Close()
	c := testConfig()
	c.HubURL = server.URL
	hub, _ := newHub(c)
	if err := hub.authenticate(context.Background(), false); err != nil || hub.token != "valid-session" {
		t.Fatal("unneeded auth metadata blocked a valid session")
	}
}

func TestInconsistentPageMetadataIsNotAuthoritative(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respond(w, map[string]any{"page": 1, "totalPages": 0, "totalItems": 1, "items": []any{map[string]string{"id": "sys000000000001", "status": "up"}}})
	}))
	defer server.Close()
	c := testConfig()
	c.HubURL = server.URL
	hub, _ := newHub(c)
	hub.token = "fixture"
	inventory, err := hub.systems(context.Background())
	if err != nil || inventory.Complete || len(inventory.Systems) != 1 {
		t.Fatal("contradictory pagination was authoritative or discarded valid rows")
	}
}
