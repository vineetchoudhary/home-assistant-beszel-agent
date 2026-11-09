package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sampleObject(t *testing.T, id string) map[string]any {
	t.Helper()
	body := fixture[page[json.RawMessage]](t, "0.21.0", "system_stats.json").Items[0]
	var record map[string]any
	if err := json.Unmarshal(body, &record); err != nil {
		t.Fatal(err)
	}
	record["system"] = id
	return record
}

func decodeSampleObject(t *testing.T, record map[string]any) Sample {
	t.Helper()
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var sample Sample
	if err = json.Unmarshal(data, &sample); err != nil {
		t.Fatal(err)
	}
	return sample
}

func TestMalformedFieldsKeepIndependentReadings(t *testing.T) {
	tests := []struct {
		name    string
		change  func(map[string]any)
		missing []string
		want    map[string]string
		issue   bool
	}{
		{"missing CPU", func(s map[string]any) { delete(s, "cpu") }, []string{"cpu_usage"}, nil, true},
		{"CPU null", func(s map[string]any) { s["cpu"] = nil }, []string{"cpu_usage"}, nil, true},
		{"CPU object", func(s map[string]any) { s["cpu"] = map[string]string{"secret": "do-not-echo"} }, []string{"cpu_usage"}, nil, true},
		{"memory text", func(s map[string]any) { s["mp"] = "do-not-echo" }, []string{"memory_usage"}, nil, true},
		{"disk out of range", func(s map[string]any) { s["dp"] = 101 }, []string{"disk_usage"}, nil, true},
		{"overflow", func(s map[string]any) { s["cpu"] = json.RawMessage(`1e400`) }, []string{"cpu_usage"}, nil, true},
		{"short bandwidth", func(s map[string]any) { s["b"] = []any{11} }, []string{"network_receive"}, map[string]string{"network_transmit": "11"}, true},
		{"one bandwidth component", func(s map[string]any) { s["b"] = []any{"do-not-echo", 23} }, []string{"network_transmit"}, map[string]string{"network_receive": "23"}, true},
		{"additional bandwidth fields", func(s map[string]any) { s["b"] = []any{11, 23, "new-format"} }, nil, map[string]string{"network_transmit": "11", "network_receive": "23"}, false},
		{"one malformed interface", func(s map[string]any) {
			s["ni"] = map[string]any{"first": []any{11, 23}, "second": []any{"do-not-echo", 7}}
		}, []string{"network_transmit"}, map[string]string{"network_receive": "30"}, true},
		{"one malformed temperature", func(s map[string]any) { s["t"] = map[string]any{"valid": 0, "invalid": "do-not-echo"} }, nil, map[string]string{"temperature": "0"}, true},
		{"unknown fields", func(s map[string]any) { s["new_metric"] = map[string]any{"arbitrary": []any{nil, true, "future"}} }, nil, nil, false},
		{"optional nulls", func(s map[string]any) { s["b"] = nil; s["ni"] = nil; s["t"] = nil }, []string{"network_receive", "network_transmit", "temperature"}, nil, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			system := fixture[page[System]](t, "0.21.0", "systems.json").Items[0]
			record := sampleObject(t, system.ID)
			test.change(record["stats"].(map[string]any))
			sample := decodeSampleObject(t, record)
			values, created, err := metricValues(system, &sample)
			if (err != nil) != test.issue {
				t.Fatalf("unexpected issue: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "do-not-echo") {
				t.Fatal("diagnostic exposed raw API value")
			}
			if created.IsZero() {
				t.Fatal("one bad field invalidated source time")
			}
			expected := map[string]string{"cpu_usage": "20", "memory_usage": "11.1", "disk_usage": "5.21", "last_sample": created.UTC().Format(time.RFC3339Nano)}
			for _, key := range test.missing {
				delete(expected, key)
				if _, exists := values[key]; exists {
					t.Errorf("invalid %s was published", key)
				}
			}
			for key, value := range test.want {
				expected[key] = value
			}
			for key, value := range expected {
				if values[key] != value {
					t.Errorf("%s: got %q, want %q", key, values[key], value)
				}
			}
		})
	}
}

func TestInvalidTimestampNeverMakesOtherFieldsFresh(t *testing.T) {
	system := fixture[page[System]](t, "0.21.0", "systems.json").Items[0]
	record := sampleObject(t, system.ID)
	for _, invalid := range []any{nil, "do-not-echo", true, map[string]string{"format": "changed"}} {
		record["created"] = invalid
		sample := decodeSampleObject(t, record)
		values, created, err := metricValues(system, &sample)
		if err == nil || len(values) != 0 || !created.IsZero() {
			t.Fatal("invalid source time fabricated fresh readings")
		}
	}
}

func TestSystemMetadataAndVersionAreIndependent(t *testing.T) {
	for _, version := range []string{`"0.21.1"`, `"0.99.0"`, `"3.0.0-beta.1"`, `null`, `{"format":"new"}`} {
		var info HubInfo
		if err := json.Unmarshal([]byte(`{"key":"ssh-ed25519 stable","v":`+version+`}`), &info); err != nil || info.Key != "ssh-ed25519 stable" {
			t.Fatal("version metadata invalidated identity")
		}
	}
	var system System
	if err := json.Unmarshal([]byte(`{"id":"sys000000000001","name":false,"status":"up","info":{"u":"do-not-echo","extra":{}}}`), &system); err != nil {
		t.Fatal(err)
	}
	if system.Name != system.ID || system.Info.Uptime != nil || len(system.Issues) == 0 {
		t.Fatal("malformed metadata was not isolated")
	}
}

type resilienceLab struct {
	service         *service
	pub             *recordingPublisher
	systems         []System
	records         map[string]any
	rawInventory    any
	failedSample    string
	malformedSample string
	now             time.Time
}

func newResilienceLab(t *testing.T) *resilienceLab {
	t.Helper()
	lab := &resilienceLab{records: map[string]any{}}
	first := fixture[page[System]](t, "0.21.0", "systems.json").Items[0]
	second := first
	second.ID = "sys000000000002"
	second.Name = "Second server"
	lab.systems = []System{first, second}
	for _, system := range lab.systems {
		lab.records[system.ID] = sampleObject(t, system.ID)
	}
	source := lab.records[first.ID].(map[string]any)["created"].(string)
	lab.now, _ = sampleTime(source)
	lab.now = lab.now.Add(time.Second)
	info := HubInfo{Version: "0.99.0", Key: "ssh-ed25519 stable-resilience-test"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/info"):
			respond(w, info)
		case strings.HasSuffix(r.URL.Path, "/systems/records"):
			if lab.rawInventory != nil {
				respond(w, lab.rawInventory)
			} else {
				respond(w, page[System]{Page: 1, TotalPages: 1, TotalItems: len(lab.systems), Items: lab.systems})
			}
		case strings.HasSuffix(r.URL.Path, "/system_stats/records"):
			if strings.Contains(r.URL.Query().Get("filter"), "type='10m'") {
				respond(w, page[Sample]{Page: 1, Items: []Sample{}})
				return
			}
			for _, system := range []System{first, second} {
				if strings.Contains(r.URL.Query().Get("filter"), system.ID) {
					if lab.malformedSample == system.ID {
						_, _ = w.Write([]byte(`{"items":[`))
						return
					}
					if lab.failedSample == system.ID {
						w.WriteHeader(503)
						return
					}
					respond(w, map[string]any{"page": 1, "totalPages": 1, "totalItems": 1, "items": []any{lab.records[system.ID]}})
					return
				}
			}
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	c := testConfig()
	c.HubURL = server.URL
	hub, _ := newHub(c)
	hub.token = "fixture"
	state, _ := loadState(t.TempDir())
	_ = state.initialize(info, c.DiscoveryPrefix, "tcp://broker:1883")
	lab.pub = &recordingPublisher{}
	lab.service = &service{cfg: c, dir: t.TempDir(), hub: hub, state: state, pub: lab.pub, health: &health{}, now: func() time.Time { return lab.now }, published: map[string]string{}}
	if err := lab.service.sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	return lab
}

func (lab *resilienceLab) available(id, key string) string {
	return lab.pub.last(lab.service.state.hostRoot(id) + "/" + key + "/availability")
}

func TestOneBrokenServerOrReadingDoesNotAbortOtherServers(t *testing.T) {
	for _, mode := range []string{"cpu", "sample request", "sample JSON", "sample shape", "status", "uptime", "timestamp", "stats"} {
		t.Run(mode, func(t *testing.T) {
			lab := newResilienceLab(t)
			first := lab.systems[0].ID
			second := lab.systems[1].ID
			record := lab.records[first].(map[string]any)
			allMetricsOffline := false
			switch mode {
			case "cpu":
				record["stats"].(map[string]any)["cpu"] = "do-not-echo"
			case "sample request":
				lab.failedSample = first
				allMetricsOffline = true
			case "sample JSON":
				lab.malformedSample = first
				allMetricsOffline = true
			case "sample shape":
				lab.records[first] = false
				allMetricsOffline = true
			case "status":
				lab.systems[0].Status = "future-status"
			case "uptime":
				recordSystem := map[string]any{"id": first, "name": "First", "status": "up", "info": map[string]any{"u": "do-not-echo"}}
				lab.rawInventory = map[string]any{"page": 1, "totalPages": 1, "totalItems": 2, "items": []any{recordSystem, lab.systems[1]}}
			case "timestamp":
				record["created"] = "do-not-echo"
				allMetricsOffline = true
			case "stats":
				record["stats"] = false
				allMetricsOffline = true
			}
			if err := lab.service.sync(context.Background()); err != nil {
				t.Fatalf("one record stopped entire sync: %v", err)
			}
			if lab.available(second, "cpu_usage") != "online" || lab.available(second, "memory_usage") != "online" {
				t.Fatal("healthy server stopped")
			}
			if allMetricsOffline {
				if lab.available(first, "cpu_usage") != "offline" || lab.available(first, "memory_usage") != "offline" {
					t.Fatal("broken sample stayed available")
				}
			} else {
				if lab.available(first, "memory_usage") != "online" {
					t.Fatal("unrelated metric stopped")
				}
				if mode == "cpu" && lab.available(first, "cpu_usage") != "offline" {
					t.Fatal("bad CPU stayed available")
				}
				if mode == "status" && (lab.available(first, "system_status") != "offline" || lab.available(first, "connectivity") != "offline") {
					t.Fatal("unknown status fabricated connectivity")
				}
				if mode == "uptime" && lab.available(first, "uptime") != "offline" {
					t.Fatal("invalid uptime remained available")
				}
			}
			if !lab.service.health.status.HubConnected || lab.service.health.status.DegradedSystems != 1 {
				t.Fatal("partial failure became a global outage or was hidden")
			}
			lab.failedSample = ""
			lab.malformedSample = ""
			lab.rawInventory = nil
			lab.systems[0].Status = "up"
			lab.records[first] = sampleObject(t, first)
			if err := lab.service.sync(context.Background()); err != nil {
				t.Fatal(err)
			}
			if lab.available(first, "cpu_usage") != "online" || lab.service.health.status.DegradedSystems != 0 || lab.service.health.status.Error != "" {
				t.Fatal("readings failed to recover")
			}
		})
	}
}

func TestIncompleteInventoryNeverRemovesDiscovery(t *testing.T) {
	for _, broken := range []any{nil, false, []any{}, map[string]any{}, map[string]any{"items": nil}, map[string]any{"items": []any{map[string]any{"id": "bad"}}, "page": 1, "totalPages": 1, "totalItems": 1}, map[string]any{"items": []any{}, "page": 1, "totalPages": "changed", "totalItems": 0}} {
		lab := newResilienceLab(t)
		count := len(lab.service.state.Topics)
		lab.systems = []System{}
		if err := lab.service.sync(context.Background()); err != nil {
			t.Fatal(err)
		}
		if broken == nil {
			lab.rawInventory = map[string]any{"items": nil}
		} else {
			lab.rawInventory = broken
		}
		lab.now = lab.now.Add(6 * time.Minute)
		lab.service.poll(context.Background())
		if len(lab.service.state.Topics) != count || len(lab.service.state.MissingSince) != 0 {
			t.Fatal("incomplete inventory deleted devices or counted toward removal grace")
		}
		lab.rawInventory = nil
		if err := lab.service.sync(context.Background()); err != nil {
			t.Fatal(err)
		}
		if len(lab.service.state.Topics) != count {
			t.Fatal("recovery reused an unsafe removal timer")
		}
	}
}

func TestMalformedInventoryRowDoesNotHideHealthyServer(t *testing.T) {
	lab := newResilienceLab(t)
	count := len(lab.service.state.Topics)
	lab.rawInventory = map[string]any{"page": 1, "totalPages": 1, "totalItems": 2, "items": []any{false, lab.systems[1]}}
	if err := lab.service.sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if lab.available(lab.systems[1].ID, "cpu_usage") != "online" || lab.available(lab.systems[0].ID, "cpu_usage") != "offline" {
		t.Fatal("record isolation failed")
	}
	if !lab.service.health.status.InventoryIncomplete || len(lab.service.state.Topics) != count {
		t.Fatal("partial inventory was treated as authoritative")
	}
}

type failingPublisher struct{}

func (failingPublisher) publish(string, any, bool) error { return errors.New("MQTT unavailable") }
func TestPublisherFailureIsNotHiddenAsADataIssue(t *testing.T) {
	lab := newResilienceLab(t)
	lab.service.pub = failingPublisher{}
	if err := lab.service.sync(context.Background()); err == nil {
		t.Fatal("MQTT failure was hidden")
	}
}

func FuzzIndependentJSONDecoding(f *testing.F) {
	f.Add([]byte(`{"stats":{"cpu":20,"mp":10,"dp":5},"created":"2026-11-01T00:00:00Z"}`))
	f.Add([]byte(`{"stats":{"cpu":"changed","ni":{"eth0":[null,0]},"t":{"cpu":{}}}}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`[]`))
	f.Fuzz(func(t *testing.T, data []byte) {
		var sample Sample
		if json.Unmarshal(data, &sample) == nil {
			values, _, _ := metricValues(System{}, &sample)
			for _, value := range values {
				if strings.Contains(value, "NaN") || strings.Contains(value, "Inf") {
					t.Fatal("nonfinite value escaped validation")
				}
			}
		}
		var system System
		_ = json.Unmarshal(data, &system)
		var info HubInfo
		_ = json.Unmarshal(data, &info)
		_, _ = decodeRecordsPage(data)
	})
}

func TestNonFiniteMetricsAreUnavailable(t *testing.T) {
	system := fixture[page[System]](t, "0.21.0", "systems.json").Items[0]
	sample := fixture[page[Sample]](t, "0.21.0", "system_stats.json").Items[0]
	invalid := math.NaN()
	sample.Stats.CPU = &invalid
	values, _, err := metricValues(system, &sample)
	if err == nil || values["memory_usage"] != "11.1" {
		t.Fatal("nonfinite metric invalidated good fields")
	}
	if _, exists := values["cpu_usage"]; exists {
		t.Fatal("NaN was published")
	}
}

func TestJSONNullRecordsAreNotAnEmptyInventory(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "0.21.0", "systems.json"))
	if err != nil {
		t.Fatal(err)
	}
	if result, err := decodeRecordsPage(data); err != nil || !result.Valid {
		t.Fatal("real inventory rejected")
	}
	if _, err = decodeRecordsPage([]byte(`{"page":1,"totalPages":0,"totalItems":0,"items":null}`)); err == nil {
		t.Fatal("null became an authoritative empty inventory")
	}
}

func TestNewMemoryFieldsDoNotDependOnDeprecatedGBFields(t *testing.T) {
	system := fixture[page[System]](t, "0.21.1", "systems.json").Items[0]
	page := fixture[page[map[string]any]](t, "0.21.1", "system_stats.json")
	record := page.Items[0]
	stats := record["stats"].(map[string]any)
	if stats["mem"] == nil {
		t.Fatal("0.21.1 fixture does not contain new byte-based memory fields")
	}
	for _, key := range []string{"m", "mm", "mu", "mb", "mz", "s", "su"} {
		delete(stats, key)
	}
	sample := decodeSampleObject(t, record)
	values, _, err := metricValues(system, &sample)
	if err != nil || values["memory_usage"] != "11.13" {
		t.Fatalf("removing deprecated GB fields broke memory percentage: %v", err)
	}
}

func TestRepeatedInvalidInventoryRowsDoNotFloodDiagnostics(t *testing.T) {
	lab := newResilienceLab(t)
	records := []any{lab.systems[1]}
	for i := 0; i < 200; i++ {
		records = append(records, false)
	}
	lab.rawInventory = map[string]any{"page": 1, "totalPages": 1, "totalItems": len(records), "items": records}
	if err := lab.service.sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	message := lab.service.warnings["inventory"]
	if len(message) > 300 || strings.Count(message, "invalid or duplicate system identifier") != 1 {
		t.Fatal("repeated malformed rows flooded diagnostics")
	}
}
