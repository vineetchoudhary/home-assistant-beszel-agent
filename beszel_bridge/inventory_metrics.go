package main

import (
	"math"
	"strconv"
	"time"
)

func (b *telemetry) monitorMetrics(result collectionResult, probes map[string]collectionResult, prefix string) {
	b.scope("network_monitors", result.Complete)
	for _, monitor := range result.Rows {
		id := monitor.text("id")
		b.component("network_monitors", id)
		name := monitor.text("target")
		if name == "" {
			name = id
		}
		label := "Probe " + name
		attrs := metadata(monitor, "target", "protocol", "port", "server", "interval", "enabled", "skipTlsVerify")
		attrs["record_id"] = id
		interval, _ := monitor.uint("interval")
		ttl := max(b.cfg.StaleAfter, int(min(interval, uint64(86400)))*3)
		configured := recordTime(monitor, "updated")
		enabled, hasEnabled := monitor.boolean("enabled")
		b.flag("network_monitors", id, "enabled", label+" enabled", enabled, hasEnabled, "", configured, ttl, cloneAttrs(attrs))
		key := metricKey("network_monitors", id, "enabled")
		if r, ok := b.readings[key]; ok {
			r.Snapshot = true
			b.readings[key] = r
		}
		probe := probes[prefix+"probe:"+id]
		if probe.Err != nil && !optionalUnsupported(probe.Err) {
			b.issues = append(b.issues, probe.Err)
		}
		var row rawObject
		var source time.Time
		if len(probe.Rows) > 0 && probe.Rows[0].text("monitor") == id && probe.Rows[0].text("type") == "1m" {
			row = probe.Rows[0]
			source = recordTime(row, "created")
		}
		b.timestamp("network_monitors", id, "last_sample", label+" last sample", source, false)
		if !enabled || !hasEnabled {
			continue
		}
		total, tok := row.uint("total_count")
		success, sok := row.uint("success_count")
		sum, sumOK := row.uint("res_sum")
		valid := tok && sok && success <= total && total > 0
		if valid {
			b.add("network_monitors", id, "probe_count", label+" probe count", strconv.FormatUint(total, 10), "", "", "measurement", source, ttl, false, false, cloneAttrs(attrs))
			b.add("network_monitors", id, "success_count", label+" success count", strconv.FormatUint(success, 10), "", "", "measurement", source, ttl, false, false, nil)
			b.gauge("network_monitors", id, "packet_loss", label+" packet loss", 100*float64(total-success)/float64(total), true, "%", "", source, ttl, false, nil)
		}
		if valid && success > 0 {
			if sumOK {
				b.gauge("network_monitors", id, "response_average", label+" average response", float64(sum)/float64(success)/1000, true, "ms", "duration", source, ttl, false, nil)
			}
			for _, entry := range []struct{ field, key, name string }{{"res_min", "response_minimum", "minimum response"}, {"res_max", "response_maximum", "maximum response"}} {
				v, ok := nonnegative(row, entry.field)
				b.gauge("network_monitors", id, entry.key, label+" "+entry.name, v/1000, ok, "ms", "duration", source, ttl, false, nil)
			}
		}
		if valid && sumOK {
			b.gauge("network_monitors", id, "response_sum", label+" response sum", float64(sum)/1000, true, "ms", "duration", source, ttl, false, nil)
		}
		loss, lossOK := percentage(monitor, "loss1h")
		b.gauge("network_monitors", id, "packet_loss_1h", label+" packet loss 1h", loss, lossOK, "%", "", source, ttl, false, nil)
		if lossOK && loss < 100 {
			for _, entry := range []struct{ field, key, name string }{{"resAvg1h", "response_average_1h", "average response 1h"}, {"resMin1h", "response_minimum_1h", "minimum response 1h"}, {"resMax1h", "response_maximum_1h", "maximum response 1h"}} {
				v, ok := nonnegative(monitor, entry.field)
				b.gauge("network_monitors", id, entry.key, label+" "+entry.name, v/1000, ok, "ms", "duration", source, ttl, false, nil)
			}
		}
		cert := obj(monitor["certInfo"])
		if expiry, ok := cert.uint("expires"); ok && expiry > 0 && expiry <= math.MaxInt64 {
			expires := time.UnixMilli(int64(expiry)).UTC()
			// This timestamp is a future expiry, not the measurement's source time.
			b.add("network_monitors", id, "certificate_expiry", label+" certificate expiry", expires.Format(time.RFC3339Nano), "", "timestamp", "", source, ttl, false, true, metadata(cert, "issuer"))
			b.gauge("network_monitors", id, "certificate_days_remaining", label+" certificate days remaining", math.Floor(expires.Sub(b.now).Hours()/24), true, "d", "duration", source, ttl, false, metadata(cert, "issuer"))
		}
	}
}

func (b *telemetry) packageMetrics(result collectionResult) {
	// The info tuple supplies counts even if a Hub lacks the detailed endpoint.
	if len(result.Rows) == 0 {
		return
	}
	row := result.Rows[0]
	packages := array(row["packages"])
	checked, checkedOK := row.uint("checkedAt")
	if !checkedOK || checked == 0 || checked > math.MaxInt64 {
		return
	}
	source := time.Unix(int64(checked), 0).UTC()
	ttl := b.cfg.detailExpiry()
	securityKnown, _ := row.boolean("securityKnown")
	if string(row["packages"]) == "null" || packages == nil {
		return
	}
	clean := []map[string]any{}
	security := uint64(0)
	for _, raw := range packages {
		pkg := obj(raw)
		if pkg == nil || pkg.text("name") == "" || pkg.text("available") == "" {
			return
		}
		clean = append(clean, metadata(pkg, "name", "current", "available", "security"))
		if value, ok := pkg.boolean("security"); ok && value {
			security++
		}
	}
	attrs := map[string]any{"manager": row.text("manager"), "security_known": securityKnown, "packages": clean}
	b.add("package_updates", "", "count_total", "Package updates total", strconv.Itoa(len(packages)), "", "", "measurement", source, ttl, false, false, attrs)
	if securityKnown {
		b.add("package_updates", "", "count_security", "Package updates security", strconv.FormatUint(security, 10), "", "", "measurement", source, ttl, false, false, nil)
	} else {
		delete(b.readings, metricKey("package_updates", "", "count_security"))
	}
	b.flag("package_updates", "", "available", "Package updates available", len(packages) > 0, true, "update", source, ttl, nil)
	b.timestamp("package_updates", "", "last_check", "Package updates last check", source, false)
	b.age("package_updates", "", "Package updates", source, false)
}

func (b *telemetry) alertMetrics(result, history collectionResult) {
	b.scope("alerts", result.Complete)
	count := 0
	valid := result.Complete
	for _, alert := range result.Rows {
		id := alert.text("id")
		b.component("alerts", id)
		source := recordTime(alert, "updated")
		attrs := metadata(alert, "name", "value", "min", "state", "pending_since")
		attrs["record_id"] = id
		triggered, ok := alert.boolean("triggered")
		valid = valid && ok
		if triggered && ok {
			count++
		}
		b.flag("alerts", id, "triggered", "Alert "+alert.text("name"), triggered, ok, "problem", source, b.cfg.StaleAfter, attrs)
		// Alert state records change only when the alert changes. A successful
		// inventory confirms that this remains the account's current alert state.
		key := metricKey("alerts", id, "triggered")
		if r, exists := b.readings[key]; exists {
			r.Snapshot = true
			b.readings[key] = r
		}
	}
	if valid {
		b.add("alerts", "", "active_count", "Active alerts", strconv.Itoa(count), "", "", "measurement", b.now, b.cfg.StaleAfter, false, true, map[string]any{"visibility": "bridge account"})
	}
	if len(history.Rows) == 0 {
		return
	}
	event := history.Rows[0]
	source := recordTime(event, "created")
	b.timestamp("alerts", "", "last_event", "Latest alert event", source, false)
	key := metricKey("alerts", "", "last_event")
	if r, ok := b.readings[key]; ok {
		r.Attributes = metadata(event, "name", "monitor_name", "value", "alert_id", "resolved")
		r.Attributes["source_timestamp"] = source.UTC().Format(time.RFC3339Nano)
		b.readings[key] = r
	}
	b.timestamp("alerts", "", "last_resolved", "Latest alert event resolved", recordTime(event, "resolved"), false)
}
