package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSupervisorMQTTServiceAndManualMode(t *testing.T) {
	t.Setenv("SUPERVISOR_TOKEN", "private-supervisor-token")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/services/mqtt" || r.Header.Get("Authorization") != "Bearer private-supervisor-token" {
			t.Error("Supervisor request did not use the scoped service endpoint and token")
		}
		respond(w, map[string]any{"result": "ok", "data": map[string]any{
			"host": "core-mosquitto", "port": 8883, "username": "bridge", "password": "private-broker-password", "ssl": true,
		}})
	}))
	defer server.Close()
	t.Setenv("BESZEL_BRIDGE_SUPERVISOR_URL", server.URL)
	c := testConfig()
	c.MQTTSource = "supervisor"
	resolved, err := resolveMQTT(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if brokerAddress(resolved) != "ssl://core-mosquitto:8883" || resolved.MQTTUsername != "bridge" || resolved.MQTTPassword != "private-broker-password" {
		t.Error("Supervisor broker settings were not applied")
	}
	c.MQTTSource = "manual"
	manual, err := resolveMQTT(context.Background(), c)
	if err != nil || manual.MQTTHost != "broker" || calls != 1 {
		t.Error("manual mode unexpectedly depends on Supervisor")
	}
}

func TestSupervisorMQTTFailuresDoNotExposeSecrets(t *testing.T) {
	c := testConfig()
	c.MQTTSource = "supervisor"
	t.Setenv("SUPERVISOR_TOKEN", "")
	if _, err := resolveMQTT(context.Background(), c); err == nil {
		t.Error("missing Supervisor service token accepted")
	}
	t.Setenv("SUPERVISOR_TOKEN", "private-supervisor-token")
	for _, response := range []struct {
		name string
		code int
		body string
	}{
		{"denied", 403, "private-supervisor-token"},
		{"missing service", 200, `{"result":"error","message":"private-broker-password"}`},
		{"incomplete", 200, `{"result":"ok","data":{"host":"broker","port":0,"password":"private-broker-password"}}`},
		{"invalid JSON", 200, "private-broker-password"},
	} {
		t.Run(response.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(response.code)
				_, _ = w.Write([]byte(response.body))
			}))
			defer server.Close()
			t.Setenv("BESZEL_BRIDGE_SUPERVISOR_URL", server.URL)
			_, err := resolveMQTT(context.Background(), c)
			if err == nil || strings.Contains(err.Error(), "private-") {
				t.Error("failed Supervisor response accepted or exposed credentials")
			}
		})
	}
}
