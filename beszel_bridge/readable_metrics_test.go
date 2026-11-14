package main

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestHumanByteFormatting(t *testing.T) {
	for _, test := range []struct{ value, unit, want string }{
		{"0", "B", "0 B"},
		{"1", "B", "1 B"},
		{"999.994", "B", "999.99 B"},
		{"999.996", "B", "1 KB"},
		{"1000", "B", "1 KB"},
		{"1024", "B", "1.02 KB"},
		{"1234567", "B", "1.23 MB"},
		{"1073741824", "B", "1.07 GB"},
		{"1000000000000", "B", "1 TB"},
		{"1000000000000000", "B", "1 PB"},
		{"18446744073709551615", "B", "18.45 EB"},
		{"1e21", "B", "1 ZB"},
		{"1e24", "B", "1 YB"},
		{"0.125", "B/s", "0.13 B/s"},
		{"125000000", "B/s", "125 MB/s"},
		{"999996", "B/s", "1 MB/s"},
		{"-1", "B", ""},
		{"NaN", "B", ""},
		{"+Inf", "B", ""},
		{"1e999", "B", ""},
		{"changed", "B", ""},
		{"", "B", ""},
		{"1000", "%", ""},
	} {
		t.Run(test.value+test.unit, func(t *testing.T) {
			got, ok := humanBytes(test.value, test.unit)
			if got != test.want || ok != (test.want != "") {
				t.Fatalf("got %q (%t), want %q", got, ok, test.want)
			}
		})
	}
}

func TestReadableCompanionsAcrossByteMetricFamilies(t *testing.T) {
	data, b, sample := expandedFixture(t)
	b.systemMetrics(sample)
	var peak Sample
	if err := json.Unmarshal(data["peak"], &peak); err != nil {
		t.Fatal(err)
	}
	b.peakMetrics(&peak)
	b.smartMetrics(expandedRecords(data, "smart_devices"))
	b.containerMetrics(expandedRecords(data, "containers"), expandedRecords(data, "container_stats"))
	b.serviceMetrics(expandedRecords(data, "systemd_services"))
	b.poolMetrics(expandedRecords(data, "zfs_pools"), sample)
	for key, want := range map[string]string{
		"memory_total_readable": "8.59 GB", "memory_free_readable": "6.44 GB",
		"memory_used_peak_10m_readable": "3.22 GB", "swap_used_readable": "0 B",
		"disk_capacity_readable": "107.37 GB", "filesystems_combined_capacity_readable": "1.07 TB",
		"network_receive_readable": "200 B/s", "network_transmit_readable": "100 B/s",
		metricKey("disk_io", "root", "read_total_readable"):         "9.01 PB",
		metricKey("disk_io", "root", "write_total_readable"):        "18.45 EB",
		metricKey("gpus", "gpu0", "memory_used_readable"):           "536.87 MB",
		metricKey("smart", "smt000000000001", "capacity_readable"):  "1 TB",
		metricKey("containers", "abcdef123456", "memory_readable"):  "134.22 MB",
		metricKey("services", "svc000000000001", "memory_readable"): "12.35 MB",
	} {
		if r, exists := b.readings[key]; !exists || r.Value != want {
			t.Errorf("%s: got %q (%t), want %q", key, r.Value, exists, want)
		}
	}
	families := map[string]bool{}
	for key, raw := range b.readings {
		if raw.Definition.Unit != "B" && raw.Definition.Unit != "B/s" {
			continue
		}
		families[raw.Family] = true
		companion, exists := b.readings[key+"_readable"]
		if !exists || companion.Definition.Name != raw.Definition.Name+" readable" {
			t.Fatalf("missing readable suffix for %s", key)
		}
		if companion.Component != raw.Component || companion.Family != raw.Family || !companion.Source.Equal(raw.Source) || companion.Snapshot != raw.Snapshot || companion.Definition.Expiry != raw.Definition.Expiry {
			t.Fatalf("%s lost source identity or freshness", key)
		}
		if !companion.Definition.Enabled || companion.Definition.Domain != "sensor" || companion.Definition.Unit != "" || companion.Definition.DeviceClass != "" || companion.Definition.StateClass != "" {
			t.Fatalf("%s must be enabled text without numeric statistics", companion.Key)
		}
		companion.Attributes["companion_only"] = true
		if raw.Attributes["companion_only"] != nil {
			t.Fatal("readable and raw attributes share mutable state")
		}
	}
	for _, family := range []string{"memory", "swap", "filesystems", "disk_io", "network", "gpus", "smart", "containers", "services", "storage_pools"} {
		if !families[family] {
			t.Fatalf("fixture did not cover %s byte metrics", family)
		}
	}
	// Exact raw integers and their statistics classes are still published.
	raw := requireValue(t, b, "disk_io", "root", "write_total", "18446744073709550000")
	if raw.Definition.Unit != "B" || raw.Definition.StateClass != "total_increasing" || raw.Definition.Enabled {
		t.Fatal("readable companion changed the original counter definition")
	}
}

func TestReadableCompanionFiltering(t *testing.T) {
	for _, selector := range []string{"memory", "network", "network_receive", "cpu_usage"} {
		t.Run(selector, func(t *testing.T) {
			_, b, sample := expandedFixture(t)
			b.cfg.Metrics = []string{selector}
			b.systemMetrics(sample)
			_, memory := b.readings["memory_total_readable"]
			_, receive := b.readings["network_receive_readable"]
			_, transmit := b.readings["network_transmit_readable"]
			if memory != (selector == "memory") || receive != (selector == "network" || selector == "network_receive") || transmit != (selector == "network") {
				t.Fatal("readable companions ignored narrow or family selection")
			}
			for key, r := range b.readings {
				if r.Family != "diagnostics" && !b.cfg.enabled(r.Family) {
					t.Fatalf("unselected metric %s was published", key)
				}
			}
		})
	}
}

func TestReadableDiscoveryAndAvailability(t *testing.T) {
	for _, mode := range []string{"fresh", "stale", "down", "missing", "invalid timestamp", "zero"} {
		t.Run(mode, func(t *testing.T) {
			_, b, sample := expandedFixture(t)
			b.systemMetrics(sample)
			s, p := telemetryService(t, b)
			if err := s.publishTelemetry(b.system, b); err != nil {
				t.Fatal(err)
			}
			root := s.state.hostRoot(b.system.ID)
			var discovery map[string]any
			if err := json.Unmarshal([]byte(p.last(s.state.discoveryTopic("sensor", b.system.ID, "memory_total_readable"))), &discovery); err != nil {
				t.Fatal(err)
			}
			if discovery["enabled_by_default"] != true || discovery["unit_of_measurement"] != nil || discovery["state_class"] != nil || discovery["device_class"] != nil {
				t.Fatal("readable discovery is not an enabled text sensor", discovery)
			}
			next := newTelemetry(b.cfg, b.system, b.now)
			switch mode {
			case "stale":
				next.now = next.now.Add(time.Duration(b.cfg.StaleAfter+1) * time.Second)
			case "down":
				next.system.Status = "down"
			case "missing":
				sample.RawStats["mem"] = json.RawMessage(`[null,2147483648]`)
			case "invalid timestamp":
				sample.Created = "changed"
			case "zero":
				sample.RawStats["mem"] = json.RawMessage(`[0,0,0,0]`)
			}
			next.systemMetrics(sample)
			p.messages = nil
			if err := s.publishTelemetry(next.system, next); err != nil {
				t.Fatal(err)
			}
			available := mode == "fresh" || mode == "zero"
			for _, key := range []string{"memory_total", "memory_total_readable"} {
				if (p.last(root+"/"+key+"/availability") == "online") != available {
					t.Fatalf("%s has incorrect availability", key)
				}
				if !available && p.last(root+"/"+key) != "" {
					t.Fatalf("%s republished an unavailable value", key)
				}
			}
			if mode == "zero" && (p.last(root+"/memory_total") != "0" || p.last(root+"/memory_total_readable") != "0 B") {
				t.Fatal("valid zero was lost")
			}
			for _, message := range p.messages {
				if message.topic == root+"/memory_total_readable" && message.retain {
					t.Fatal("retained readable value can replay stale data")
				}
			}
		})
	}
}

func TestPercentagePrecision(t *testing.T) {
	_, b, sample := expandedFixture(t)
	if err := json.Unmarshal([]byte(`{"created":"2026-11-01 12:00:00.000Z","stats":{"cpu":12.3456,"mp":33.3333333333,"dp":99.9999}}`), sample); err != nil {
		t.Fatal(err)
	}
	values, _, err := metricValues(b.system, sample)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"cpu_usage": "12.35", "memory_usage": "33.33", "disk_usage": "100"} {
		if values[key] != want {
			t.Errorf("%s: got %q, want %q", key, values[key], want)
		}
	}
	for _, test := range []struct {
		value float64
		want  string
	}{{0, "0"}, {math.Copysign(0, -1), "0"}, {0.001, "0"}, {0.005, "0.01"}, {12.3456, "12.35"}, {33.3333333333, "33.33"}, {99.9999, "100"}, {125.5555, "125.56"}, {20.5, "20.5"}, {1 << 53, "9007199254740992"}} {
		b.gauge("cpu", "", "test", "Test percent", test.value, true, "%", "", b.now, b.cfg.StaleAfter, false, nil)
		requireValue(t, b, "cpu", "", "test", test.want)
	}
	if got := roundedNumber(math.MaxFloat64); got != number(math.MaxFloat64) {
		t.Fatalf("rounding overflowed a finite percentage: %s", got)
	}
	// Precision of sizes, temperatures, load averages and other units is unchanged.
	b.gauge("memory", "", "test", "Test bytes", 123.456789, true, "B", "data_size", b.now, b.cfg.StaleAfter, false, nil)
	requireValue(t, b, "memory", "", "test", "123.456789")
	b.gauge("temperatures", "", "test", "Test temperature", 20.123456, true, "°C", "temperature", b.now, b.cfg.StaleAfter, false, nil)
	requireValue(t, b, "temperatures", "", "test", "20.123456")
}
