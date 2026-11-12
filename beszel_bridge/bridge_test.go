package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fixture[T any](t *testing.T, version, name string) T {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", version, name))
	if err != nil {
		t.Fatal(err)
	}
	var value T
	if err = json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func testConfig() Config {
	return Config{HubURL: "http://hub:8090/beszel", HubEmail: "reader@example.test", HubPassword: "secret-do-not-log", MQTTSource: "manual", MQTTHost: "broker", MQTTPort: 1883, PollInterval: 30, StaleAfter: 180, DiscoveryPrefix: "homeassistant", HAStatusTopic: "homeassistant/status"}
}

func TestEnableAllEntitiesConfiguration(t *testing.T) {
	for _, test := range []struct {
		name  string
		value any
		want  bool
		valid bool
	}{
		{"omitted", nil, false, true},
		{"disabled", false, false, true},
		{"enabled", true, true, true},
		{"string rejected", "true", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(testConfig())
			if err != nil {
				t.Fatal(err)
			}
			var options map[string]any
			if err = json.Unmarshal(data, &options); err != nil {
				t.Fatal(err)
			}
			delete(options, "enable_all_entities")
			if test.value != nil {
				options["enable_all_entities"] = test.value
			}
			data, err = json.Marshal(options)
			if err != nil {
				t.Fatal(err)
			}
			filename := filepath.Join(t.TempDir(), "options.json")
			if err = os.WriteFile(filename, data, 0o600); err != nil {
				t.Fatal(err)
			}
			config, err := loadConfig(filename)
			if (err == nil) != test.valid || config.EnableAllEntities != test.want {
				t.Fatalf("enable_all_entities = %v, error = %v", config.EnableAllEntities, err)
			}
		})
	}
}

func TestHistoryRetentionConfiguration(t *testing.T) {
	for _, days := range []int{0, 1, 2, 5, 365, -1, 366} {
		t.Run(strconv.Itoa(days), func(t *testing.T) {
			config := testConfig()
			config.HistoryRetentionDays = days
			err := config.validate()
			if (err == nil) != (days >= 0 && days <= 365) {
				t.Fatalf("history_retention_days %d: %v", days, err)
			}
		})
	}
}

func TestCapturedVersionMetricsAndSourceFreshness(t *testing.T) {
	var policy Compatibility
	if err := json.Unmarshal(compatibilityJSON, &policy); err != nil || len(policy.TestedHubVersions) == 0 {
		t.Fatal("compatibility policy must declare tested versions")
	}
	for _, version := range policy.TestedHubVersions {
		t.Run(version, func(t *testing.T) {
			info := fixture[HubInfo](t, version, "info.json")
			if info.Version != version || info.Key == "" {
				t.Fatal("declared version is missing its captured identity/version response")
			}
			system := fixture[page[System]](t, version, "systems.json").Items[0]
			sample := fixture[page[Sample]](t, version, "system_stats.json").Items[0]
			values, created, err := metricValues(system, &sample)
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"cpu_usage", "memory_usage", "disk_usage", "uptime", "network_receive", "network_transmit", "last_sample"} {
				if values[key] == "" {
					t.Errorf("missing verified metric %s", key)
				}
			}
			if _, ok := values["temperature"]; ok {
				t.Error("invented temperature from summary dt=0")
			}
			if values["network_receive"] != "0" || values["network_transmit"] != "0" {
				t.Error("idle interfaces did not retain valid zero rates")
			}
			system.Updated = created.Add(10 * time.Minute).Format(time.RFC3339Nano)
			_, sameCreated, err := metricValues(system, &sample)
			if err != nil {
				t.Fatal(err)
			}
			if !sameCreated.Equal(created) || fresh(sameCreated, created.Add(181*time.Second), 180) {
				t.Error("editing a system made an old source sample fresh")
			}
			if !fresh(created, created.Add(time.Minute), 180) || fresh(created, created.Add(-time.Minute), 180) {
				t.Error("freshness did not enforce timestamp age and future tolerance")
			}
		})
	}
}
func TestMissingAndMalformedMetricsAreNotZero(t *testing.T) {
	system := fixture[page[System]](t, "0.20.0", "systems.json").Items[0]
	sample := fixture[page[Sample]](t, "0.20.0", "system_stats.json").Items[0]
	zero := 0.0
	sample.Stats.CPU = &zero
	sample.Stats.Memory = &zero
	sample.Stats.Disk = &zero
	sample.Stats.Temperatures = map[string]float64{"observed_sensor": 0}
	values, _, err := metricValues(system, &sample)
	if err != nil {
		t.Fatal(err)
	}
	if values["cpu_usage"] != "0" || values["temperature"] != "0" {
		t.Error("actual numeric zero was discarded")
	}
	sample.Stats.Bandwidth = []float64{1024, 2048}
	values, _, err = metricValues(system, &sample)
	if err != nil {
		t.Fatal(err)
	}
	if values["network_receive"] != "2048" || values["network_transmit"] != "1024" {
		t.Error("bandwidth bytes/s were swapped or rescaled")
	}
	sample.Stats.Bandwidth = []float64{10}
	if _, _, err = metricValues(system, &sample); err == nil {
		t.Error("short tuple fabricated a second network value")
	}
	sample.Stats.Bandwidth = nil
	sample.Stats.CPU = nil
	if _, _, err = metricValues(system, &sample); err == nil {
		t.Error("missing required CPU became zero")
	}
	values, created, err := metricValues(system, nil)
	if err != nil || len(values) != 0 || !created.IsZero() {
		t.Error("system without a sample fabricated telemetry")
	}
}
func TestConfigurationBoundariesAndFilters(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Config)
	}{
		{"ingress", func(c *Config) { c.HubURL = "http://ha/api/hassio_ingress/token" }},
		{"url credentials", func(c *Config) { c.HubURL = "http://reader:secret-do-not-log@hub:8090" }},
		{"query secret", func(c *Config) { c.HubURL = "http://hub/?token=secret-do-not-log" }},
		{"poll lower bound", func(c *Config) { c.PollInterval = 1 }},
		{"stale versus polling", func(c *Config) { c.PollInterval = 300; c.StaleAfter = 180 }},
		{"detail expiry lower bound", func(c *Config) { c.DetailStaleAfter = 3599 }},
		{"detail expiry upper bound", func(c *Config) { c.DetailStaleAfter = 31536001 }},
		{"disk exclusion identity", func(c *Config) { c.DiskAggregateExclude = []diskExclusion{{System: "invalid", Filesystem: "root"}} }},
		{"disk exclusion filesystem", func(c *Config) { c.DiskAggregateExclude = []diskExclusion{{System: "sys000000000001"}} }},
		{"CA traversal", func(c *Config) { c.HubCAFile = "/ssl/../etc/passwords" }},
		{"topic wildcard", func(c *Config) { c.DiscoveryPrefix = "homeassistant/#" }},
		{"MQTT URL credentials", func(c *Config) { c.MQTTHost = "reader:secret-do-not-log@broker" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := testConfig()
			test.change(&c)
			err := c.validate()
			if err == nil {
				t.Fatal("invalid config accepted")
			}
			if strings.Contains(err.Error(), "secret-do-not-log") {
				t.Fatal("config error leaked secret")
			}
		})
	}
	c := testConfig()
	c.PollInterval = 300
	c.StaleAfter = 0
	if err := c.validate(); err != nil || c.StaleAfter != 900 {
		t.Fatalf("automatic expiry must account for long polling: %v, %d", err, c.StaleAfter)
	}
	c.IncludeSystems = []string{"sys000000000001"}
	c.ExcludeSystems = []string{"sys000000000001"}
	if c.selected("sys000000000001") || c.selected("sys000000000002") {
		t.Error("selection/exclusion did not limit publication")
	}
}
func TestStableIdentityAndPrivatePersistence(t *testing.T) {
	dir := t.TempDir()
	s, err := loadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	info := HubInfo{Key: "ssh-ed25519 stable-example-key", Version: "0.20.0"}
	if err = s.initialize(info, "homeassistant", "tcp://broker:1883"); err != nil {
		t.Fatal(err)
	}
	namespace := s.Namespace
	if err = s.save(dir); err != nil {
		t.Fatal(err)
	}
	restored, err := loadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = restored.initialize(info, "homeassistant", "tcp://broker:1883"); err != nil || restored.Namespace != namespace {
		t.Error("restore changed identity")
	}
	clean, _ := loadState(t.TempDir())
	_ = clean.initialize(info, "homeassistant", "tcp://broker:1883")
	if clean.Namespace != namespace {
		t.Error("clean reinstall against same Hub created a new identity")
	}
	if err = restored.initialize(HubInfo{Key: "ssh-ed25519 different-hub"}, "homeassistant", "tcp://broker:1883"); err == nil {
		t.Error("different Hub silently reused identity")
	}
	if err = restored.initialize(info, "new-prefix", "tcp://broker:1883"); err == nil {
		t.Error("prefix change orphaned old topics")
	}
	if err = restored.initialize(info, "homeassistant", "tcp://other-broker:1883"); err == nil {
		t.Error("broker change orphaned old topics")
	}
	stat, err := os.Stat(filepath.Join(dir, "bridge-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if stat.Mode().Perm() != 0600 {
		t.Error("private state permissions are too broad")
	}
}
