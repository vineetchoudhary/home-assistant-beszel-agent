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

func metricValues(system System, sample *Sample) (map[string]string, time.Time, error) {
	values := map[string]string{}
	if sample == nil {
		return values, time.Time{}, nil
	}
	created, err := sampleTime(sample.Created)
	if err != nil {
		return nil, time.Time{}, err
	}
	core := map[string]*float64{"cpu_usage": sample.Stats.CPU, "memory_usage": sample.Stats.Memory, "disk_usage": sample.Stats.Disk}
	for key, value := range core {
		if value == nil || !validNumber(*value) || *value < 0 || *value > 100 {
			return nil, time.Time{}, fmt.Errorf("Hub sample has unsupported or invalid %s data", key)
		}
		values[key] = number(*value)
	}
	if system.Info.Uptime != nil {
		values["uptime"] = strconv.FormatUint(*system.Info.Uptime, 10)
	}
	values["last_sample"] = created.UTC().Format(time.RFC3339Nano)
	// Temperature exists only when an actual sensor map is present. The summary
	// dt field can be zero for machines without sensors, so it is not evidence.
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
	var sent, received float64
	hasNetwork := false
	if sample.Stats.Bandwidth != nil {
		if len(sample.Stats.Bandwidth) != 2 {
			return nil, time.Time{}, errors.New("invalid network bandwidth tuple")
		}
		sent = sample.Stats.Bandwidth[0]
		received = sample.Stats.Bandwidth[1]
		hasNetwork = true
	} else if len(sample.Stats.Interfaces) > 0 {
		// In the verified versions the aggregate b array is omitted when idle,
		// but ni still records [sent B/s, received B/s, total sent, total received].
		for _, network := range sample.Stats.Interfaces {
			if len(network) != 4 {
				return nil, time.Time{}, errors.New("invalid network interface tuple")
			}
			sent += network[0]
			received += network[1]
		}
		hasNetwork = true
	}
	if hasNetwork && validNumber(sent) && validNumber(received) && sent >= 0 && received >= 0 {
		values["network_transmit"] = number(sent)
		values["network_receive"] = number(received)
	}
	return values, created, nil
}
func validNumber(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }
func number(n float64) string    { return strconv.FormatFloat(n, 'f', -1, 64) }
func fresh(created, now time.Time, seconds int) bool {
	return !created.IsZero() && !created.After(now.Add(30*time.Second)) && now.Sub(created) <= time.Duration(seconds)*time.Second
}
