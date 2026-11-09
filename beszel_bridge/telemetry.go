package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Family selectors are separate from the original scalar metric keys, whose
// discovery identifiers and payloads remain unchanged.
var metricFamilies = map[string]bool{
	"cpu": true, "memory": true, "swap": true, "filesystems": true, "disk_io": true,
	"network": true, "temperatures": true, "fans": true, "gpus": true, "batteries": true,
	"wifi": true, "smart": true, "smart_attributes": true, "containers": true,
	"services": true, "storage_pools": true, "network_monitors": true,
	"package_updates": true, "alerts": true, "system_info": true, "uptime_human": true,
}

var legacyFamilies = map[string]string{
	"cpu_usage": "cpu", "memory_usage": "memory", "disk_usage": "filesystems",
	"temperature": "temperatures", "network_receive": "network", "network_transmit": "network",
	"uptime": "system_info", "last_sample": "system_info",
}

type rawObject map[string]json.RawMessage

func obj(raw json.RawMessage) rawObject {
	value, _ := jsonObject(raw)
	return value
}
func array(raw json.RawMessage) []json.RawMessage {
	var value []json.RawMessage
	_ = json.Unmarshal(raw, &value)
	return value
}
func (o rawObject) text(key string) string { return stringField(o, key) }
func (o rawObject) number(key string) (float64, bool) {
	v := floatField(o[key])
	if v == nil {
		return 0, false
	}
	return *v, true
}
func (o rawObject) uint(key string) (uint64, bool) {
	var v uint64
	if len(o[key]) == 0 || string(o[key]) == "null" || json.Unmarshal(o[key], &v) != nil {
		return 0, false
	}
	return v, true
}
func (o rawObject) boolean(key string) (bool, bool) {
	var v bool
	if len(o[key]) == 0 || string(o[key]) == "null" || json.Unmarshal(o[key], &v) != nil {
		return false, false
	}
	return v, true
}
func fieldNumber(raw json.RawMessage) (float64, bool) { return rawObject{"v": raw}.number("v") }
func fieldUint(raw json.RawMessage) (uint64, bool)    { return rawObject{"v": raw}.uint("v") }
func validNonnegative(v float64) bool                 { return validNumber(v) && v >= 0 }
func keys(o rawObject) []string {
	result := make([]string, 0, len(o))
	for key := range o {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func identityToken(identity string) string {
	hash := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(hash[:12])
}

// Store definitions, not readings, in the ownership manifest. This lets an
// unavailable entity be rediscovered after restart without replaying old data.
type entityDefinition struct {
	Name        string `json:"name"`
	Unit        string `json:"unit,omitempty"`
	DeviceClass string `json:"device_class,omitempty"`
	StateClass  string `json:"state_class,omitempty"`
	Domain      string `json:"domain,omitempty"`
	Diagnostic  bool   `json:"diagnostic,omitempty"`
	Enabled     bool   `json:"enabled"`
	Expiry      int    `json:"expiry"`
}

type reading struct {
	Key, Family, Component string
	Definition             entityDefinition
	Value                  string
	Attributes             map[string]any
	Source                 time.Time
	Snapshot               bool // Descriptive metadata and timestamps can explain stale readings.
}

type telemetry struct {
	cfg        Config
	system     System
	now        time.Time
	readings   map[string]reading
	components map[string]bool
	complete   map[string]bool
	issues     []error
	known      map[string]bool
}

func newTelemetry(c Config, system System, now time.Time) *telemetry {
	return &telemetry{cfg: c, system: system, now: now, readings: map[string]reading{}, components: map[string]bool{}, complete: map[string]bool{}, known: map[string]bool{}}
}

// Several Beszel fields use omitzero. Treat omission as idle zero only after
// that exact metric has demonstrated support, never on first discovery.
func (b *telemetry) omittedZero(o rawObject, field, key string) bool {
	_, present := o[field]
	return o != nil && !present && b.known[key]
}

func (b *telemetry) scope(family string, complete bool) {
	if previous, exists := b.complete[family]; exists {
		complete = complete && previous
	}
	b.complete[family] = complete
}

func (b *telemetry) issue(message string) {
	for _, err := range b.issues {
		if err.Error() == message {
			return
		}
	}
	b.issues = append(b.issues, errors.New(message))
}
func (b *telemetry) component(family, identity string) string {
	id := family + ":" + identity
	b.components[id] = true
	return id
}
func metricKey(family, identity, metric string) string {
	if identity == "" {
		return family + "_" + metric
	}
	if identity == "root" && family == "filesystems" {
		return "disk_" + metric
	}
	return family + "_" + identityToken(identity) + "_" + metric
}
func (b *telemetry) add(family, identity, metric, name, value, unit, device, state string, source time.Time, ttl int, enabled, snapshot bool, attributes map[string]any) {
	if !b.cfg.enabled(family) && family != "diagnostics" {
		return
	}
	if attributes == nil {
		attributes = map[string]any{}
	}
	attributes["source_timestamp"] = source.UTC().Format(time.RFC3339Nano)
	if utf8.RuneCountInString(value) > 255 {
		attributes["full_value"] = value
		value = string([]rune(value)[:255])
	}
	key := metricKey(family, identity, metric)
	if family == "uptime_human" {
		key = family
	}
	b.readings[key] = reading{Key: key, Family: family, Component: b.component(family, identity), Value: value,
		Definition: entityDefinition{Name: name, Unit: unit, DeviceClass: device, StateClass: state, Domain: "sensor", Enabled: enabled, Expiry: ttl, Diagnostic: family == "system_info" || family == "diagnostics"},
		Source:     source, Snapshot: snapshot, Attributes: attributes}
}
func (b *telemetry) gauge(family, identity, metric, name string, v float64, ok bool, unit, device string, source time.Time, ttl int, enabled bool, attributes map[string]any) {
	if !ok || !validNumber(v) {
		return
	}
	b.add(family, identity, metric, name, number(v), unit, device, "measurement", source, ttl, enabled, false, attributes)
}
func (b *telemetry) counter(family, identity, metric, name string, v uint64, ok bool, unit, device string, source time.Time, ttl int, attributes map[string]any) {
	if !ok {
		return
	}
	b.add(family, identity, metric, name, strconv.FormatUint(v, 10), unit, device, "total_increasing", source, ttl, false, false, attributes)
}
func (b *telemetry) text(family, identity, metric, name, value string, source time.Time, ttl int, enabled, snapshot bool, attributes map[string]any) {
	if value == "" {
		return
	}
	b.add(family, identity, metric, name, value, "", "", "", source, ttl, enabled, snapshot, attributes)
}
func (b *telemetry) timestamp(family, identity, metric, name string, source time.Time, enabled bool) {
	if source.IsZero() || source.After(b.now.Add(30*time.Second)) {
		return
	}
	b.add(family, identity, metric, name, source.UTC().Format(time.RFC3339Nano), "", "timestamp", "", source, b.cfg.StaleAfter, enabled, true, nil)
}
func (b *telemetry) flag(family, identity, metric, name string, value, ok bool, device string, source time.Time, ttl int, attrs map[string]any) {
	if !ok {
		return
	}
	v := "OFF"
	if value {
		v = "ON"
	}
	b.add(family, identity, metric, name, v, "", device, "", source, ttl, false, false, attrs)
	key := metricKey(family, identity, metric)
	if r, exists := b.readings[key]; exists {
		r.Definition.Domain = "binary_sensor"
		b.readings[key] = r
	}
}
func (b *telemetry) age(family, identity, name string, source time.Time, enabled bool) {
	if source.IsZero() || source.After(b.now.Add(30*time.Second)) {
		return
	}
	seconds := uint64(max(0, b.now.Sub(source).Seconds()))
	b.add(family, identity, "update_age", name+" update age", strconv.FormatUint(seconds, 10), "s", "duration", "measurement", source, b.cfg.StaleAfter, false, true, nil)
	b.text(family, identity, "updated_human", name+" updated", humanDuration(seconds)+" ago", source, b.cfg.StaleAfter, enabled, true, nil)
}

func humanDuration(seconds uint64) string {
	divisor, unit := uint64(1), "second"
	switch {
	case seconds >= 365*86400:
		divisor, unit = 365*86400, "year"
	case seconds >= 60*86400:
		divisor, unit = 30*86400, "month"
	case seconds >= 86400:
		divisor, unit = 86400, "day"
	case seconds >= 3600:
		divisor, unit = 3600, "hour"
	case seconds >= 60:
		divisor, unit = 60, "min"
	}
	value := seconds / divisor
	if value != 1 {
		unit += "s"
	}
	return fmt.Sprintf("%d %s", value, unit)
}

func recordTime(o rawObject, field string) time.Time {
	if t, err := sampleTime(o.text(field)); err == nil {
		return t
	}
	if v, ok := o.uint(field); ok && v > 0 && v <= math.MaxInt64 {
		return time.UnixMilli(int64(v)).UTC()
	}
	return time.Time{}
}

func cloneAttrs(attrs map[string]any) map[string]any {
	copy := make(map[string]any, len(attrs)+1)
	for k, v := range attrs {
		copy[k] = v
	}
	return copy
}

func metadata(o rawObject, fields ...string) map[string]any {
	attrs := map[string]any{}
	for _, field := range fields {
		if raw := o[field]; len(raw) > 0 && string(raw) != "null" {
			attrs[field] = raw
		}
	}
	return attrs
}

func nonnegative(o rawObject, field string) (float64, bool) {
	v, ok := o.number(field)
	return v, ok && validNonnegative(v)
}

func percentage(o rawObject, field string) (float64, bool) {
	v, ok := o.number(field)
	return v, ok && v >= 0 && v <= 100
}

func label(prefix, identity, metric string) string {
	return strings.TrimSpace(prefix + " " + identity + " " + metric)
}
