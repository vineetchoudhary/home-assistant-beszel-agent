package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

const gib = float64(1024 * 1024 * 1024)
const mib = float64(1024 * 1024)

var sampleFamilies = []string{"cpu", "memory", "swap", "filesystems", "disk_io", "network", "temperatures", "fans", "gpus", "batteries", "wifi", "services", "uptime_human"}

func (b *telemetry) systemMetrics(sample *Sample) {
	complete := false
	if sample != nil && len(sample.RawStats) > 0 {
		for _, key := range []string{"cpu", "mp", "dp"} {
			if _, valid := sample.RawStats.number(key); valid {
				complete = true
			}
		}
	}
	for _, family := range sampleFamilies {
		b.scope(family, complete)
	}
	if sample == nil {
		return
	}
	source, err := sampleTime(sample.Created)
	if err != nil {
		for _, family := range sampleFamilies {
			b.scope(family, false)
		}
		return
	}
	o := sample.RawStats
	info := obj(b.system.Raw["info"])
	ttl := b.cfg.StaleAfter
	if b.system.Info.Uptime != nil {
		b.text("uptime_human", "", "", "Uptime readable", humanDuration(*b.system.Info.Uptime), source, ttl, true, false, nil)
	}
	if !source.After(b.now.Add(30 * time.Second)) {
		b.add("diagnostics", "", "sample_age", "Sample age", strconv.FormatUint(uint64(max(0, b.now.Sub(source).Seconds())), 10), "s", "duration", "measurement", source, ttl, false, true, nil)
	}
	if o == nil {
		return
	}
	for _, family := range sampleFamilies {
		b.component(family, "")
	}
	for _, entry := range []struct{ field, family string }{{"cpus", "cpu"}, {"cpub", "cpu"}, {"la", "cpu"}, {"mem", "memory"}, {"swap", "swap"}, {"bat", "batteries"}} {
		if raw, exists := o[entry.field]; exists && array(raw) == nil {
			b.scope(entry.family, false)
		}
	}
	for i, field := range []string{"user", "system", "iowait", "steal", "idle", "irq", "softirq", "nice"} {
		values := array(o["cpub"])
		if i < len(values) {
			v, ok := fieldNumber(values[i])
			b.gauge("cpu", "", field, "CPU "+field, v, ok && v >= 0 && v <= 100, "%", "", source, ttl, false, nil)
		}
	}
	for i, raw := range array(o["cpus"]) {
		v, ok := fieldNumber(raw)
		id := strconv.Itoa(i)
		b.component("cpu", id)
		b.gauge("cpu", id, "usage", "CPU "+id+" usage", v, ok && v >= 0 && v <= 100, "%", "", source, ttl, false, nil)
	}
	for i, raw := range array(o["la"]) {
		if i >= 3 {
			break
		}
		v, ok := fieldNumber(raw)
		period := []string{"1m", "5m", "15m"}[i]
		b.gauge("cpu", "", "load_"+period, "Load average "+period, v, ok && v >= 0, "", "", source, ttl, false, nil)
	}
	b.memoryMetrics(o, source)
	b.filesystemMetrics(o, source)
	b.networkMetrics(o, source)
	for _, entry := range []struct{ family, field, metric, prefix, unit, device string }{
		{"temperatures", "t", "temperature", "Temperature", "°C", "temperature"},
		{"fans", "f", "speed", "Fan", "rpm", ""},
		{"batteries", "bats", "level", "Battery", "%", "battery"},
		{"wifi", "wf", "signal", "Wi-Fi", "dBm", "signal_strength"},
	} {
		values := obj(o[entry.field])
		if _, exists := o[entry.field]; exists && values == nil {
			b.scope(entry.family, false)
		}
		for _, identity := range keys(values) {
			b.component(entry.family, identity)
			v, ok := fieldNumber(values[identity])
			if entry.family == "fans" {
				ok = ok && v >= 0
			}
			if entry.family == "batteries" {
				ok = ok && v >= 0 && v <= 100
			}
			if entry.family == "wifi" {
				ok = ok && v >= -127 && v <= 0
			}
			attrs := map[string]any{"source_name": identity}
			if entry.family == "wifi" {
				attrs["interface"] = identity
				if ssid := obj(obj(info["wf"])[identity]).text("s"); ssid != "" {
					attrs["ssid"] = ssid
				}
			}
			b.gauge(entry.family, identity, entry.metric, label(entry.prefix, identity, entry.metric), v, ok, entry.unit, entry.device, source, ttl, false, attrs)
		}
	}
	for _, raw := range []json.RawMessage{o["bat"], info["bat"]} {
		values := array(raw)
		if len(values) < 2 {
			continue
		}
		level, ok := fieldNumber(values[0])
		b.gauge("batteries", "", "level", "Battery level", level, ok && level >= 0 && level <= 100, "%", "battery", source, ttl, false, nil)
		state, ok := fieldUint(values[1])
		if ok && state < 6 {
			b.text("batteries", "", "state", "Battery state", []string{"unknown", "empty", "full", "charging", "discharging", "idle"}[state], source, ttl, false, false, nil)
		}
		break
	}
	if v, ok := percentage(info, "g"); ok {
		b.gauge("gpus", "", "usage", "Highest GPU usage", v, true, "%", "", source, ttl, false, nil)
	}
	b.gpuMetrics(o, source)
	services := array(info["sv"])
	for i, key := range []string{"total", "failed"} {
		if i < len(services) {
			v, ok := fieldUint(services[i])
			if ok {
				b.add("services", "", "count_"+key, "Services "+key, strconv.FormatUint(v, 10), "", "", "measurement", source, ttl, false, false, nil)
			}
		}
	}
	updates := array(info["pu"])
	for i, key := range []string{"total", "security"} {
		if i < len(updates) {
			v, ok := fieldUint(updates[i])
			if ok {
				b.add("package_updates", "", "count_"+key, "Package updates "+key, strconv.FormatUint(v, 10), "", "", "measurement", source, ttl, false, false, nil)
			}
		}
	}
}

func (b *telemetry) memoryMetrics(o rawObject, source time.Time) {
	ttl := b.cfg.StaleAfter
	mem, swap := array(o["mem"]), array(o["swap"])
	values := map[string]float64{}
	for i, entry := range []struct{ key, legacy string }{{"total", "m"}, {"used", "mu"}, {"cache", "mb"}, {"zfs_arc", "mz"}} {
		var v float64
		var ok bool
		if _, native := o["mem"]; native {
			if i < len(mem) {
				v, ok = fieldNumber(mem[i])
			}
		} else {
			v, ok = nonnegative(o, entry.legacy)
			v *= gib
		}
		if !ok || !validNonnegative(v) {
			continue
		}
		values[entry.key] = v
		b.gauge("memory", "", entry.key, "Memory "+entry.key, v, true, "B", "data_size", source, ttl, false, nil)
	}
	if total, ok := values["total"]; ok {
		if used, ok := values["used"]; ok && used <= total {
			b.gauge("memory", "", "free", "Memory free", total-used, true, "B", "data_size", source, ttl, false, nil)
		}
	}
	swapValues := map[string]float64{}
	for i, entry := range []struct{ key, legacy string }{{"total", "s"}, {"used", "su"}} {
		var v float64
		var ok bool
		if _, native := o["swap"]; native {
			if i < len(swap) {
				v, ok = fieldNumber(swap[i])
			}
		} else {
			v, ok = nonnegative(o, entry.legacy)
			v *= gib
		}
		if !ok || !validNonnegative(v) {
			continue
		}
		swapValues[entry.key] = v
		b.gauge("swap", "", entry.key, "Swap "+entry.key, v, true, "B", "data_size", source, ttl, false, nil)
	}
	if total, ok := swapValues["total"]; ok {
		if used, ok := swapValues["used"]; ok && used <= total {
			b.gauge("swap", "", "free", "Swap free", total-used, true, "B", "data_size", source, ttl, false, nil)
			if total > 0 {
				b.gauge("swap", "", "usage", "Swap usage", 100*used/total, true, "%", "", source, ttl, false, nil)
			}
		}
	}
}

func (b *telemetry) filesystemMetrics(o rawObject, source time.Time) {
	ttl := b.cfg.StaleAfter
	info := obj(b.system.Raw["info"])
	rootName := info.text("rdn")
	if rootName == "" {
		rootName = "Primary disk"
	}
	filesystems := rawObject{"root": nil}
	names := map[string]string{}
	extra := obj(o["efs"])
	aggregateValid := true
	if _, exists := o["efs"]; exists && extra == nil {
		aggregateValid = false
		b.scope("filesystems", false)
		b.scope("disk_io", false)
	}
	for identity, raw := range extra {
		key := filesystemIdentity(identity)
		filesystems[key] = raw
		names[key] = identity
	}
	var total, used float64
	included := []string{}
	for _, identity := range keys(filesystems) {
		fs := o
		name := rootName
		if identity != "root" {
			fs = obj(filesystems[identity])
			name = "Disk " + names[identity]
		}
		b.component("filesystems", identity)
		b.component("disk_io", identity)
		if fs == nil {
			b.scope("filesystems", false)
			b.scope("disk_io", false)
		}
		d, dok := nonnegative(fs, "d")
		du, uok := nonnegative(fs, "du")
		valid := dok && uok && d > 0 && du <= d
		if !valid && (identity != "root" || fs["d"] != nil || fs["du"] != nil) {
			b.scope("filesystems", false)
			b.issue("invalid filesystem capacity or usage")
		}
		attrs := map[string]any{"filesystem": identity}
		if identity != "root" {
			attrs["source_name"] = names[identity]
		}
		b.gauge("filesystems", identity, "capacity", name+" capacity", d*gib, dok && d > 0, "B", "data_size", source, ttl, false, cloneAttrs(attrs))
		b.gauge("filesystems", identity, "used", name+" used", du*gib, uok, "B", "data_size", source, ttl, false, cloneAttrs(attrs))
		b.gauge("filesystems", identity, "free", name+" free", (d-du)*gib, valid, "B", "data_size", source, ttl, false, cloneAttrs(attrs))
		if identity != "root" {
			b.gauge("filesystems", identity, "usage", name+" usage", 100*du/d, valid, "%", "", source, ttl, true, cloneAttrs(attrs))
		}
		if b.cfg.aggregateIncludes(b.system.ID, identity) {
			included = append(included, identity)
			aggregateValid = aggregateValid && valid
			if valid {
				total += d
				used += du
			}
		}
		b.diskIOMetrics(fs, identity, name, source, false)
	}
	attrs := map[string]any{"filesystems": included, "capacity_scope": "monitored filesystems"}
	valid := aggregateValid && total > 0 && validNumber(total) && validNumber(used)
	for _, entry := range []struct {
		key, name, unit, device string
		value                   float64
	}{
		{"capacity", "Combined disk capacity", "B", "data_size", total * gib},
		{"used", "Combined disk used", "B", "data_size", used * gib},
		{"free", "Combined disk free", "B", "data_size", (total - used) * gib},
		{"usage", "Combined disk usage", "%", "", 100 * used / total},
	} {
		b.gauge("filesystems", "", "combined_"+entry.key, entry.name, entry.value, valid, entry.unit, entry.device, source, ttl, true, cloneAttrs(attrs))
	}
}

func (b *telemetry) diskIOMetrics(o rawObject, identity, name string, source time.Time, peak bool) {
	ttl := b.cfg.StaleAfter
	suffix := ""
	attrs := map[string]any{"filesystem": identity}
	if peak {
		ttl = max(ttl, 1800)
		suffix = " peak 10m"
		attrs["source_window"] = "10m"
	}
	rateKey, totalKey := []string{"rb", "wb"}, []string{"tr", "tw"}
	if identity == "root" {
		rateKey = []string{"dio", "diom"}
		totalKey = []string{"diot"}
	} else if peak {
		rateKey = []string{"rbm", "wbm"}
	}
	for i, direction := range []string{"read", "write"} {
		var v float64
		var ok bool
		if identity == "root" {
			field := rateKey[0]
			if peak {
				field = rateKey[1]
			}
			values := array(o[field])
			if i < len(values) {
				v, ok = fieldNumber(values[i])
			}
		} else {
			v, ok = nonnegative(o, rateKey[i])
			if _, exists := o[rateKey[i]]; !exists && !peak {
				v, ok = nonnegative(o, []string{"r", "w"}[i])
				v *= mib
			}
		}
		metric := direction + "_rate"
		if peak {
			metric += "_peak_10m"
		}
		field := rateKey[i]
		if identity == "root" {
			field = rateKey[0]
			if peak {
				field = rateKey[1]
			}
		}
		if b.omittedZero(o, field, metricKey("disk_io", identity, metric)) {
			v, ok = 0, true
		}
		b.gauge("disk_io", identity, metric, name+" "+direction+suffix, v, ok && v >= 0, "B/s", "data_rate", source, ttl, false, cloneAttrs(attrs))
		if !peak {
			var count uint64
			if identity == "root" {
				values := array(o[totalKey[0]])
				if i < len(values) {
					count, ok = fieldUint(values[i])
				} else {
					ok = false
				}
			} else {
				count, ok = o.uint(totalKey[i])
			}
			b.counter("disk_io", identity, direction+"_total", name+" total "+direction, count, ok, "B", "data_size", source, ttl, cloneAttrs(attrs))
		}
	}
	field := "dios"
	if peak {
		field = "diosm"
	}
	stats := array(o[field])
	for i, entry := range []struct{ key, name, unit, device string }{
		{"read_time", "read time factor", "", ""}, {"write_time", "write time factor", "", ""},
		{"utilization", "I/O utilization", "%", ""}, {"read_latency", "read latency", "ms", "duration"},
		{"write_latency", "write latency", "ms", "duration"}, {"weighted_io", "weighted I/O", "%", ""},
	} {
		if i >= len(stats) {
			continue
		}
		v, ok := fieldNumber(stats[i])
		key := entry.key
		if peak {
			key += "_peak_10m"
		}
		b.gauge("disk_io", identity, key, name+" "+entry.name+suffix, v, ok && v >= 0, entry.unit, entry.device, source, ttl, false, cloneAttrs(attrs))
	}
}

func (b *telemetry) networkMetrics(o rawObject, source time.Time) {
	ttl := b.cfg.StaleAfter
	interfaces := obj(o["ni"])
	if _, exists := o["ni"]; exists && interfaces == nil {
		b.scope("network", false)
	}
	for _, identity := range keys(interfaces) {
		b.component("network", identity)
		values := array(interfaces[identity])
		for i, direction := range []string{"transmit", "receive"} {
			attrs := map[string]any{"interface": identity}
			if i < len(values) {
				v, ok := fieldNumber(values[i])
				b.gauge("network", identity, direction, "Network "+identity+" "+direction, v, ok && v >= 0, "B/s", "data_rate", source, ttl, false, cloneAttrs(attrs))
			}
			if i+2 < len(values) {
				v, ok := fieldUint(values[i+2])
				b.counter("network", identity, direction+"_total", "Network "+identity+" total "+direction, v, ok, "B", "data_size", source, ttl, cloneAttrs(attrs))
			}
		}
	}
	bandwidth := array(o["b"])
	var sum float64
	valid := false
	if len(bandwidth) >= 2 {
		a, aok := fieldNumber(bandwidth[0])
		z, zok := fieldNumber(bandwidth[1])
		sum = a + z
		valid = aok && zok && a >= 0 && z >= 0
	} else if _, exists := o["b"]; !exists && len(interfaces) > 0 {
		valid = true
		for _, raw := range interfaces {
			values := array(raw)
			if len(values) < 2 {
				valid = false
				continue
			}
			for _, v := range values[:2] {
				n, ok := fieldNumber(v)
				sum += n
				valid = valid && ok && n >= 0
			}
		}
	}
	b.gauge("network", "", "bandwidth", "Network bandwidth", sum, valid, "B/s", "data_rate", source, ttl, false, nil)
}

func (b *telemetry) gpuMetrics(o rawObject, source time.Time) {
	gpus := obj(o["g"])
	if _, exists := o["g"]; exists && gpus == nil {
		b.scope("gpus", false)
	}
	for _, identity := range keys(gpus) {
		b.component("gpus", identity)
		gpu := obj(gpus[identity])
		if gpu == nil {
			b.scope("gpus", false)
			continue
		}
		name := gpu.text("n")
		if name == "" {
			name = identity
		}
		prefix := "GPU " + name
		attrs := map[string]any{"gpu_id": identity, "model": name}
		for _, entry := range []struct {
			field, key, name, unit, device string
			factor                         float64
		}{
			{"u", "usage", "usage", "%", "", 1}, {"mu", "memory_used", "memory used", "B", "data_size", mib},
			{"mt", "memory_total", "memory total", "B", "data_size", mib}, {"p", "power", "power", "W", "power", 1}, {"pp", "package_power", "package power", "W", "power", 1},
		} {
			v, ok := nonnegative(gpu, entry.field)
			if b.omittedZero(gpu, entry.field, metricKey("gpus", identity, entry.key)) {
				v, ok = 0, true
			}
			if entry.field == "u" {
				ok = ok && v <= 100
			}
			b.gauge("gpus", identity, entry.key, prefix+" "+entry.name, v*entry.factor, ok, entry.unit, entry.device, source, b.cfg.StaleAfter, false, cloneAttrs(attrs))
		}
		total, tok := nonnegative(gpu, "mt")
		used, uok := nonnegative(gpu, "mu")
		valid := tok && uok && total > 0 && used <= total
		b.gauge("gpus", identity, "memory_free", prefix+" memory free", (total-used)*mib, valid, "B", "data_size", source, b.cfg.StaleAfter, false, cloneAttrs(attrs))
		b.gauge("gpus", identity, "memory_usage", prefix+" memory usage", 100*used/total, valid, "%", "", source, b.cfg.StaleAfter, false, cloneAttrs(attrs))
		engines := obj(gpu["e"])
		for _, engine := range keys(engines) {
			v, ok := fieldNumber(engines[engine])
			b.gauge("gpus", identity, "engine_"+identityToken(engine), prefix+" "+engine+" usage", v, ok && v >= 0 && v <= 100, "%", "", source, b.cfg.StaleAfter, false, map[string]any{"gpu_id": identity, "engine": engine})
		}
	}
}

func (b *telemetry) peakMetrics(sample *Sample) {
	if sample == nil || sample.Type != "10m" {
		return
	}
	source, err := sampleTime(sample.Created)
	if err != nil {
		return
	}
	o := sample.RawStats
	ttl := max(1800, b.cfg.StaleAfter)
	attrs := map[string]any{"source_window": "10m"}
	v, ok := percentage(o, "cpum")
	if b.omittedZero(o, "cpum", metricKey("cpu", "", "peak_10m")) {
		v, ok = 0, true
	}
	b.gauge("cpu", "", "peak_10m", "CPU usage peak 10m", v, ok, "%", "", source, ttl, false, cloneAttrs(attrs))
	v, ok = nonnegative(o, "memm")
	if _, exists := o["memm"]; !exists {
		v, ok = nonnegative(o, "mm")
		v *= gib
	}
	if b.omittedZero(o, "memm", metricKey("memory", "", "used_peak_10m")) {
		if _, legacy := o["mm"]; !legacy {
			v, ok = 0, true
		}
	}
	b.gauge("memory", "", "used_peak_10m", "Memory used peak 10m", v, ok, "B", "data_size", source, ttl, false, cloneAttrs(attrs))
	bandwidth := array(o["bm"])
	for i, direction := range []string{"transmit", "receive"} {
		if i < len(bandwidth) {
			v, ok := fieldNumber(bandwidth[i])
			b.gauge("network", "", direction+"_peak_10m", "Network "+direction+" peak 10m", v, ok && v >= 0, "B/s", "data_rate", source, ttl, false, cloneAttrs(attrs))
		}
	}
	b.diskIOMetrics(o, "root", "Primary disk", source, true)
	for _, identity := range keys(obj(o["efs"])) {
		b.diskIOMetrics(obj(obj(o["efs"])[identity]), filesystemIdentity(identity), "Disk "+identity, source, true)
	}
}

func filesystemIdentity(key string) string {
	if key == "root" || strings.HasPrefix(key, "extra:") {
		return "extra:" + key
	}
	return key
}
