package main

import (
	"encoding/json"
	"reflect"
	"sort"
	"time"
)

func (s *State) telemetryDiscovery(c Config, system System, entity ownedEntity) map[string]any {
	d := entity.Definition
	id := s.Namespace + "_" + system.ID + "_" + entity.Metric
	root := s.hostRoot(system.ID) + "/" + entity.Metric
	payload := map[string]any{
		"name": d.Name, "unique_id": id, "default_entity_id": d.Domain + "." + id,
		"state_topic": root, "json_attributes_topic": root + "/attributes", "device": s.device(system),
		"enabled_by_default": c.EnableAllEntities || d.Enabled, "expire_after": d.Expiry,
		"availability":      []map[string]string{{"topic": s.root() + "/availability"}, {"topic": s.root() + "/hub/availability"}, {"topic": root + "/availability"}},
		"availability_mode": "all", "payload_available": "online", "payload_not_available": "offline",
		"origin": map[string]string{"name": "Beszel Bridge", "sw_version": version, "support_url": "https://github.com/vineetchoudhary/home-assistant-beszel-agent"},
	}
	if d.Unit != "" {
		payload["unit_of_measurement"] = d.Unit
	}
	if d.DeviceClass != "" {
		payload["device_class"] = d.DeviceClass
	}
	if d.StateClass != "" && d.Domain == "sensor" {
		payload["state_class"] = d.StateClass
	}
	if d.Diagnostic {
		payload["entity_category"] = "diagnostic"
	}
	if d.Domain == "binary_sensor" {
		payload["payload_on"] = "ON"
		payload["payload_off"] = "OFF"
	}
	return payload
}

func (s *service) publishTelemetry(system System, b *telemetry) error {
	if s.state.ComponentMissing == nil {
		s.state.ComponentMissing = map[string]time.Time{}
	}
	dirty := false
	// Persist ownership of the entire new batch before publishing any retained
	// discovery. This avoids both orphan topics and one flash write per sensor.
	for _, r := range b.readings {
		definition := r.Definition
		entity := ownedEntity{System: system.ID, Metric: r.Key, Family: r.Family, Component: r.Component, Definition: &definition}
		topic := s.state.discoveryTopic(definition.Domain, system.ID, r.Key)
		if previous, exists := s.state.Topics[topic]; !exists || !reflect.DeepEqual(previous, entity) {
			s.state.Topics[topic] = entity
			dirty = true
		}
	}
	if dirty {
		if err := s.state.save(s.dir); err != nil {
			return err
		}
	}
	topics := []string{}
	for topic, entity := range s.state.Topics {
		if entity.System == system.ID && entity.Definition != nil {
			topics = append(topics, topic)
		}
	}
	sort.Strings(topics)
	for _, topic := range topics {
		entity := s.state.Topics[topic]
		if !s.cfg.enabled(entity.Family) && entity.Family != "diagnostics" {
			continue
		}
		if err := s.configMessage(topic, s.state.telemetryDiscovery(s.cfg, system, entity), entity); err != nil {
			return err
		}
		root := s.state.hostRoot(system.ID) + "/" + entity.Metric
		r, present := b.readings[entity.Metric]
		available := "offline"
		validTime := !r.Source.IsZero() && !r.Source.After(b.now.Add(30*time.Second))
		validStatus := system.Status == "up" || !(system.Status == "down" || system.Status == "paused" || system.Status == "pending")
		if present && validTime && (r.Snapshot || (validStatus && fresh(r.Source, b.now, r.Definition.Expiry))) {
			data, err := json.Marshal(r.Attributes)
			if err != nil {
				return err
			}
			if err = s.pub.publish(root+"/attributes", data, false); err != nil {
				return err
			}
			if err = s.pub.publish(root, r.Value, false); err != nil {
				return err
			}
			available = "online"
		}
		if err := s.pub.publish(root+"/availability", available, true); err != nil {
			return err
		}
	}
	return s.reconcileComponents(system.ID, b)
}

func (s *service) reconcileComponents(system string, b *telemetry) error {
	components := map[string]string{}
	for _, entity := range s.state.Topics {
		if entity.System == system && entity.Component != "" {
			components[entity.Component] = entity.Family
		}
	}
	remove := map[string]bool{}
	for component, family := range components {
		key := system + "/" + component
		if b.components[component] || !b.complete[family] {
			delete(s.state.ComponentMissing, key)
			continue
		}
		since, exists := s.state.ComponentMissing[key]
		if !exists {
			since = b.now
			s.state.ComponentMissing[key] = since
		}
		remove[component] = b.now.Sub(since) >= 5*time.Minute
	}
	for topic, entity := range s.state.Topics {
		if entity.System == system && remove[entity.Component] {
			if err := s.state.removeEntity(s.pub, s.dir, topic); err != nil {
				return err
			}
			delete(s.published, topic)
		}
	}
	for component, removed := range remove {
		if removed {
			delete(s.state.ComponentMissing, system+"/"+component)
		}
	}
	return s.state.save(s.dir)
}
