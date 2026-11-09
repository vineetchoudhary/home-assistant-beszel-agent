package main

import (
	"math"
	"strconv"
	"strings"
	"time"
)

func (b *telemetry) smartMetrics(result collectionResult) {
	b.scope("smart", result.Complete)
	b.scope("smart_attributes", result.Complete)
	for _, disk := range result.Rows {
		id := disk.text("id")
		b.component("smart", id)
		b.component("smart_attributes", id)
		source := recordTime(disk, "updated")
		ttl := b.cfg.detailExpiry()
		name := disk.text("name")
		if name == "" {
			name = id
		}
		prefix := "SMART " + name
		attrs := metadata(disk, "model", "type", "firmware", "serial")
		attrs["device"] = name
		attrs["record_id"] = id
		if capacity, ok := disk.uint("capacity"); ok && capacity > 0 {
			attrs["capacity_bytes"] = capacity
			b.add("smart", id, "capacity", prefix+" capacity", strconv.FormatUint(capacity, 10), "B", "data_size", "measurement", source, ttl, true, false, nil)
		}
		b.text("smart", id, "status", prefix+" status", disk.text("state"), source, ttl, true, false, attrs)
		b.timestamp("smart", id, "last_updated", prefix+" last updated", source, true)
		b.age("smart", id, prefix, source, true)
		attributes := array(disk["attributes"])
		hoursSupported, cyclesSupported, tempSupported := false, false, false
		seen := map[string]bool{}
		for _, raw := range attributes {
			attribute := obj(raw)
			attributeName := attribute.text("n")
			if attribute == nil || attributeName == "" {
				b.scope("smart_attributes", false)
				continue
			}
			lower := strings.ToLower(attributeName)
			hoursSupported = hoursSupported || strings.Contains(lower, "poweronhours") || strings.Contains(lower, "power_on_hours")
			cyclesSupported = cyclesSupported || (strings.Contains(lower, "power") && strings.Contains(lower, "cycle")) || strings.Contains(lower, "startstopcycles")
			tempSupported = tempSupported || strings.Contains(lower, "temperature")
			identity := "name:" + attributeName
			if aid, ok := attribute.uint("id"); ok && aid != 0 {
				identity = "id:" + strconv.FormatUint(aid, 10)
			}
			if seen[identity] {
				b.scope("smart_attributes", false)
				continue
			}
			seen[identity] = true
			attributeAttrs := metadata(attribute, "id", "n", "v", "w", "t", "rs", "wf")
			attributeAttrs["device"] = name
			attributeAttrs["record_id"] = id
			if v, ok := attribute.uint("rv"); ok {
				attributeAttrs["raw_value"] = strconv.FormatUint(v, 10)
				b.add("smart_attributes", id, "attribute_"+identityToken(identity), prefix+" "+attributeName, strconv.FormatUint(v, 10), "", "", "", source, ttl, false, false, attributeAttrs)
			} else if value := attribute.text("rs"); value != "" {
				b.text("smart_attributes", id, "attribute_"+identityToken(identity), prefix+" "+attributeName, value, source, ttl, false, false, attributeAttrs)
			}
		}
		if raw, exists := disk["attributes"]; exists && string(raw) != "null" && attributes == nil {
			b.scope("smart_attributes", false)
		}
		if hours, ok := disk.uint("hours"); ok && (hours > 0 || hoursSupported) && hours <= math.MaxUint64/3600 {
			seconds := hours * 3600
			precision := map[string]any{"source_unit": "hours", "source_precision_seconds": 3600}
			b.add("smart", id, "power_on_seconds", prefix+" power on", strconv.FormatUint(seconds, 10), "s", "duration", "measurement", source, ttl, true, false, cloneAttrs(precision))
			b.text("smart", id, "power_on_human", prefix+" power on readable", humanDuration(seconds), source, ttl, true, false, cloneAttrs(precision))
		}
		if cycles, ok := disk.uint("cycles"); ok && (cycles > 0 || cyclesSupported) {
			b.add("smart", id, "cycles", prefix+" power cycles", strconv.FormatUint(cycles, 10), "", "", "total_increasing", source, ttl, true, false, nil)
		}
		v, ok := disk.number("temp")
		b.gauge("smart", id, "temperature", prefix+" temperature", v, ok && v >= 0 && (v > 0 || tempSupported), "°C", "temperature", source, ttl, true, nil)
	}
}

func (b *telemetry) containerMetrics(result, stats collectionResult) {
	b.scope("containers", result.Complete)
	// Historical container samples identify containers by display name. Join only
	// unique names so duplicate names never assign one container's rates to another.
	counts := map[string]int{}
	rates := map[string]rawObject{}
	sampleSource := time.Time{}
	for _, container := range result.Rows {
		counts[container.text("name")]++
	}
	if len(stats.Rows) > 0 && stats.Rows[0].text("type") == "1m" {
		sampleSource = recordTime(stats.Rows[0], "created")
		for _, raw := range array(stats.Rows[0]["stats"]) {
			row := obj(raw)
			name := row.text("n")
			if _, exists := rates[name]; exists {
				counts[name]++
			}
			rates[name] = row
		}
	}
	for _, container := range result.Rows {
		id := container.text("id")
		b.component("containers", id)
		name := container.text("name")
		if name == "" {
			name = id
		}
		prefix := "Container " + name
		source := recordTime(container, "updated")
		ttl := b.cfg.StaleAfter
		attrs := metadata(container, "name", "image", "ports")
		attrs["container_id"] = id
		b.text("containers", id, "status", prefix+" status", container.text("status"), source, ttl, false, false, attrs)
		if state, ok := container.uint("health"); ok && state < 4 {
			b.text("containers", id, "health", prefix+" health", []string{"none", "starting", "healthy", "unhealthy"}[state], source, ttl, false, false, nil)
		}
		v, ok := nonnegative(container, "cpu")
		b.gauge("containers", id, "cpu", prefix+" CPU", v, ok, "%", "", source, ttl, false, nil)
		v, ok = nonnegative(container, "memory")
		b.gauge("containers", id, "memory", prefix+" memory", v*mib, ok, "B", "data_size", source, ttl, false, nil)
		v, ok = nonnegative(container, "net")
		b.gauge("containers", id, "bandwidth", prefix+" bandwidth", v, ok, "B/s", "data_rate", source, ttl, false, nil)
		updatable, present := container.boolean("updatable")
		b.flag("containers", id, "update_available", prefix+" image update available", updatable, present, "update", source, ttl, nil)
		b.timestamp("containers", id, "last_updated", prefix+" last updated", source, false)
		if counts[container.text("name")] != 1 {
			continue
		}
		row := rates[container.text("name")]
		bandwidth := array(row["b"])
		for i, direction := range []string{"transmit", "receive"} {
			var rate float64
			var valid bool
			if _, native := row["b"]; native {
				if i < len(bandwidth) {
					rate, valid = fieldNumber(bandwidth[i])
				}
			} else {
				rate, valid = nonnegative(row, []string{"ns", "nr"}[i])
				rate *= mib
			}
			b.gauge("containers", id, direction, prefix+" "+direction, rate, valid && rate >= 0, "B/s", "data_rate", sampleSource, ttl, false, nil)
		}
	}
}

func (b *telemetry) serviceMetrics(result collectionResult) {
	b.scope("services", result.Complete)
	for _, service := range result.Rows {
		id := service.text("id")
		b.component("services", id)
		name := service.text("name")
		if name == "" {
			name = id
		}
		prefix := "Service " + name
		source := recordTime(service, "updated")
		ttl := b.cfg.StaleAfter
		for _, entry := range []struct {
			field, key string
			names      []string
		}{
			{"state", "state", []string{"active", "inactive", "failed", "activating", "deactivating", "reloading"}},
			{"sub", "substate", []string{"dead", "running", "exited", "failed", "unknown"}},
		} {
			if v, ok := service.uint(entry.field); ok && v < uint64(len(entry.names)) {
				b.text("services", id, entry.key, prefix+" "+entry.key, entry.names[v], source, ttl, false, false, map[string]any{"service": name})
			}
		}
		for _, entry := range []struct{ field, key, name, unit, device string }{{"cpu", "cpu", "CPU", "%", ""}, {"cpuPeak", "cpu_peak", "CPU peak", "%", ""}, {"memory", "memory", "memory", "B", "data_size"}, {"memPeak", "memory_peak", "memory peak", "B", "data_size"}} {
			if entry.unit == "B" {
				if v, ok := service.uint(entry.field); ok {
					b.add("services", id, entry.key, prefix+" "+entry.name, strconv.FormatUint(v, 10), entry.unit, entry.device, "measurement", source, ttl, false, false, nil)
				}
			} else {
				v, ok := nonnegative(service, entry.field)
				b.gauge("services", id, entry.key, prefix+" "+entry.name, v, ok, entry.unit, entry.device, source, ttl, false, nil)
			}
		}
		b.timestamp("services", id, "last_updated", prefix+" last updated", source, false)
	}
}

func (b *telemetry) poolMetrics(result collectionResult, sample *Sample) {
	b.scope("storage_pools", result.Complete)
	var pools rawObject
	var sampleSource time.Time
	if sample != nil {
		pools = obj(sample.RawStats["z"])
		sampleSource, _ = sampleTime(sample.Created)
	}
	for _, pool := range result.Rows {
		id := pool.text("id")
		b.component("storage_pools", id)
		name := pool.text("display_name")
		if name == "" {
			name = pool.text("name")
		}
		if name == "" {
			name = id
		}
		prefix := "Pool " + name
		source := recordTime(pool, "updated")
		detailSource := recordTime(pool, "details_updated")
		if detailSource.IsZero() {
			b.scope("storage_pools", false)
		}
		ttl := b.cfg.StaleAfter
		detailTTL := b.cfg.detailExpiry()
		attrs := metadata(pool, "name", "display_name", "raw")
		attrs["record_id"] = id
		stats := obj(pools[pool.text("name")])
		health := pool.text("health")
		total, tok := nonnegative(pool, "size")
		used, uok := nonnegative(pool, "alloc")
		free, fok := nonnegative(pool, "free")
		if stats != nil && !sampleSource.IsZero() {
			source = sampleSource
			if v, ok := nonnegative(stats, "d"); ok {
				total, tok = v*gib, true
			}
			if v, ok := nonnegative(stats, "du"); ok {
				used, uok = v*gib, true
			}
			if tok && uok && used <= total {
				free, fok = total-used, true
			}
			if value := stats.text("h"); value != "" {
				health = value
			}
		}
		b.text("storage_pools", id, "health", prefix+" health", health, source, ttl, false, false, attrs)
		b.text("storage_pools", id, "advisory", prefix+" advisory", pool.text("status"), detailSource, detailTTL, false, false, nil)
		for _, entry := range []struct {
			key, name string
			value     float64
			valid     bool
		}{{"capacity", "capacity", total, tok}, {"used", "allocated", used, uok}, {"free", "free", free, fok}} {
			b.gauge("storage_pools", id, entry.key, prefix+" "+entry.name, entry.value, entry.valid, "B", "data_size", source, ttl, false, nil)
		}
		b.gauge("storage_pools", id, "usage", prefix+" usage", 100*used/total, tok && uok && total > 0 && used <= total, "%", "", source, ttl, false, nil)
		for i, direction := range []string{"read", "write"} {
			v, ok := nonnegative(stats, []string{"rb", "wb"}[i])
			b.gauge("storage_pools", id, direction+"_rate", prefix+" "+direction, v, ok, "B/s", "data_rate", sampleSource, ttl, false, nil)
		}
		b.timestamp("storage_pools", id, "details_updated", prefix+" details updated", detailSource, false)
		scrub := obj(pool["scrub"])
		if scrub != nil {
			b.text("storage_pools", id, "scrub_state", prefix+" scrub state", scrub.text("state"), detailSource, detailTTL, false, false, metadata(scrub, "progress"))
			if progress := strings.TrimSuffix(scrub.text("progress"), "%"); progress != "" {
				v, err := strconv.ParseFloat(progress, 64)
				b.gauge("storage_pools", id, "scrub_progress", prefix+" scrub progress", v, err == nil && v >= 0 && v <= 100, "%", "", detailSource, detailTTL, false, nil)
			}
			if v, ok := optionalZeroUint(scrub, "errors"); ok {
				b.add("storage_pools", id, "scrub_errors", prefix+" scrub errors", strconv.FormatUint(v, 10), "", "", "measurement", detailSource, detailTTL, false, false, nil)
			}
		}
		for _, raw := range array(pool["vdevs"]) {
			vdev := obj(raw)
			child := vdev.text("name")
			if child == "" {
				b.scope("storage_pools", false)
				continue
			}
			identity := id + "/vdev/" + child
			b.component("storage_pools", identity)
			b.text("storage_pools", identity, "state", prefix+" vdev "+child+" state", vdev.text("state"), detailSource, detailTTL, false, false, metadata(vdev, "name"))
			for _, entry := range []struct{ field, key string }{{"readErrs", "read_errors"}, {"writeErrs", "write_errors"}, {"checksumErrs", "checksum_errors"}} {
				if v, ok := optionalZeroUint(vdev, entry.field); ok {
					b.add("storage_pools", identity, entry.key, prefix+" vdev "+child+" "+entry.key, strconv.FormatUint(v, 10), "", "", "measurement", detailSource, detailTTL, false, false, nil)
				}
			}
		}
		for _, raw := range array(pool["datasets"]) {
			dataset := obj(raw)
			child := dataset.text("name")
			if child == "" {
				b.scope("storage_pools", false)
				continue
			}
			identity := id + "/dataset/" + child
			b.component("storage_pools", identity)
			for _, field := range []string{"used", "avail"} {
				if v, ok := optionalZeroUint(dataset, field); ok {
					b.add("storage_pools", identity, field, prefix+" dataset "+child+" "+field, strconv.FormatUint(v, 10), "B", "data_size", "measurement", detailSource, detailTTL, false, false, metadata(dataset, "name", "mount"))
				}
			}
		}
		for _, field := range []string{"vdevs", "datasets"} {
			if raw, exists := pool[field]; exists && string(raw) != "null" && array(raw) == nil {
				b.scope("storage_pools", false)
			}
		}
	}
}

// ZFS explicitly omits zero counters in nested, named objects. Missing counters
// in these objects mean zero; malformed or null values still mean unavailable.
func optionalZeroUint(o rawObject, field string) (uint64, bool) {
	if o == nil {
		return 0, false
	}
	if _, exists := o[field]; !exists {
		return 0, true
	}
	return o.uint(field)
}

func (b *telemetry) systemInfo(result collectionResult, sample *Sample) {
	b.scope("system_info", result.Complete)
	info := obj(b.system.Raw["info"])
	source := recordTime(b.system.Raw, "updated")
	if sample != nil {
		if created, err := sampleTime(sample.Created); err == nil {
			source = created
		}
	}
	attrs := metadata(info, "t", "ct", "rdn", "jl")
	if len(result.Rows) > 0 {
		details := result.Rows[0]
		attrs = metadata(details, "hostname", "os", "os_name", "kernel", "arch", "cpu", "cores", "threads", "sockets", "memory", "podman")
		for _, field := range []string{"t", "ct", "rdn", "jl"} {
			if raw, ok := info[field]; ok {
				attrs[field] = raw
			}
		}
		value := details.text("os_name")
		if value == "" {
			value = details.text("hostname")
		}
		b.text("system_info", "", "overview", "System information", value, recordTime(details, "updated"), b.cfg.detailExpiry(), false, true, attrs)
	}
	b.text("system_info", "", "agent_version", "Agent version", b.system.Info.Version, source, b.cfg.StaleAfter, false, true, cloneAttrs(attrs))
	if v, ok := info.uint("ct"); ok && v > 0 && v < 3 {
		b.text("system_info", "", "connection_type", "Agent connection", []string{"", "SSH", "WebSocket"}[v], source, b.cfg.StaleAfter, false, true, nil)
	}
}
