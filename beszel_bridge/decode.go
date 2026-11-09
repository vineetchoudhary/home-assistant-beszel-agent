package main

import (
	"encoding/json"
	"errors"
	"math"
)

// Decode known fields independently so changed optional fields cannot invalidate
// unrelated readings. Unknown fields are left to the Hub.
func jsonObject(data []byte) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return nil, errors.New("Hub record must be a JSON object")
	}
	return object, nil
}

func stringField(object map[string]json.RawMessage, field string) string {
	var value string
	_ = json.Unmarshal(object[field], &value)
	return value
}

func floatField(raw json.RawMessage) *float64 {
	var value *float64
	if json.Unmarshal(raw, &value) != nil || value == nil || !validNumber(*value) {
		return nil
	}
	return value
}

func (info *HubInfo) UnmarshalJSON(data []byte) error {
	object, err := jsonObject(data)
	if err != nil {
		return err
	}
	*info = HubInfo{Key: stringField(object, "key"), Version: stringField(object, "v")}
	// Version is diagnostic metadata, never a runtime compatibility gate.
	if len(info.Version) > 128 || !releaseVersion.MatchString(info.Version) {
		info.Version = "unknown"
	}
	return nil
}

func (system *System) UnmarshalJSON(data []byte) error {
	object, err := jsonObject(data)
	if err != nil {
		return err
	}
	*system = System{ID: stringField(object, "id"), Name: stringField(object, "name"), Status: stringField(object, "status"), Updated: stringField(object, "updated")}
	system.Raw = object
	if system.Name == "" {
		system.Name = system.ID
	}
	if raw, exists := object["info"]; exists {
		info, err := jsonObject(raw)
		if err != nil {
			system.Issues = append(system.Issues, "invalid system info")
			return nil
		}
		system.Info.Version = stringField(info, "v")
		if raw, exists := info["u"]; exists && string(raw) != "null" {
			if json.Unmarshal(raw, &system.Info.Uptime) != nil {
				system.Info.Uptime = nil
				system.Issues = append(system.Issues, "invalid uptime")
			}
		}
	}
	return nil
}

func numberTuple(raw json.RawMessage) ([]float64, bool) {
	var fields []json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil, false
	}
	values := make([]float64, len(fields))
	valid := len(fields) >= 2
	for i, field := range fields {
		values[i] = math.NaN()
		if number := floatField(field); number != nil {
			values[i] = *number
		} else if i < 2 {
			valid = false
		}
	}
	return values, valid
}

func (sample *Sample) UnmarshalJSON(data []byte) error {
	object, err := jsonObject(data)
	if err != nil {
		return err
	}
	*sample = Sample{ID: stringField(object, "id"), System: stringField(object, "system"), Type: stringField(object, "type"), Created: stringField(object, "created")}
	stats, err := jsonObject(object["stats"])
	if err != nil {
		sample.Issues = append(sample.Issues, "missing or invalid stats")
		return nil
	}
	sample.RawStats = stats
	sample.Stats.CPU = floatField(stats["cpu"])
	sample.Stats.Memory = floatField(stats["mp"])
	sample.Stats.Disk = floatField(stats["dp"])
	if raw, exists := stats["b"]; exists && string(raw) != "null" {
		var valid bool
		sample.Stats.Bandwidth, valid = numberTuple(raw)
		if !valid {
			sample.Issues = append(sample.Issues, "invalid bandwidth reading")
		}
	}
	if raw, exists := stats["ni"]; exists && string(raw) != "null" {
		interfaces, err := jsonObject(raw)
		if err != nil {
			sample.Issues = append(sample.Issues, "invalid network interfaces")
		} else {
			sample.Stats.Interfaces = map[string][]float64{}
			for name, raw := range interfaces {
				values, valid := numberTuple(raw)
				sample.Stats.Interfaces[name] = values
				if !valid {
					sample.Issues = append(sample.Issues, "invalid network interface reading")
				}
			}
		}
	}
	if raw, exists := stats["t"]; exists && string(raw) != "null" {
		temperatures, err := jsonObject(raw)
		if err != nil {
			sample.Issues = append(sample.Issues, "invalid temperature sensors")
		} else {
			sample.Stats.Temperatures = map[string]float64{}
			for name, raw := range temperatures {
				if value := floatField(raw); value != nil {
					sample.Stats.Temperatures[name] = *value
				} else {
					sample.Issues = append(sample.Issues, "invalid temperature reading")
				}
			}
		}
	}
	return nil
}

type recordsPage struct {
	page[json.RawMessage]
	Valid bool
}

func decodeRecordsPage(data []byte) (recordsPage, error) {
	var result recordsPage
	object, err := jsonObject(data)
	if err != nil {
		return result, err
	}
	if json.Unmarshal(object["items"], &result.Items) != nil || result.Items == nil {
		return result, errors.New("Hub response has no valid records array")
	}
	result.Valid = true
	for field, target := range map[string]*int{"page": &result.Page, "totalPages": &result.TotalPages, "totalItems": &result.TotalItems} {
		var value *int
		if json.Unmarshal(object[field], &value) != nil || value == nil || *value < 0 {
			result.Valid = false
		} else {
			*target = *value
		}
	}
	return result, nil
}
