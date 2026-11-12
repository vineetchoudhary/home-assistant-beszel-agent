package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHistoryCleanupTargetsOnlyOwnedRegisteredEntities(t *testing.T) {
	entries := []map[string]any{
		{"entity_id": "sensor.my_renamed_cpu", "unique_id": "owned_cpu", "platform": "mqtt", "disabled_by": nil},
		{"entity_id": "binary_sensor.my_disk_health", "unique_id": "owned_disk", "platform": "mqtt", "disabled_by": "user"},
		{"entity_id": "sensor.unrelated", "unique_id": "not_owned", "platform": "mqtt"},
		{"entity_id": "sensor.other_platform", "unique_id": "owned_cpu", "platform": "another"},
		{"entity_id": "all", "unique_id": "owned_all", "platform": "mqtt"},
		{"entity_id": "sensor.*", "unique_id": "owned_glob", "platform": "mqtt"},
	}
	for _, days := range []int{2, 5} {
		address, requests := registryServer(t, entries, "")
		count, err := purgeRegisteredEntityHistory(context.Background(), address, "secret-test-token", map[string]bool{"owned_cpu": true, "owned_disk": true, "owned_all": true, "owned_glob": true}, days)
		if err != nil || count != 2 {
			t.Fatalf("purge %d days: %d targets, %v", days, count, err)
		}
		calls := <-requests
		if len(calls) != 1 || calls[0]["domain"] != "recorder" || calls[0]["service"] != "purge_entities" {
			t.Fatal("history cleanup used an unexpected action or modified entity settings")
		}
		data := calls[0]["service_data"].(map[string]any)
		ids := data["entity_id"].([]any)
		if len(data) != 2 || data["keep_days"] != float64(days) || len(ids) != 2 || ids[0] != "binary_sensor.my_disk_health" || ids[1] != "sensor.my_renamed_cpu" {
			t.Fatal("cleanup did not use explicit owned entity IDs and the requested keep_days")
		}
	}
}

func TestHistoryCleanupNeverPurgesWithEmptyScopeOrZeroDays(t *testing.T) {
	address, requests := registryServer(t, []map[string]any{{"entity_id": "sensor.other", "unique_id": "other", "platform": "mqtt"}}, "")
	count, err := purgeRegisteredEntityHistory(context.Background(), address, "secret-test-token", map[string]bool{}, 2)
	if err != nil || count != 0 || len(<-requests) != 0 {
		t.Fatal("cleanup ran without owned registered entities")
	}
	for _, days := range []int{-1, 0, 366} {
		if _, err := purgeRegisteredEntityHistory(context.Background(), "invalid", "", nil, days); err == nil {
			t.Fatal("invalid retention could reach a destructive API request")
		}
	}
	address, requests = registryServer(t, []map[string]any{{"entity_id": "sensor.owned", "unique_id": "owned", "platform": "mqtt"}}, "call_service")
	if _, err := purgeRegisteredEntityHistory(context.Background(), address, "secret-test-token", map[string]bool{"owned": true}, 5); err == nil || strings.Contains(err.Error(), "secret-test-token") {
		t.Fatal("cleanup API error was ignored or leaked credentials")
	}
	<-requests
}

func TestHistoryRetentionRespectsSystemAndMetricFilters(t *testing.T) {
	_, b, sample := expandedFixture(t)
	b.systemMetrics(sample)
	s, _ := telemetryService(t, b)
	if err := s.publishTelemetry(b.system, b); err != nil {
		t.Fatal(err)
	}
	core := metricKey("cpu", "0", "usage")
	memory := metricKey("memory", "", "total")
	otherSystem := "sys000000000099"
	s.state.Topics[s.state.discoveryTopic("sensor", otherSystem, core)] = ownedEntity{System: otherSystem, Metric: core, Family: "cpu", Definition: &entityDefinition{Domain: "sensor"}}
	entries := []map[string]any{}
	for _, entry := range []struct{ system, metric string }{{b.system.ID, core}, {b.system.ID, memory}, {otherSystem, core}} {
		entries = append(entries, map[string]any{"entity_id": "sensor." + entry.system + "_" + entry.metric, "unique_id": s.state.Namespace + "_" + entry.system + "_" + entry.metric, "platform": "mqtt"})
	}
	address, requests := registryServer(t, entries, "")
	t.Setenv("BESZEL_BRIDGE_SUPERVISOR_URL", "http"+strings.TrimSuffix(strings.TrimPrefix(address, "ws"), "/core/websocket"))
	t.Setenv("SUPERVISOR_TOKEN", "secret-test-token")
	s.cfg.HistoryRetentionDays = 7
	s.cfg.Metrics = []string{"cpu"}
	s.cfg.ExcludeSystems = []string{otherSystem}
	s.applyHistoryRetention(context.Background())
	calls := <-requests
	if len(calls) != 1 || calls[0]["type"] != "call_service" || s.historyPurgeWarning != "" {
		t.Fatal("filtered history retention did not request one cleanup")
	}
	data := calls[0]["service_data"].(map[string]any)
	ids := data["entity_id"].([]any)
	if len(ids) != 1 || ids[0] != "sensor."+b.system.ID+"_"+core || data["keep_days"] != float64(7) {
		t.Fatal("history retention included an excluded system or unselected metric")
	}
}

func TestHistoryRetentionRetriesWhenScheduleCannotBeSaved(t *testing.T) {
	_, b, sample := expandedFixture(t)
	b.systemMetrics(sample)
	s, _ := telemetryService(t, b)
	if err := s.publishTelemetry(b.system, b); err != nil {
		t.Fatal(err)
	}
	entries := []map[string]any{{"entity_id": "sensor.my_cpu", "unique_id": s.state.Namespace + "_" + b.system.ID + "_" + metricKey("cpu", "0", "usage"), "platform": "mqtt"}}
	address, requests := registryServer(t, entries, "")
	t.Setenv("BESZEL_BRIDGE_SUPERVISOR_URL", "http"+strings.TrimSuffix(strings.TrimPrefix(address, "ws"), "/core/websocket"))
	t.Setenv("SUPERVISOR_TOKEN", "secret-test-token")
	s.cfg.HistoryRetentionDays = 7
	previous := b.now.Add(-48 * time.Hour)
	s.state.HistoryPurgeAt, s.state.HistoryKeepDays = previous, 14
	s.dir = filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(s.dir, []byte("block state writes"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.applyHistoryRetention(context.Background())
	if len(<-requests) != 1 || !s.state.HistoryPurgeAt.Equal(previous) || s.state.HistoryKeepDays != 14 || s.historyPurgeWarning == "" {
		t.Fatal("failed persistence did not restore the previous cleanup schedule")
	}
	s.dir = t.TempDir()
	s.applyHistoryRetention(context.Background())
	if len(<-requests) != 1 || !s.state.HistoryPurgeAt.Equal(b.now) || s.state.HistoryKeepDays != 7 || s.historyPurgeWarning != "" {
		t.Fatal("cleanup did not retry after schedule persistence recovered")
	}
}

func TestHistoryRetentionSchedulePersistsAndRetries(t *testing.T) {
	_, b, sample := expandedFixture(t)
	b.systemMetrics(sample)
	s, _ := telemetryService(t, b)
	if err := s.publishTelemetry(b.system, b); err != nil {
		t.Fatal(err)
	}
	key := metricKey("cpu", "0", "usage")
	entries := []map[string]any{{"entity_id": "sensor.my_cpu", "unique_id": s.state.Namespace + "_" + b.system.ID + "_" + key, "platform": "mqtt"}}
	address, requests := registryServer(t, entries, "")
	base := "http" + strings.TrimSuffix(strings.TrimPrefix(address, "ws"), "/core/websocket")
	t.Setenv("BESZEL_BRIDGE_SUPERVISOR_URL", base)
	t.Setenv("SUPERVISOR_TOKEN", "secret-test-token")
	s.applyHistoryRetention(context.Background())
	if !s.state.HistoryPurgeAt.IsZero() {
		t.Fatal("default retention changed history")
	}
	s.cfg.HistoryRetentionDays = 5
	s.applyHistoryRetention(context.Background())
	if len(<-requests) != 1 || !s.state.HistoryPurgeAt.Equal(b.now) || s.state.HistoryKeepDays != 5 {
		t.Fatal("initial retention cleanup was not recorded")
	}
	persisted, err := loadState(s.dir)
	if err != nil || !persisted.HistoryPurgeAt.Equal(b.now) || persisted.HistoryKeepDays != 5 {
		t.Fatal("cleanup schedule was not persisted")
	}
	s.state = persisted
	s.now = func() time.Time { return b.now.Add(time.Hour) }
	s.applyHistoryRetention(context.Background())
	select {
	case <-requests:
		t.Fatal("cleanup repeated within 24 hours after a restart")
	default:
	}
	s.cfg.HistoryRetentionDays = 2
	s.applyHistoryRetention(context.Background())
	if len(<-requests) != 1 || s.state.HistoryKeepDays != 2 {
		t.Fatal("changed retention was not applied immediately")
	}
	last := s.state.HistoryPurgeAt
	s.now = func() time.Time { return b.now.Add(26 * time.Hour) }
	failingAddress, failures := registryServer(t, entries, "call_service")
	t.Setenv("BESZEL_BRIDGE_SUPERVISOR_URL", "http"+strings.TrimSuffix(strings.TrimPrefix(failingAddress, "ws"), "/core/websocket"))
	s.applyHistoryRetention(context.Background())
	<-failures
	if !s.state.HistoryPurgeAt.Equal(last) || s.historyPurgeWarning == "" {
		t.Fatal("failed cleanup advanced its schedule or lost its diagnostic")
	}
	t.Setenv("BESZEL_BRIDGE_SUPERVISOR_URL", base)
	s.applyHistoryRetention(context.Background())
	if len(<-requests) != 1 || !s.state.HistoryPurgeAt.Equal(s.now()) || s.historyPurgeWarning != "" {
		t.Fatal("failed cleanup did not recover on the next poll")
	}
	s.cfg.HistoryRetentionDays = 0
	s.now = func() time.Time { return b.now.Add(52 * time.Hour) }
	s.applyHistoryRetention(context.Background())
	select {
	case <-requests:
		t.Fatal("turning retention off still purged history")
	default:
	}
}
