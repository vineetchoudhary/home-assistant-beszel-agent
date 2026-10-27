package main

import (
	"fmt"
	"sort"
)

type publisher interface {
	publish(topic string, payload any, retain bool) error
}

func (s *State) hostRoot(id string) string { return s.root() + "/system/" + id }
func (s *State) discoveryTopic(domain, id, key string) string {
	return fmt.Sprintf("%s/%s/%s_%s_%s/config", s.DiscoveryPrefix, domain, s.Namespace, id, key)
}
func (s *State) device(system System) map[string]any {
	return map[string]any{"identifiers": []string{s.Namespace + "_" + system.ID}, "name": system.Name,
		"manufacturer": "Beszel", "model": "Monitored server", "via_device": s.Namespace}
}

func (s *State) discovery(c Config, system System, key string) map[string]any {
	domain := "sensor"
	name := "System status"
	definition, metric := metricDefinitions[key]
	if metric {
		name = definition.Name
	}
	if key == "connectivity" {
		domain = "binary_sensor"
		name = "Connectivity"
	}
	id := s.Namespace + "_" + system.ID + "_" + key
	availability := []map[string]string{{"topic": s.root() + "/availability"}, {"topic": s.root() + "/hub/availability"}}
	availability = append(availability, map[string]string{"topic": s.hostRoot(system.ID) + "/" + key + "/availability"})
	payload := map[string]any{"name": name, "unique_id": id, "default_entity_id": domain + "." + id,
		"state_topic": s.hostRoot(system.ID) + "/" + key, "device": s.device(system), "availability": availability,
		"availability_mode": "all", "payload_available": "online", "payload_not_available": "offline",
		"origin": map[string]string{"name": "Beszel Bridge", "sw_version": version, "support_url": "https://github.com/vineetchoudhary/home-assistant-beszel-agent"}}
	if metric {
		payload["expire_after"] = c.StaleAfter
		if definition.Unit != "" {
			payload["unit_of_measurement"] = definition.Unit
		}
		if definition.DeviceClass != "" {
			payload["device_class"] = definition.DeviceClass
		}
		if definition.StateClass != "" {
			payload["state_class"] = definition.StateClass
		}
	} else if key == "connectivity" {
		payload["device_class"] = "connectivity"
		payload["payload_on"] = "ON"
		payload["payload_off"] = "OFF"
		payload["expire_after"] = c.StaleAfter
	} else {
		payload["device_class"] = "enum"
		payload["options"] = []string{"up", "down", "paused", "pending"}
		payload["expire_after"] = c.StaleAfter
	}
	return payload
}

func (s *State) removeEntity(p publisher, dir, topic string) error {
	if !s.owns(topic) {
		return fmt.Errorf("refusing to remove a discovery topic outside this Bridge namespace")
	}
	entity := s.Topics[topic]
	if err := p.publish(topic, "", true); err != nil {
		return err
	}
	if entity.System != "" {
		if err := p.publish(s.hostRoot(entity.System)+"/"+entity.Metric+"/availability", "", true); err != nil {
			return err
		}
	}
	delete(s.Topics, topic)
	return s.save(dir)
}

func sortedMetricKeys() []string {
	keys := make([]string, 0, len(metricDefinitions))
	for key := range metricDefinitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
