package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGroupEntitiesConfiguration(t *testing.T) {
	for _, value := range []any{nil, false, true, "true", 1} {
		data, _ := json.Marshal(testConfig())
		options := map[string]any{}
		_ = json.Unmarshal(data, &options)
		delete(options, "group_entities")
		if value != nil {
			options["group_entities"] = value
		}
		data, _ = json.Marshal(options)
		filename := filepath.Join(t.TempDir(), "options.json")
		if err := os.WriteFile(filename, data, 0600); err != nil {
			t.Fatal(err)
		}
		config, err := loadConfig(filename)
		valid := value == nil || value == false || value == true
		if (err == nil) != valid || config.GroupEntities != (value == true) {
			t.Fatalf("group_entities=%v: grouped=%v error=%v", value, config.GroupEntities, err)
		}
	}
}

func groupedFixture(t *testing.T) (map[string]json.RawMessage, *telemetry) {
	t.Helper()
	data, b, sample := expandedFixture(t)
	b.cfg.GroupEntities = true
	b.systemMetrics(sample)
	b.smartMetrics(expandedRecords(data, "smart_devices"))
	b.containerMetrics(expandedRecords(data, "containers"), expandedRecords(data, "container_stats"))
	b.serviceMetrics(expandedRecords(data, "systemd_services"))
	return data, b
}

func readingAttrs(t *testing.T, r reading) rawObject {
	t.Helper()
	data, err := json.Marshal(r.Attributes)
	if err != nil {
		t.Fatal(err)
	}
	return obj(data)
}

func TestGroupedDiskContainerAndServiceProperties(t *testing.T) {
	_, b := groupedFixture(t)
	b.groupReadings()
	counts := map[string]int{}
	for _, r := range b.readings {
		if _, _, grouped := groupedPrimary(r.Family, r.Component); grouped {
			counts[r.Family]++
		}
	}
	if counts["smart"] != 3 || counts["smart_attributes"] != 0 || counts["containers"] != 1 || counts["services"] != 1 {
		t.Fatal("expected one entity per disk, container and service", counts)
	}
	smart := requireValue(t, b, "smart", "smt000000000001", "status", "PASSED")
	attrs := readingAttrs(t, smart)
	for field, want := range map[string]string{"capacity_bytes": "1000204886016", "power_on_seconds": "72000", "power_on_hours": "20", "power_cycles": "2", "temperature_celsius": "32"} {
		if string(attrs[field]) != want {
			t.Errorf("SMART %s: %s, want %s", field, attrs[field], want)
		}
	}
	if attrs.text("power_on_human") != "20 hours" || attrs.text("device") != "/dev/sda" || attrs.text("model") != "CT1000BX500SSD1" || attrs.text("last_updated") == "" {
		t.Fatal("lost SMART metadata or readable power-on time", attrs)
	}
	if attrs.text("capacity_readable") != "1 TB" {
		t.Fatal("lost readable SMART capacity", attrs)
	}
	attributes := array(attrs["smart_attributes"])
	crc := obj(attributes[len(attributes)-1])
	if crc.text("name") != "CRC_Error_Count" || crc.text("raw_value") != "9007199254740993" || string(crc["id"]) != "199" {
		t.Fatal("raw SMART properties lost precision or identity", crc)
	}
	unknown := readingAttrs(t, requireValue(t, b, "smart", "smt000000000002", "status", "UNKNOWN"))
	zero := readingAttrs(t, requireValue(t, b, "smart", "smt000000000003", "status", "PASSED"))
	for _, field := range []string{"power_on_seconds", "power_cycles", "temperature_celsius"} {
		if _, exists := unknown[field]; exists || string(zero[field]) != "0" {
			t.Fatalf("%s lost unsupported versus valid zero semantics", field)
		}
	}
	container := readingAttrs(t, requireValue(t, b, "containers", "abcdef123456", "status", "Up 20 hours"))
	for field, want := range map[string]string{"cpu_usage_percent": "1.5", "memory_bytes": "134217728", "bandwidth_bytes_per_second": "300", "receive_bytes_per_second": "200", "transmit_bytes_per_second": "100", "update_available": "true"} {
		if string(container[field]) != want {
			t.Errorf("container %s: %s, want %s", field, container[field], want)
		}
	}
	if container.text("health") != "healthy" || container.text("last_updated") == "" {
		t.Fatal("lost container health or update time")
	}
	for field, want := range map[string]string{"memory_readable": "134.22 MB", "bandwidth_readable": "300 B/s", "receive_readable": "200 B/s", "transmit_readable": "100 B/s"} {
		if container.text(field) != want {
			t.Errorf("container %s: %s, want %s", field, container.text(field), want)
		}
	}
	service := readingAttrs(t, requireValue(t, b, "services", "svc000000000001", "state", "failed"))
	if service.text("substate") != "failed" || string(service["cpu_usage_percent"]) != "0" || string(service["memory_bytes"]) != "12345678" || string(service["memory_peak_bytes"]) != "23456789" {
		t.Fatal("lost service properties", service)
	}
	if service.text("memory_readable") != "12.35 MB" || service.text("memory_peak_readable") != "23.46 MB" {
		t.Fatal("lost readable service memory", service)
	}
	requireValue(t, b, "cpu", "0", "usage", "10")
	// Repeated grouping must not discard previously attached attributes.
	b.groupReadings()
	if string(readingAttrs(t, b.readings[smart.Key])["power_on_hours"]) != "20" {
		t.Fatal("grouping was not idempotent")
	}
}

func TestGroupedSMARTSelectionAndVendorProperties(t *testing.T) {
	for _, selector := range []string{"smart", "smart_attributes"} {
		data, b, _ := expandedFixture(t)
		b.cfg.GroupEntities = true
		b.cfg.Metrics = []string{selector}
		if err := b.cfg.validate(); err != nil {
			t.Fatal(err)
		}
		b.smartMetrics(expandedRecords(data, "smart_devices"))
		b.groupReadings()
		if len(b.readings) != 3 {
			t.Fatalf("%s: expected only three disk summaries, got %d", selector, len(b.readings))
		}
		attrs := readingAttrs(t, requireValue(t, b, "smart", "smt000000000001", "status", "PASSED"))
		if _, present := attrs["smart_attributes"]; present != (selector == "smart_attributes") {
			t.Fatal("SMART raw-attribute filter ignored", selector)
		}
	}
	properties := smartAttributeDetails(obj(json.RawMessage(`{"id":1,"n":"vendor","v":99,"w":98,"t":10,"rv":18446744073709551615,"rs":"raw","wf":"now","vendor_extension":{"flag":true}}`)))
	data, _ := json.Marshal(properties)
	attrs := obj(data)
	if attrs.text("raw_value") != "18446744073709551615" || string(attrs["normalized_value"]) != "99" || string(attrs["worst_value"]) != "98" || string(attrs["threshold"]) != "10" || attrs.text("when_failed") != "now" || string(attrs["vendor_extension"]) != `{"flag":true}` {
		t.Fatal("vendor SMART data was lost", string(data))
	}
}

func TestGroupedPercentagePrecision(t *testing.T) {
	data, b, _ := expandedFixture(t)
	b.cfg.GroupEntities = true
	containers := expandedRecords(data, "containers")
	containers.Rows[0]["cpu"] = json.RawMessage(`33.3333333333`)
	services := expandedRecords(data, "systemd_services")
	services.Rows[0]["cpuPeak"] = json.RawMessage(`125.5555`)
	b.containerMetrics(containers, expandedRecords(data, "container_stats"))
	b.serviceMetrics(services)
	b.groupReadings()
	container := readingAttrs(t, requireValue(t, b, "containers", "abcdef123456", "status", "Up 20 hours"))
	service := readingAttrs(t, requireValue(t, b, "services", "svc000000000001", "state", "failed"))
	if string(container["cpu_usage_percent"]) != "33.33" || string(service["cpu_peak_percent"]) != "125.56" {
		t.Fatal("grouped percentages were not rounded as numbers", container, service)
	}
}

func TestGroupedStaleAndAmbiguousContainerRates(t *testing.T) {
	for _, mode := range []string{"fresh", "stale", "duplicate name", "invalid rate"} {
		t.Run(mode, func(t *testing.T) {
			data, b, _ := expandedFixture(t)
			b.cfg.GroupEntities = true
			containers, stats := expandedRecords(data, "containers"), expandedRecords(data, "container_stats")
			switch mode {
			case "stale":
				stamp, _ := json.Marshal(b.now.Add(-181 * time.Second).Format(time.RFC3339Nano))
				stats.Rows[0]["created"] = stamp
			case "duplicate name":
				duplicate := rawObject{}
				for field, value := range containers.Rows[0] {
					duplicate[field] = value
				}
				duplicate["id"] = json.RawMessage(`"other"`)
				containers.Rows = append(containers.Rows, duplicate)
			case "invalid rate":
				stats.Rows[0]["stats"] = json.RawMessage(`[{"n":"web","b":[-1,"invalid"]}]`)
			}
			b.containerMetrics(containers, stats)
			b.groupReadings()
			attrs := readingAttrs(t, requireValue(t, b, "containers", "abcdef123456", "status", "Up 20 hours"))
			_, receive := attrs["receive_bytes_per_second"]
			_, transmit := attrs["transmit_bytes_per_second"]
			_, receiveReadable := attrs["receive_readable"]
			_, transmitReadable := attrs["transmit_readable"]
			if receive != (mode == "fresh") || transmit != (mode == "fresh") || receiveReadable != receive || transmitReadable != transmit || string(attrs["memory_bytes"]) != "134217728" {
				t.Fatal("invalid rates contaminated the current inventory properties", attrs)
			}
		})
	}
}

func TestGroupingMigrationAndRestoringIndividualLayout(t *testing.T) {
	data, b, sample := expandedFixture(t)
	b.systemMetrics(sample)
	b.smartMetrics(expandedRecords(data, "smart_devices"))
	b.containerMetrics(expandedRecords(data, "containers"), expandedRecords(data, "container_stats"))
	b.serviceMetrics(expandedRecords(data, "systemd_services"))
	s, p := telemetryService(t, b)
	if err := s.publishTelemetry(b.system, b); err != nil {
		t.Fatal(err)
	}
	original := len(s.state.Topics)
	summaryKey := metricKey("smart", "smt000000000001", "status")
	summaryTopic := s.state.discoveryTopic("sensor", b.system.ID, summaryKey)
	originalSummary := p.last(summaryTopic)
	oldDetails := []string{}
	for topic, entity := range s.state.Topics {
		if _, primary, grouped := groupedPrimary(entity.Family, entity.Component); grouped && primary != entity.Metric {
			oldDetails = append(oldDetails, topic)
		}
	}
	if len(oldDetails) == 0 {
		t.Fatal("fixture must include individual detail entities")
	}
	_, next := groupedFixture(t)
	s.cfg = next.cfg
	if err := s.publishTelemetry(next.system, next); err != nil {
		t.Fatal(err)
	}
	if len(s.state.Topics) != original-len(oldDetails) || p.last(summaryTopic) != originalSummary {
		t.Fatal("grouping changed summary identity or left obsolete discovery")
	}
	for _, topic := range oldDetails {
		if _, exists := s.state.Topics[topic]; exists || p.last(topic) != "" {
			t.Fatal("obsolete individual discovery was not removed", topic)
		}
	}
	state, err := loadState(s.dir)
	if err != nil || len(state.Topics) != len(s.state.Topics) {
		t.Fatal("grouped manifest did not persist", err)
	}
	// An unavailable Hub after restart must not resurrect obsolete discovery.
	if err := s.publishTelemetry(next.system, newTelemetry(next.cfg, next.system, next.now)); err != nil {
		t.Fatal(err)
	}
	for _, topic := range oldDetails {
		if p.last(topic) != "" {
			t.Fatal("obsolete discovery reappeared without readings")
		}
	}
	individual := newTelemetry(b.cfg, b.system, b.now)
	individual.systemMetrics(sample)
	individual.smartMetrics(expandedRecords(data, "smart_devices"))
	individual.containerMetrics(expandedRecords(data, "containers"), expandedRecords(data, "container_stats"))
	individual.serviceMetrics(expandedRecords(data, "systemd_services"))
	s.cfg = individual.cfg
	if err := s.publishTelemetry(individual.system, individual); err != nil {
		t.Fatal(err)
	}
	for _, topic := range oldDetails {
		if _, exists := s.state.Topics[topic]; !exists || p.last(topic) == "" {
			t.Fatal("individual detail discovery did not return", topic)
		}
	}
	if p.last(summaryTopic) != originalSummary {
		t.Fatal("switching back changed the existing summary")
	}
}

type groupingFailPublisher struct {
	publisher
	topic string
}

func (p groupingFailPublisher) publish(topic string, payload any, retain bool) error {
	if topic == p.topic {
		return errors.New("test broker failure")
	}
	return p.publisher.publish(topic, payload, retain)
}

func TestGroupingPublicationFailurePreservesIndividualDiscovery(t *testing.T) {
	data, b, _ := expandedFixture(t)
	b.smartMetrics(expandedRecords(data, "smart_devices"))
	s, p := telemetryService(t, b)
	if err := s.publishTelemetry(b.system, b); err != nil {
		t.Fatal(err)
	}
	count := len(s.state.Topics)
	b.cfg.GroupEntities = true
	s.cfg = b.cfg
	next := newTelemetry(b.cfg, b.system, b.now)
	next.smartMetrics(expandedRecords(data, "smart_devices"))
	s.pub = groupingFailPublisher{publisher: p, topic: s.state.hostRoot(b.system.ID) + "/" + metricKey("smart", "smt000000000001", "status") + "/attributes"}
	if err := s.publishTelemetry(next.system, next); err == nil {
		t.Fatal("broker failure was ignored")
	}
	if len(s.state.Topics) != count {
		t.Fatal("removed old discovery before the summary was published")
	}
	for topic := range s.state.Topics {
		if p.last(topic) == "" {
			t.Fatal("cleared old discovery after a failed summary publish")
		}
	}
}

func TestGroupingWaitsForCompleteFreshReplacement(t *testing.T) {
	for _, mode := range []string{"incomplete", "stale", "missing"} {
		t.Run(mode, func(t *testing.T) {
			data, b, _ := expandedFixture(t)
			b.smartMetrics(expandedRecords(data, "smart_devices"))
			s, _ := telemetryService(t, b)
			if err := s.publishTelemetry(b.system, b); err != nil {
				t.Fatal(err)
			}
			count := len(s.state.Topics)
			b.cfg.GroupEntities = true
			s.cfg = b.cfg
			next := newTelemetry(b.cfg, b.system, b.now)
			if mode != "missing" {
				next.smartMetrics(expandedRecords(data, "smart_devices"))
			}
			if mode == "incomplete" {
				next.complete["smart"], next.complete["smart_attributes"] = false, false
			} else if mode == "stale" {
				next.now = next.now.Add(49 * time.Hour)
			}
			if err := s.publishTelemetry(next.system, next); err != nil {
				t.Fatal(err)
			}
			if len(s.state.Topics) != count {
				t.Fatal("removed details without a complete, fresh summary")
			}
		})
	}
}
