package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func expandedFixture(t *testing.T) (map[string]json.RawMessage, *telemetry, *Sample) {
	t.Helper()
	data := fixture[map[string]json.RawMessage](t, "expanded", "monitoring.json")
	var system System
	var sample Sample
	if err := json.Unmarshal(data["system"], &system); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data["sample"], &sample); err != nil {
		t.Fatal(err)
	}
	now, _ := sampleTime(sample.Created)
	return data, newTelemetry(testConfig(), system, now.Add(time.Second)), &sample
}

func expandedRecords(data map[string]json.RawMessage, key string) collectionResult {
	result := collectionResult{Complete: true}
	for _, raw := range array(data[key]) {
		result.Rows = append(result.Rows, obj(raw))
	}
	return result
}

func requireValue(t *testing.T, b *telemetry, family, identity, metric, want string) reading {
	t.Helper()
	key := metricKey(family, identity, metric)
	if family == "uptime_human" {
		key = family
	}
	r, ok := b.readings[key]
	if !ok || r.Value != want {
		t.Fatalf("%s: got %q (present %t), want %q", key, r.Value, ok, want)
	}
	return r
}

func TestExpandedHardwareAndSourceUnits(t *testing.T) {
	data, b, sample := expandedFixture(t)
	b.systemMetrics(sample)
	requireValue(t, b, "uptime_human", "", "", "20 hours")
	requireValue(t, b, "filesystems", "", "combined_usage", "14")
	requireValue(t, b, "filesystems", "data", "usage", "10")
	requireValue(t, b, "filesystems", "root", "capacity", "107374182400")
	requireValue(t, b, "memory", "", "total", "8589934592")
	requireValue(t, b, "memory", "", "free", "6442450944")
	requireValue(t, b, "swap", "", "usage", "0")
	requireValue(t, b, "cpu", "0", "usage", "10")
	requireValue(t, b, "cpu", "", "idle", "87")
	requireValue(t, b, "cpu", "", "load_15m", "0.3")
	requireValue(t, b, "network", "eth0", "transmit_total", "9007199254740993")
	requireValue(t, b, "disk_io", "root", "write_total", "18446744073709550000")
	requireValue(t, b, "temperatures", "ambient", "temperature", "0")
	requireValue(t, b, "gpus", "gpu0", "power", "0")
	requireValue(t, b, "gpus", "gpu0", "memory_usage", "25")
	requireValue(t, b, "batteries", "BAT1", "level", "0")
	requireValue(t, b, "batteries", "", "state", "discharging")
	wifi := requireValue(t, b, "wifi", "wlan0", "signal", "-61")
	if wifi.Attributes["ssid"] != "Lab Wi-Fi" {
		t.Fatal("lost SSID metadata")
	}
	values, _, _ := metricValues(b.system, sample)
	if values["disk_usage"] != "55" {
		t.Fatal("existing primary usage changed")
	}
	var peak Sample
	_ = json.Unmarshal(data["peak"], &peak)
	b.peakMetrics(&peak)
	r := requireValue(t, b, "cpu", "", "peak_10m", "95")
	if r.Attributes["source_window"] != "10m" || r.Definition.Expiry < 1800 {
		t.Fatal("rollup peak lost its source window or expiry")
	}
	requireValue(t, b, "disk_io", "data", "read_rate_peak_10m", "1000")
}

func TestDiskAggregationInvalidMembersAndExclusions(t *testing.T) {
	for _, test := range []struct {
		name     string
		edit     func(rawObject)
		excluded bool
		want     string
	}{
		{"weighted", func(rawObject) {}, false, "14"},
		{"bad capacity", func(o rawObject) { o["efs"] = json.RawMessage(`{"data":{"d":"changed","du":90}}`) }, false, ""},
		{"null member", func(o rawObject) { o["efs"] = json.RawMessage(`{"data":null}`) }, false, ""},
		{"bad inventory", func(o rawObject) { o["efs"] = json.RawMessage(`[]`) }, false, ""},
		{"zero root", func(o rawObject) { o["d"] = json.RawMessage(`0`) }, false, ""},
		{"over capacity", func(o rawObject) { o["du"] = json.RawMessage(`101`) }, false, ""},
		{"excluded bad member", func(o rawObject) { o["efs"] = json.RawMessage(`{"data":{"d":"changed","du":90}}`) }, true, "50"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, b, sample := expandedFixture(t)
			test.edit(sample.RawStats)
			if test.excluded {
				b.cfg.DiskAggregateExclude = []diskExclusion{{System: b.system.ID, Filesystem: "data"}}
			}
			b.systemMetrics(sample)
			r, exists := b.readings[metricKey("filesystems", "", "combined_usage")]
			if (test.want != "") != exists || r.Value != test.want {
				t.Fatalf("aggregate: got %q (%t), want %q", r.Value, exists, test.want)
			}
		})
	}
}

func TestSMARTOverviewRawPrecisionAndMissingVersusZero(t *testing.T) {
	data, b, _ := expandedFixture(t)
	b.smartMetrics(expandedRecords(data, "smart_devices"))
	id := "smt000000000001"
	requireValue(t, b, "smart", id, "status", "PASSED")
	r := requireValue(t, b, "smart", id, "power_on_seconds", "72000")
	if r.Attributes["source_precision_seconds"] != 3600 {
		t.Fatal("conversion implied seconds precision")
	}
	requireValue(t, b, "smart", id, "power_on_human", "20 hours")
	requireValue(t, b, "smart", id, "cycles", "2")
	requireValue(t, b, "smart", id, "temperature", "32")
	requireValue(t, b, "smart", id, "updated_human", "10 hours ago")
	requireValue(t, b, "smart_attributes", id, "attribute_"+identityToken("id:199"), "9007199254740993")
	requireValue(t, b, "smart", "smt000000000002", "status", "UNKNOWN")
	for _, metric := range []string{"power_on_seconds", "cycles", "temperature"} {
		if _, ok := b.readings[metricKey("smart", "smt000000000002", metric)]; ok {
			t.Fatalf("invented %s from unsupported zero", metric)
		}
		requireValue(t, b, "smart", "smt000000000003", metric, "0")
	}
}

func TestDurationsAndMalformedNativeMemory(t *testing.T) {
	for _, test := range []struct {
		seconds uint64
		want    string
	}{{0, "0 seconds"}, {59, "59 seconds"}, {60, "1 min"}, {120, "2 mins"}, {3600, "1 hour"}, {72000, "20 hours"}, {86400, "1 day"}, {30 * 86400, "30 days"}, {60 * 86400, "2 months"}, {180 * 86400, "6 months"}, {365 * 86400, "1 year"}, {3650 * 86400, "10 years"}} {
		if got := humanDuration(test.seconds); got != test.want {
			t.Errorf("%d: %s, want %s", test.seconds, got, test.want)
		}
	}
	_, b, sample := expandedFixture(t)
	sample.RawStats["mem"] = json.RawMessage(`[8589934592,"changed",0,0]`)
	b.systemMetrics(sample)
	if _, exists := b.readings[metricKey("memory", "", "used")]; exists {
		t.Fatal("malformed native bytes fell back to legacy memory")
	}
	requireValue(t, b, "memory", "", "total", "8589934592")
}

func TestStoredInventoriesAndFailedProbe(t *testing.T) {
	data, b, sample := expandedFixture(t)
	b.containerMetrics(expandedRecords(data, "containers"), expandedRecords(data, "container_stats"))
	b.serviceMetrics(expandedRecords(data, "systemd_services"))
	b.poolMetrics(expandedRecords(data, "zfs_pools"), sample)
	b.alertMetrics(expandedRecords(data, "alerts"), expandedRecords(data, "alerts_history"))
	b.systemInfo(expandedRecords(data, "system_details"), sample)
	b.packageMetrics(collectionResult{Complete: true, Rows: []rawObject{obj(data["package_updates"])}})
	probes := map[string]collectionResult{"probe:net000000000001": expandedRecords(data, "network_monitor_stats")}
	b.monitorMetrics(expandedRecords(data, "network_monitors"), probes, "")
	requireValue(t, b, "containers", "abcdef123456", "health", "healthy")
	requireValue(t, b, "containers", "abcdef123456", "memory", "134217728")
	requireValue(t, b, "containers", "abcdef123456", "receive", "200")
	requireValue(t, b, "services", "svc000000000001", "state", "failed")
	requireValue(t, b, "services", "svc000000000001", "cpu", "0")
	requireValue(t, b, "storage_pools", "zfs000000000001", "usage", "10")
	requireValue(t, b, "storage_pools", "zfs000000000001", "scrub_progress", "10")
	requireValue(t, b, "storage_pools", "zfs000000000001/vdev/mirror-0", "write_errors", "0")
	requireValue(t, b, "network_monitors", "net000000000001", "packet_loss", "25")
	r := requireValue(t, b, "network_monitors", "net000000000001", "response_average", "2")
	if r.Definition.Expiry != 1800 {
		t.Fatal("long-interval probe used fast telemetry expiry")
	}
	requireValue(t, b, "network_monitors", "net000000000001", "certificate_days_remaining", "29")
	requireValue(t, b, "alerts", "", "active_count", "1")
	requireValue(t, b, "package_updates", "", "count_security", "1")
	if !requireValue(t, b, "alerts", "alt000000000001", "triggered", "ON").Snapshot {
		t.Fatal("unchanged alert incorrectly ages out")
	}
	failed := newTelemetry(b.cfg, b.system, b.now)
	probes["probe:net000000000001"].Rows[0]["success_count"] = json.RawMessage(`0`)
	failed.monitorMetrics(expandedRecords(data, "network_monitors"), probes, "")
	requireValue(t, failed, "network_monitors", "net000000000001", "packet_loss", "100")
	if _, ok := failed.readings[metricKey("network_monitors", "net000000000001", "response_average")]; ok {
		t.Fatal("failed probes fabricated zero latency")
	}
	packages := obj(data["package_updates"])
	delete(packages, "securityKnown")
	unknown := newTelemetry(b.cfg, b.system, b.now)
	unknown.systemMetrics(sample)
	unknown.packageMetrics(collectionResult{Complete: true, Rows: []rawObject{packages}})
	if _, ok := unknown.readings[metricKey("package_updates", "", "count_security")]; ok {
		t.Fatal("unknown security classification became zero or retained an old count")
	}
}

func telemetryService(t *testing.T, b *telemetry) (*service, *recordingPublisher) {
	t.Helper()
	state, _ := loadState(t.TempDir())
	_ = state.initialize(HubInfo{Key: "ssh-ed25519 expanded-fixture"}, "homeassistant", "tcp://broker:1883")
	p := &recordingPublisher{}
	return &service{cfg: b.cfg, state: state, dir: t.TempDir(), pub: p, published: map[string]string{}, now: func() time.Time { return b.now }}, p
}

func TestExpandedDiscoveryFreshnessRenameAndCleanup(t *testing.T) {
	data, b, sample := expandedFixture(t)
	b.systemMetrics(sample)
	b.smartMetrics(expandedRecords(data, "smart_devices"))
	s, p := telemetryService(t, b)
	if err := s.publishTelemetry(b.system, b); err != nil {
		t.Fatal(err)
	}
	key := metricKey("smart", "smt000000000001", "status")
	root := s.state.hostRoot(b.system.ID) + "/" + key
	if p.last(root+"/availability") != "online" {
		t.Fatal("10-hour-old SMART prematurely expired")
	}
	for _, entry := range []struct {
		key     string
		enabled bool
	}{{key, true}, {"uptime_human", true}, {metricKey("cpu", "0", "usage"), false}, {metricKey("filesystems", "data", "usage"), true}} {
		var config map[string]any
		_ = json.Unmarshal([]byte(p.last(s.state.discoveryTopic("sensor", b.system.ID, entry.key))), &config)
		if config["enabled_by_default"] != entry.enabled || config["json_attributes_topic"] == nil {
			t.Fatalf("wrong discovery defaults for %s", entry.key)
		}
	}
	var attrs rawObject
	_ = json.Unmarshal([]byte(p.last(root+"/attributes")), &attrs)
	if attrs.text("model") != "CT1000BX500SSD1" || attrs.text("device") != "/dev/sda" {
		t.Fatal("SMART metadata not published")
	}
	b.now = b.now.Add(49 * time.Hour)
	b.system.Name = "Renamed server"
	if err := s.publishTelemetry(b.system, b); err != nil {
		t.Fatal(err)
	}
	if p.last(root+"/availability") != "offline" {
		t.Fatal("stale SMART remains available")
	}
	if p.last(s.state.hostRoot(b.system.ID)+"/"+metricKey("smart", "smt000000000001", "last_updated")+"/availability") != "online" {
		t.Fatal("lost timestamp explaining old SMART data")
	}
	if !strings.Contains(p.last(s.state.discoveryTopic("sensor", b.system.ID, key)), "Renamed server") {
		t.Fatal("device rename failed")
	}
	for _, message := range p.messages {
		if (message.topic == root || message.topic == root+"/attributes") && message.retain {
			t.Fatal("retained live telemetry")
		}
	}
	if err := s.reconcile(map[string]bool{}, b.now, true); err != nil {
		t.Fatal(err)
	}
	if err := s.reconcile(map[string]bool{}, b.now.Add(301*time.Second), true); err != nil {
		t.Fatal(err)
	}
	if len(s.state.Topics) != 0 {
		t.Fatal("system cleanup left dynamic topics")
	}
}

func TestEnableAllEntitiesDiscovery(t *testing.T) {
	data, b, sample := expandedFixture(t)
	b.systemMetrics(sample)
	b.smartMetrics(expandedRecords(data, "smart_devices"))
	s, p := telemetryService(t, b)
	if err := s.publishTelemetry(b.system, b); err != nil {
		t.Fatal(err)
	}
	before := map[string]map[string]any{}
	disabled := 0
	for topic, entity := range s.state.Topics {
		var payload map[string]any
		if err := json.Unmarshal([]byte(p.last(topic)), &payload); err != nil {
			t.Fatal(err)
		}
		before[topic] = payload
		if !entity.Definition.Enabled {
			disabled++
		}
	}
	if disabled == 0 {
		t.Fatal("fixture must include detailed entities disabled by default")
	}
	for _, enabled := range []bool{true, false} {
		s.cfg.EnableAllEntities = enabled
		// Rediscover known entities even when the Hub has no current readings.
		missing := newTelemetry(s.cfg, b.system, b.now)
		if err := s.publishTelemetry(b.system, missing); err != nil {
			t.Fatal(err)
		}
		for topic, original := range before {
			var payload map[string]any
			if err := json.Unmarshal([]byte(p.last(topic)), &payload); err != nil {
				t.Fatal(err)
			}
			if payload["enabled_by_default"] != (enabled || original["enabled_by_default"] == true) {
				t.Fatalf("incorrect discovery default for %s with enable_all_entities=%v", topic, enabled)
			}
			for _, field := range []string{"unique_id", "default_entity_id", "name", "state_topic"} {
				if payload[field] != original[field] {
					t.Fatalf("%s changed for %s", field, topic)
				}
			}
		}
	}
}

func TestComponentRemovalRequiresConsecutiveCompleteInventories(t *testing.T) {
	data, b, _ := expandedFixture(t)
	b.smartMetrics(expandedRecords(data, "smart_devices"))
	s, p := telemetryService(t, b)
	if err := s.publishTelemetry(b.system, b); err != nil {
		t.Fatal(err)
	}
	initial := len(s.state.Topics)
	empty := func(complete bool, advance time.Duration) {
		t.Helper()
		next := newTelemetry(b.cfg, b.system, b.now.Add(advance))
		next.scope("smart", complete)
		next.scope("smart_attributes", complete)
		if err := s.publishTelemetry(b.system, next); err != nil {
			t.Fatal(err)
		}
	}
	empty(true, time.Minute)
	empty(false, 7*time.Minute)
	empty(true, 8*time.Minute)
	if len(s.state.Topics) != initial {
		t.Fatal("failed inventory counted toward component deletion")
	}
	empty(true, 14*time.Minute)
	if len(s.state.Topics) != 0 {
		t.Fatal("absent components not removed after grace")
	}
	for _, message := range p.messages {
		if strings.HasSuffix(message.topic, "/config") && message.value == "" && !message.retain {
			t.Fatal("cleanup did not clear retained discovery")
		}
	}
}

func TestExtendedSelectorsAndOldManifest(t *testing.T) {
	c := testConfig()
	c.Metrics = []string{"cpu", "smart_attributes", "uptime_human"}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	if !c.enabled("cpu_usage") || c.enabled("memory_usage") {
		t.Fatal("family selection broke legacy scalar filters")
	}
	c.Metrics = []string{"cpu_usage"}
	if c.enabled("cpu") {
		t.Fatal("scalar selection unexpectedly enabled the whole family")
	}
	dir := t.TempDir()
	old := `{"namespace":"beszel_old","topics":{"homeassistant/sensor/beszel_old_sys000000000001_cpu_usage/config":{"system":"sys000000000001","metric":"cpu_usage"}}}`
	if err := os.WriteFile(filepath.Join(dir, "bridge-state.json"), []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	state, err := loadState(dir)
	if err != nil || len(state.Topics) != 1 || state.ComponentMissing == nil {
		t.Fatal("old manifest failed additive migration", err)
	}
	for _, entity := range state.Topics {
		if entity.Definition != nil || entity.Metric != "cpu_usage" {
			t.Fatal("old entity identity changed")
		}
	}
}

func TestCollectorPaginationConcurrencyAndCache(t *testing.T) {
	var active, maximum, requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		n := active.Add(1)
		defer active.Add(-1)
		for {
			previous := maximum.Load()
			if n <= previous || maximum.CompareAndSwap(previous, n) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		if r.URL.Query().Get("page") == "2" {
			respond(w, map[string]any{"page": 2, "totalPages": 2, "totalItems": 2, "items": []any{map[string]any{"id": "second", "system": "sys000000000001"}}})
			return
		}
		respond(w, map[string]any{"page": 1, "totalPages": 2, "totalItems": 2, "items": []any{map[string]any{"id": "first", "system": "sys000000000001"}}})
	}))
	defer server.Close()
	c := testConfig()
	c.HubURL = server.URL
	hub, _ := newHub(c)
	hub.token = "fixture"
	now := time.Now()
	s := &service{hub: hub, now: func() time.Time { return now }}
	jobs := []collectionJob{}
	for i := 0; i < 12; i++ {
		jobs = append(jobs, collectionJob{key: strconv.Itoa(i), interval: time.Minute, fetch: func(ctx context.Context) collectionResult {
			return hub.records(ctx, "smart_devices", "sys000000000001")
		}})
	}
	results := s.fetchJobs(context.Background(), jobs)
	if maximum.Load() > 4 || maximum.Load() < 2 {
		t.Fatalf("request concurrency: %d", maximum.Load())
	}
	for _, result := range results {
		if !result.Complete || len(result.Rows) != 2 || result.Err != nil {
			t.Fatal("pagination incomplete", result.Err)
		}
	}
	count := requests.Load()
	_ = s.fetchJobs(context.Background(), jobs)
	if requests.Load() != count {
		t.Fatal("slow collections were polled too often")
	}
	now = now.Add(time.Minute)
	_ = s.fetchJobs(context.Background(), jobs)
	if requests.Load() != count*2 {
		t.Fatal("slow cache did not refresh")
	}
}

func TestExpandedLegacyVersionsAndIdleCapability(t *testing.T) {
	for _, version := range []string{"0.20.0", "0.21.0", "0.21.1", "0.21.2"} {
		t.Run(version, func(t *testing.T) {
			system := fixture[page[System]](t, version, "systems.json").Items[0]
			sample := fixture[page[Sample]](t, version, "system_stats.json").Items[0]
			now, _ := sampleTime(sample.Created)
			b := newTelemetry(testConfig(), system, now.Add(time.Second))
			b.systemMetrics(&sample)
			if _, ok := b.readings[metricKey("filesystems", "", "combined_usage")]; !ok {
				t.Fatal("legacy disk totals unavailable")
			}
			if _, ok := b.readings[metricKey("memory", "", "total")]; !ok {
				t.Fatal("legacy GiB or native byte memory unavailable")
			}
		})
	}
	_, b, sample := expandedFixture(t)
	b.systemMetrics(sample)
	next := newTelemetry(b.cfg, b.system, b.now)
	for key := range b.readings {
		next.known[key] = true
	}
	delete(sample.RawStats, "dio")
	gpus := obj(sample.RawStats["g"])
	gpu := obj(gpus["gpu0"])
	delete(gpu, "p")
	raw, _ := json.Marshal(gpu)
	gpus["gpu0"] = raw
	raw, _ = json.Marshal(gpus)
	sample.RawStats["g"] = raw
	next.systemMetrics(sample)
	requireValue(t, next, "disk_io", "root", "read_rate", "0")
	requireValue(t, next, "gpus", "gpu0", "power", "0")
	first := newTelemetry(b.cfg, b.system, b.now)
	first.systemMetrics(sample)
	if _, exists := first.readings[metricKey("gpus", "gpu0", "power")]; exists {
		t.Fatal("first observation invented omitted GPU power")
	}
}

func TestMalformedSampleAndMapsPreserveComponentDiscovery(t *testing.T) {
	_, b, sample := expandedFixture(t)
	b.systemMetrics(sample)
	for _, mutate := range []func(*Sample){
		func(sample *Sample) { sample.Created = "invalid" },
		func(sample *Sample) { sample.RawStats = rawObject{} },
		func(sample *Sample) { sample.RawStats["efs"] = json.RawMessage(`null`) },
	} {
		_, next, current := expandedFixture(t)
		mutate(current)
		next.systemMetrics(current)
		if next.complete["filesystems"] {
			t.Fatal("malformed source authorized component deletion")
		}
	}
	_, named, current := expandedFixture(t)
	current.RawStats["efs"] = json.RawMessage(`{"root":{"d":900,"du":90}}`)
	named.systemMetrics(current)
	requireValue(t, named, "filesystems", "", "combined_usage", "14")
	requireValue(t, named, "filesystems", "extra:root", "usage", "10")
}

func TestExpandedServiceCollectorsAndOptionalFailureIsolation(t *testing.T) {
	data, b, _ := expandedFixture(t)
	var failSMART atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/info") {
			respond(w, HubInfo{Key: "ssh-ed25519 expanded-fixture", Version: "0.21.1"})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/package-updates") {
			respond(w, data["package_updates"])
			return
		}
		parts := strings.Split(r.URL.Path, "/")
		collection := parts[len(parts)-2]
		if collection == "smart_devices" && failSMART.Load() {
			w.WriteHeader(503)
			return
		}
		var items []json.RawMessage
		switch collection {
		case "systems":
			items = []json.RawMessage{data["system"]}
		case "system_stats":
			key := "sample"
			if strings.Contains(r.URL.Query().Get("filter"), "type='10m'") {
				key = "peak"
			}
			items = []json.RawMessage{data[key]}
		default:
			items = array(data[collection])
		}
		if items == nil {
			items = []json.RawMessage{}
		}
		respond(w, map[string]any{"page": 1, "totalPages": 1, "totalItems": len(items), "items": items})
	}))
	defer server.Close()
	c := b.cfg
	c.HubURL = server.URL
	hub, _ := newHub(c)
	hub.token = "fixture"
	s, p := telemetryService(t, b)
	s.cfg = c
	s.hub = hub
	s.health = &health{}
	if err := s.sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !s.health.status.HubConnected || s.health.status.DegradedSystems != 0 {
		t.Fatal("complete telemetry sync degraded", s.health.status)
	}
	if p.last(s.state.hostRoot(b.system.ID)+"/filesystems_combined_usage") != "14" {
		t.Fatal("collector lost disk aggregation")
	}
	failSMART.Store(true)
	b.now = b.now.Add(61 * time.Second)
	if err := s.sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.health.status.DegradedSystems != 1 || !s.health.status.HubConnected {
		t.Fatal("optional failure not isolated")
	}
	root := s.state.hostRoot(b.system.ID)
	if p.last(root+"/cpu_usage/availability") != "online" || p.last(root+"/"+metricKey("smart", "smt000000000001", "status")+"/availability") != "offline" {
		t.Fatal("SMART failure affected wrong entities")
	}
}

func TestUptimeAndByteCountersCanRestart(t *testing.T) {
	_, b, sample := expandedFixture(t)
	b.systemMetrics(sample)
	s, p := telemetryService(t, b)
	if err := s.publishTelemetry(b.system, b); err != nil {
		t.Fatal(err)
	}
	previousTopic := s.state.discoveryTopic("sensor", b.system.ID, metricKey("disk_io", "root", "read_total"))
	var definition map[string]any
	_ = json.Unmarshal([]byte(p.last(previousTopic)), &definition)
	if definition["state_class"] != "total_increasing" {
		t.Fatal("byte counter cannot account for resets")
	}
	uptime := uint64(60)
	b.system.Info.Uptime = &uptime
	next := newTelemetry(b.cfg, b.system, b.now.Add(time.Second))
	sample.RawStats["diot"] = json.RawMessage(`[1,2]`)
	next.systemMetrics(sample)
	if err := s.publishTelemetry(next.system, next); err != nil {
		t.Fatal(err)
	}
	root := s.state.hostRoot(b.system.ID)
	if p.last(root+"/uptime_human") != "1 min" || p.last(root+"/"+metricKey("disk_io", "root", "read_total")) != "1" {
		t.Fatal("reset readings were rejected or extrapolated")
	}
	if p.last(previousTopic) == "" {
		t.Fatal("reset changed counter identity")
	}
}
