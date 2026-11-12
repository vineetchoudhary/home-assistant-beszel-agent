package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func registryServer(t *testing.T, entries []map[string]any, failure string) (string, <-chan []map[string]any) {
	t.Helper()
	requests := make(chan []map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/core/websocket" {
			t.Errorf("unexpected Core API path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		connection, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer connection.Close()
		updates := []map[string]any{}
		defer func() { requests <- updates }()
		_ = connection.WriteJSON(map[string]any{"type": "auth_required"})
		var message map[string]any
		if connection.ReadJSON(&message) != nil {
			return
		}
		if message["type"] != "auth" || message["access_token"] != "secret-test-token" {
			t.Error("missing Supervisor WebSocket authentication")
			return
		}
		if failure == "auth" {
			_ = connection.WriteJSON(map[string]any{"type": "auth_invalid", "message": "secret-test-token"})
			return
		}
		_ = connection.WriteJSON(map[string]any{"type": "auth_ok"})
		for {
			message = map[string]any{}
			if connection.ReadJSON(&message) != nil {
				return
			}
			reply := map[string]any{"id": message["id"], "type": "result", "success": true, "result": nil}
			switch message["type"] {
			case "config/entity_registry/list":
				reply["result"] = entries
			case "config/entity_registry/update":
				updates = append(updates, message)
			default:
				t.Errorf("unexpected API command: %v", message["type"])
				return
			}
			if message["type"] == failure {
				reply["success"] = false
				reply["error"] = map[string]any{"message": "secret-test-token"}
			}
			if err := connection.WriteJSON(reply); err != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)
	return "ws" + strings.TrimPrefix(server.URL, "http") + "/core/websocket", requests
}

func TestEnableExistingEntitiesInBulk(t *testing.T) {
	entries := []map[string]any{}
	owned := map[string]bool{}
	for i := range 512 {
		uniqueID := fmt.Sprintf("beszel_example_system_detail_%d", i)
		owned[uniqueID] = true
		entries = append(entries, map[string]any{"entity_id": fmt.Sprintf("sensor.custom_detail_%d", i), "unique_id": uniqueID, "platform": "mqtt", "disabled_by": "integration"})
	}
	for i, disabled := range []any{nil, "user", "device", "config_entry"} {
		uniqueID := fmt.Sprintf("owned_preserved_%d", i)
		owned[uniqueID] = true
		entries = append(entries, map[string]any{"entity_id": "sensor.preserved", "unique_id": uniqueID, "platform": "mqtt", "disabled_by": disabled})
	}
	entries = append(entries,
		map[string]any{"entity_id": "sensor.foreign", "unique_id": "not_owned", "platform": "mqtt", "disabled_by": "integration"},
		map[string]any{"entity_id": "sensor.other_integration", "unique_id": "owned_preserved_0", "platform": "another", "disabled_by": "integration"})
	address, requests := registryServer(t, entries, "")
	count, err := enableRegisteredEntities(context.Background(), address, "secret-test-token", owned)
	if err != nil || count != 512 {
		t.Fatalf("enabled %d entities: %v", count, err)
	}
	updates := <-requests
	if len(updates) != 512 {
		t.Fatalf("expected 512 registry updates, got %d", len(updates))
	}
	for i, update := range updates {
		if len(update) != 4 || update["entity_id"] != fmt.Sprintf("sensor.custom_detail_%d", i) || update["disabled_by"] != nil {
			t.Fatal("bulk enabling changed unrelated properties or entities")
		}
	}
}

func TestEnableEntitiesIsOptInAndRespectsFilters(t *testing.T) {
	_, b, sample := expandedFixture(t)
	b.systemMetrics(sample)
	s, _ := telemetryService(t, b)
	if err := s.publishTelemetry(b.system, b); err != nil {
		t.Fatal(err)
	}
	core := metricKey("cpu", "0", "usage")
	memory := metricKey("memory", "", "total")
	otherSystem := "sys000000000099"
	topic := s.state.discoveryTopic("sensor", otherSystem, core)
	s.state.Topics[topic] = ownedEntity{System: otherSystem, Metric: core, Family: "cpu", Definition: &entityDefinition{Domain: "sensor"}}
	entries := []map[string]any{}
	for _, entry := range []struct{ system, metric string }{{b.system.ID, core}, {b.system.ID, memory}, {otherSystem, core}} {
		entries = append(entries, map[string]any{"entity_id": "sensor." + entry.system + "_" + entry.metric, "unique_id": s.state.Namespace + "_" + entry.system + "_" + entry.metric, "platform": "mqtt", "disabled_by": "integration"})
	}
	address, requests := registryServer(t, entries, "")
	t.Setenv("BESZEL_BRIDGE_SUPERVISOR_URL", "http"+strings.TrimSuffix(strings.TrimPrefix(address, "ws"), "/core/websocket"))
	t.Setenv("SUPERVISOR_TOKEN", "secret-test-token")
	s.enableEntities(context.Background())
	select {
	case <-requests:
		t.Fatal("contacted Home Assistant with enable_all_entities disabled")
	default:
	}
	s.cfg.EnableAllEntities = true
	s.cfg.Metrics = []string{"cpu"}
	s.cfg.ExcludeSystems = []string{otherSystem}
	s.enableEntities(context.Background())
	updates := <-requests
	if len(updates) != 1 || updates[0]["entity_id"] != "sensor."+b.system.ID+"_"+core || s.entityEnableWarning != "" {
		t.Fatal("bulk enabling ignored metric/system selection or failed")
	}
}

func TestEnableEntitiesAPIFailuresDoNotLeakCredentials(t *testing.T) {
	for _, failure := range []string{"auth", "config/entity_registry/list", "config/entity_registry/update"} {
		t.Run(failure, func(t *testing.T) {
			address, requests := registryServer(t, []map[string]any{{"entity_id": "sensor.detail", "unique_id": "owned", "platform": "mqtt", "disabled_by": "integration"}}, failure)
			_, err := enableRegisteredEntities(context.Background(), address, "secret-test-token", map[string]bool{"owned": true})
			if err == nil || strings.Contains(err.Error(), "secret-test-token") {
				t.Fatalf("API failure missing or credential leaked: %v", err)
			}
			<-requests
		})
	}
	for _, address := range []string{"http://supervisor/core/websocket", "ws://secret-test-token@supervisor/core/websocket"} {
		_, err := enableRegisteredEntities(context.Background(), address, "secret-test-token", nil)
		if err == nil || strings.Contains(err.Error(), "secret-test-token") {
			t.Fatal("invalid endpoint accepted or leaked credentials")
		}
	}
	if _, err := enableRegisteredEntities(context.Background(), "ws://supervisor/core/websocket", "", nil); err == nil {
		t.Fatal("missing Supervisor token was accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := enableRegisteredEntities(ctx, "ws://127.0.0.1:1/core/websocket", "secret-test-token", nil); err == nil {
		t.Fatal("cancelled API operation succeeded")
	}
}
