package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// Grouping reuses the existing status/state identifiers. Host-wide counters and
// other metric families keep their independent entities and statistics.
func groupedPrimary(family, component string) (string, string, bool) {
	_, identity, ok := strings.Cut(component, ":")
	if !ok || identity == "" {
		return "", "", false
	}
	switch family {
	case "smart", "smart_attributes":
		return "smart", metricKey("smart", identity, "status"), true
	case "containers":
		return family, metricKey(family, identity, "status"), true
	case "services":
		return family, metricKey(family, identity, "state"), true
	}
	return "", "", false
}

var groupedAttributeNames = map[string]map[string]string{
	"smart": {
		"capacity": "capacity_bytes", "capacity_readable": "capacity_readable", "power_on_seconds": "power_on_seconds",
		"power_on_human": "power_on_human", "cycles": "power_cycles",
		"temperature": "temperature_celsius", "last_updated": "last_updated",
	},
	"containers": {
		"health": "health", "cpu": "cpu_usage_percent", "memory": "memory_bytes",
		"memory_readable": "memory_readable", "bandwidth_readable": "bandwidth_readable",
		"receive_readable": "receive_readable", "transmit_readable": "transmit_readable",
		"bandwidth": "bandwidth_bytes_per_second", "receive": "receive_bytes_per_second",
		"transmit": "transmit_bytes_per_second", "update_available": "update_available",
		"last_updated": "last_updated",
	},
	"services": {
		"substate": "substate", "cpu": "cpu_usage_percent", "cpu_peak": "cpu_peak_percent",
		"memory": "memory_bytes", "memory_peak": "memory_peak_bytes", "last_updated": "last_updated",
		"memory_readable": "memory_readable", "memory_peak_readable": "memory_peak_readable",
	},
}

func (b *telemetry) groupReadings() {
	if !b.cfg.GroupEntities {
		return
	}
	for key, r := range b.readings {
		_, primary, grouped := groupedPrimary(r.Family, r.Component)
		if !grouped || key == primary {
			continue
		}
		summary, exists := b.readings[primary]
		if !exists {
			continue
		}
		// Source-specific rates must not look fresh merely because inventory was
		// refreshed. Unsupported, malformed and stale values stay absent.
		validTime := !r.Source.IsZero() && !r.Source.After(b.now.Add(30*time.Second))
		_, identity, _ := strings.Cut(r.Component, ":")
		metric := strings.TrimPrefix(key, metricKey(r.Family, identity, ""))
		if field := groupedAttributeNames[r.Family][metric]; field != "" && validTime && (r.Snapshot || fresh(r.Source, b.now, r.Definition.Expiry)) {
			var value any = r.Value
			if r.Definition.Domain == "binary_sensor" {
				value = r.Value == "ON"
			} else if r.Definition.Unit != "" || r.Definition.StateClass != "" {
				value = json.Number(r.Value)
			}
			summary.Attributes[field] = value
			if field == "power_on_seconds" {
				if seconds, err := strconv.ParseUint(r.Value, 10, 64); err == nil {
					summary.Attributes["power_on_hours"] = seconds / 3600
					summary.Attributes["power_on_source_precision_seconds"] = 3600
				}
			}
			if !r.Source.Equal(summary.Source) {
				timestamps, _ := summary.Attributes["attribute_timestamps"].(map[string]string)
				if timestamps == nil {
					timestamps = map[string]string{}
					summary.Attributes["attribute_timestamps"] = timestamps
				}
				timestamps[field] = r.Source.UTC().Format(time.RFC3339Nano)
			}
		}
		// Readable update ages are intentionally omitted: last_updated already
		// explains age without changing attributes on every otherwise idle poll.
		delete(b.readings, key)
	}
}

// Preserve every reported vendor property, including unfamiliar fields, and
// keep large raw integers as strings for templates and JavaScript clients.
func smartAttributeDetails(attribute rawObject) map[string]any {
	names := map[string]string{"n": "name", "v": "normalized_value", "w": "worst_value", "t": "threshold", "rv": "raw_value", "rs": "raw_string", "wf": "when_failed"}
	result := map[string]any{}
	for field, value := range attribute {
		name := names[field]
		if name == "" {
			name = field
		}
		result[name] = value
	}
	if raw, ok := attribute.uint("rv"); ok {
		result["raw_value"] = strconv.FormatUint(raw, 10)
	}
	return result
}
