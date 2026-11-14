package main

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"
)

type metricDefinition struct{ Name, Unit, DeviceClass, StateClass string }

var metricDefinitions = map[string]metricDefinition{
	"cpu_usage":        {"CPU usage", "%", "", "measurement"},
	"memory_usage":     {"Memory usage", "%", "", "measurement"},
	"disk_usage":       {"Disk usage", "%", "", "measurement"},
	"temperature":      {"Temperature", "°C", "temperature", "measurement"},
	"network_receive":  {"Network receive", "B/s", "data_rate", "measurement"},
	"network_transmit": {"Network transmit", "B/s", "data_rate", "measurement"},
	"uptime":           {"Uptime", "s", "duration", "measurement"},
	"last_sample":      {"Last sample", "", "timestamp", ""},
}

func sampleTime(raw string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999Z"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("Hub sample has no valid source timestamp")
}

// Return every independently valid reading alongside any data issues. A caller
// must still enforce the shared source timestamp before publishing readings.
func metricValues(system System, sample *Sample) (map[string]string, time.Time, error) {
	values := map[string]string{}
	if sample == nil {
		return values, time.Time{}, nil
	}
	created, err := sampleTime(sample.Created)
	if err != nil {
		return values, time.Time{}, err
	}
	var issues []error
	for _, issue := range sample.Issues {
		issues = append(issues, errors.New(issue))
	}
	core := map[string]*float64{"cpu_usage": sample.Stats.CPU, "memory_usage": sample.Stats.Memory, "disk_usage": sample.Stats.Disk}
	for _, key := range []string{"cpu_usage", "memory_usage", "disk_usage"} {
		value := core[key]
		if value == nil || !validNumber(*value) || *value < 0 || *value > 100 {
			issues = append(issues, fmt.Errorf("missing or invalid %s", key))
			continue
		}
		values[key] = roundedNumber(*value)
	}
	if system.Info.Uptime != nil {
		values["uptime"] = strconv.FormatUint(*system.Info.Uptime, 10)
	}
	values["last_sample"] = created.UTC().Format(time.RFC3339Nano)
	// A real sensor map is evidence of temperature capability. Summary dt=0 is not.
	hasTemp := false
	var highest float64
	for _, temperature := range sample.Stats.Temperatures {
		if validNumber(temperature) && (!hasTemp || temperature > highest) {
			highest = temperature
			hasTemp = true
		}
	}
	if hasTemp {
		values["temperature"] = number(highest)
	}
	for index, key := range []string{"network_transmit", "network_receive"} {
		var rate float64
		valid := true
		switch {
		case sample.Stats.Bandwidth != nil:
			rate, valid = tupleRate(sample.Stats.Bandwidth, index)
		case len(sample.Stats.Interfaces) > 0:
			// ni starts with [sent B/s, received B/s]. Ignore additional counters.
			for _, network := range sample.Stats.Interfaces {
				value, ok := tupleRate(network, index)
				rate += value
				valid = valid && ok
			}
		default:
			continue
		}
		if valid && validNumber(rate) {
			values[key] = number(rate)
		} else {
			issues = append(issues, fmt.Errorf("missing or invalid %s", key))
		}
	}
	return values, created, errors.Join(issues...)
}

func tupleRate(values []float64, index int) (float64, bool) {
	if len(values) <= index || !validNumber(values[index]) || values[index] < 0 {
		return 0, false
	}
	return values[index], true
}
func validNumber(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }
func number(n float64) string    { return strconv.FormatFloat(n, 'f', -1, 64) }
func roundedNumber(n float64) string {
	// Float64 values this large already have no fractional part. Avoid
	// overflowing otherwise valid, unbounded per-service CPU percentages.
	if math.Abs(n) >= 1<<52 {
		return number(n)
	}
	n = math.Round(n*100) / 100
	if n == 0 {
		return "0"
	}
	return number(n)
}
func fresh(created, now time.Time, seconds int) bool {
	return !created.IsZero() && !created.After(now.Add(30*time.Second)) && now.Sub(created) <= time.Duration(seconds)*time.Second
}
