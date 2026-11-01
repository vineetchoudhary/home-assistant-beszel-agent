package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type healthStatus struct {
	Alive               bool   `json:"alive"`
	HubConnected        bool   `json:"hub_connected"`
	MQTTConnected       bool   `json:"mqtt_connected"`
	Systems             int    `json:"systems"`
	LastSync            string `json:"last_sync,omitempty"`
	Error               string `json:"error,omitempty"`
	DegradedSystems     int    `json:"degraded_systems,omitempty"`
	InventoryIncomplete bool   `json:"inventory_incomplete,omitempty"`
}
type health struct {
	mu     sync.Mutex
	status healthStatus
}

func (h *health) update(f func(*healthStatus)) { h.mu.Lock(); defer h.mu.Unlock(); f(&h.status) }
func (h *health) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path != "/health" && r.URL.Path != "/status" {
		http.NotFound(w, r)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	_ = json.NewEncoder(w).Encode(h.status)
}

type service struct {
	cfg       Config
	dir       string
	hub       *Hub
	state     *State
	pub       publisher
	health    *health
	now       func() time.Time
	published map[string]string
	warnings  map[string]string
}

func (s *service) diagnostics(info HubInfo) error {
	device := map[string]any{"identifiers": []string{s.state.Namespace}, "name": "Beszel Bridge", "manufacturer": "Beszel", "model": "Home Assistant MQTT bridge", "sw_version": version}
	for _, key := range []string{"bridge_connected", "hub_connected", "last_sync", "hub_version"} {
		domain := "sensor"
		name := map[string]string{"bridge_connected": "Bridge connected", "hub_connected": "Hub connected", "last_sync": "Last successful sync", "hub_version": "Hub version"}[key]
		stateTopic := s.state.root() + "/" + key
		if key == "bridge_connected" {
			domain = "binary_sensor"
			stateTopic = s.state.root() + "/availability"
		}
		if key == "hub_connected" {
			domain = "binary_sensor"
			stateTopic = s.state.root() + "/hub/availability"
		}
		topic := s.state.discoveryTopic(domain, "bridge", key)
		payload := map[string]any{"name": name, "unique_id": s.state.Namespace + "_bridge_" + key, "default_entity_id": domain + "." + s.state.Namespace + "_bridge_" + key,
			"device": device, "entity_category": "diagnostic", "state_topic": stateTopic}
		if domain == "binary_sensor" {
			payload["device_class"] = "connectivity"
			payload["payload_on"] = "online"
			payload["payload_off"] = "offline"
		}
		if key != "bridge_connected" {
			payload["availability_topic"] = s.state.root() + "/availability"
		}
		if key == "last_sync" {
			payload["device_class"] = "timestamp"
		}
		if err := s.configMessage(topic, payload, ownedEntity{Metric: key}); err != nil {
			return err
		}
	}
	return s.pub.publish(s.state.root()+"/hub_version", info.Version, false)
}

func (s *service) configMessage(topic string, payload any, entity ownedEntity) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if s.published[topic] == string(data) {
		return nil
	}
	if _, ok := s.state.Topics[topic]; !ok {
		s.state.Topics[topic] = entity
		if err = s.state.save(s.dir); err != nil {
			return err
		}
	}
	if err = s.pub.publish(topic, data, true); err != nil {
		return err
	}
	s.published[topic] = string(data)
	return nil
}

func (s *service) sync(ctx context.Context) error {
	info, err := s.hub.info(ctx)
	if err != nil {
		return err
	}
	if err = s.state.initialize(info, s.cfg.DiscoveryPrefix, s.state.Broker); err != nil {
		return err
	}
	inventory, err := s.hub.systems(ctx)
	if err != nil {
		return err
	}
	if err = s.diagnostics(info); err != nil {
		return err
	}
	present := map[string]bool{}
	count, degraded := 0, 0
	now := s.now()
	for _, system := range inventory.Systems {
		if err = ctx.Err(); err != nil {
			return err
		}
		present[system.ID] = true
		if !s.cfg.selected(system.ID) {
			continue
		}
		count++
		err = s.syncSystem(ctx, system)
		var dataErr *readingError
		if err != nil && !errors.As(err, &dataErr) {
			return err
		}
		if dataErr != nil {
			degraded++
		}
		s.noteIssue(system.ID, err)
	}
	var inventoryErr error
	if !inventory.Complete {
		inventoryErr = errors.New(strings.Join(inventory.Issues, "\n"))
	}
	s.noteIssue("inventory", inventoryErr)
	if err = s.reconcile(present, now, inventory.Complete); err != nil {
		return err
	}
	if err = s.pub.publish(s.state.root()+"/hub/availability", "online", true); err != nil {
		return err
	}
	if err = s.pub.publish(s.state.root()+"/last_sync", now.UTC().Format(time.RFC3339), false); err != nil {
		return err
	}
	s.health.mu.Lock()
	announce := s.health.status.LastSync == "" || !s.health.status.HubConnected || s.health.status.Systems != count || s.health.status.DegradedSystems != degraded
	s.health.mu.Unlock()
	if announce {
		slog.Info("Bridge synchronized", "systems", count, "hub_version", info.Version)
	}
	s.health.update(func(h *healthStatus) {
		h.HubConnected = true
		h.MQTTConnected = true
		h.Systems = count
		h.LastSync = now.UTC().Format(time.RFC3339)
		h.DegradedSystems = degraded
		h.InventoryIncomplete = !inventory.Complete
		h.Error = ""
		if degraded > 0 {
			h.Error = fmt.Sprintf("%d systems have unavailable or invalid readings", degraded)
		}
		if !inventory.Complete {
			h.Error = strings.TrimSpace(h.Error + " incomplete Hub inventory")
		}
	})
	for id := range s.warnings {
		if id != "inventory" && !present[id] && inventory.Complete {
			delete(s.warnings, id)
		}
	}
	return nil
}

type readingError struct{ error }

// Only log a changed issue set or a recovery, without API bodies or raw values.
func (s *service) noteIssue(scope string, err error) {
	if s.warnings == nil {
		s.warnings = map[string]string{}
	}
	if err == nil {
		if _, exists := s.warnings[scope]; exists {
			delete(s.warnings, scope)
			slog.Info("Hub data recovered", "scope", scope)
		}
		return
	}
	unique := map[string]bool{}
	for _, issue := range strings.Split(err.Error(), "\n") {
		unique[issue] = true
	}
	issues := make([]string, 0, len(unique))
	for issue := range unique {
		issues = append(issues, issue)
	}
	sort.Strings(issues)
	message := strings.Join(issues, "; ")
	if s.warnings[scope] != message {
		s.warnings[scope] = message
		slog.Warn("Some Hub data is unavailable", "scope", scope, "reason", message)
	}
}

func (s *service) syncSystem(ctx context.Context, system System) error {
	for _, key := range []string{"system_status", "connectivity"} {
		domain := "sensor"
		if key == "connectivity" {
			domain = "binary_sensor"
		}
		if err := s.configMessage(s.state.discoveryTopic(domain, system.ID, key), s.state.discovery(s.cfg, system, key), ownedEntity{System: system.ID, Metric: key}); err != nil {
			return err
		}
	}
	var issues []error
	for _, issue := range system.Issues {
		issues = append(issues, errors.New(issue))
	}
	validStatus := system.Status == "up" || system.Status == "down" || system.Status == "paused" || system.Status == "pending"
	statusAvailability := "offline"
	if validStatus {
		if err := s.pub.publish(s.state.hostRoot(system.ID)+"/system_status", system.Status, false); err != nil {
			return err
		}
		statusAvailability = "online"
	} else {
		issues = append(issues, errors.New("missing or unsupported system status"))
	}
	if err := s.pub.publish(s.state.hostRoot(system.ID)+"/system_status/availability", statusAvailability, true); err != nil {
		return err
	}
	sample, err := s.hub.latestSample(ctx, system.ID)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		issues = append(issues, err)
	}
	values, created, valueErr := metricValues(system, sample)
	if valueErr != nil {
		issues = append(issues, valueErr)
	}
	isFresh := (system.Status == "up" || !validStatus) && fresh(created, s.now(), s.cfg.StaleAfter)
	connection := ""
	if system.Status == "up" && isFresh {
		connection = "ON"
	} else if system.Status == "down" {
		connection = "OFF"
	}
	connectivityAvailability := "offline"
	if connection != "" {
		if err = s.pub.publish(s.state.hostRoot(system.ID)+"/connectivity", connection, false); err != nil {
			return err
		}
		connectivityAvailability = "online"
	}
	if err = s.pub.publish(s.state.hostRoot(system.ID)+"/connectivity/availability", connectivityAvailability, true); err != nil {
		return err
	}
	for _, key := range sortedMetricKeys() {
		if !s.cfg.enabled(key) {
			continue
		}
		value, exists := values[key]
		topic := s.state.discoveryTopic("sensor", system.ID, key)
		// Optional readings are discovered only after actual capability evidence.
		_, known := s.state.Topics[topic]
		if !exists && !known {
			continue
		}
		if err = s.configMessage(topic, s.state.discovery(s.cfg, system, key), ownedEntity{System: system.ID, Metric: key}); err != nil {
			return err
		}
		available := "offline"
		if exists && isFresh {
			if err = s.pub.publish(s.state.hostRoot(system.ID)+"/"+key, value, false); err != nil {
				return err
			}
			available = "online"
		}
		if err = s.pub.publish(s.state.hostRoot(system.ID)+"/"+key+"/availability", available, true); err != nil {
			return err
		}
	}
	if issue := errors.Join(issues...); issue != nil {
		return &readingError{issue}
	}
	return nil
}

func (s *service) reconcile(present map[string]bool, now time.Time, complete bool) error {
	for topic, entity := range s.state.Topics {
		if entity.System == "" {
			continue
		}
		remove := !s.cfg.selected(entity.System) || (entity.Metric != "system_status" && entity.Metric != "connectivity" && !s.cfg.enabled(entity.Metric))
		if present[entity.System] {
			delete(s.state.MissingSince, entity.System)
		} else {
			since, ok := s.state.MissingSince[entity.System]
			if !complete {
				delete(s.state.MissingSince, entity.System)
			} else {
				if !ok {
					since = now
					s.state.MissingSince[entity.System] = now
				}
				remove = remove || now.Sub(since) >= 5*time.Minute
			}
			if err := s.pub.publish(s.state.hostRoot(entity.System)+"/"+entity.Metric+"/availability", "offline", true); err != nil {
				return err
			}
		}
		if remove {
			if err := s.state.removeEntity(s.pub, s.dir, topic); err != nil {
				return err
			}
			delete(s.published, topic)
		}
	}
	return s.state.save(s.dir)
}

func run(ctx context.Context, c Config, dir, address string) error {
	h := &health{status: healthStatus{Alive: true}}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return errors.New("cannot bind Bridge health endpoint")
	}
	server := &http.Server{Handler: http.HandlerFunc(h.serve), ReadHeaderTimeout: 5 * time.Second}
	defer server.Close()
	go func() { _ = server.Serve(listener) }()
	hub, err := newHub(c)
	if err != nil {
		return err
	}
	state, err := loadState(dir)
	if err != nil {
		return err
	}
	slog.Info("Starting Beszel Bridge", "version", version)
	backoff := 5 * time.Second
	for ctx.Err() == nil {
		info, attemptErr := hub.info(ctx)
		var mqttConfig Config
		if attemptErr == nil {
			mqttConfig, attemptErr = resolveMQTT(ctx, c)
		}
		if attemptErr == nil {
			attemptErr = state.initialize(info, c.DiscoveryPrefix, brokerAddress(mqttConfig))
		}
		if attemptErr == nil {
			attemptErr = state.save(dir)
		}
		if attemptErr != nil {
			h.update(func(status *healthStatus) { status.Error = attemptErr.Error() })
			slog.Warn("Bridge setup is waiting", "reason", attemptErr)
			if !pause(ctx, backoff) {
				break
			}
			backoff = min(5*time.Minute, backoff*2)
			continue
		}
		events := make(chan bool, 1)
		client, attemptErr := connectMQTT(mqttConfig, state, events)
		if attemptErr != nil {
			h.update(func(status *healthStatus) { status.Error = attemptErr.Error() })
			slog.Warn("Bridge setup is waiting", "reason", attemptErr)
			if !pause(ctx, backoff) {
				break
			}
			backoff = min(5*time.Minute, backoff*2)
			continue
		}
		backoff = 5 * time.Second
		pub := mqttPublisher{client: client}
		s := &service{cfg: c, dir: dir, hub: hub, state: state, pub: pub, health: h, now: time.Now, published: map[string]string{}}
		ticker := time.NewTicker(time.Duration(c.PollInterval) * time.Second)
		loop := true
		for loop {
			select {
			case <-ctx.Done():
				loop = false
			case <-events:
				// Events coalesce while a sync is running. Read the current
				// connection so an older disconnect cannot hide a reconnect.
				online := client.IsConnectionOpen()
				h.update(func(status *healthStatus) { status.MQTTConnected = online })
				if !online {
					continue
				}
				s.published = map[string]string{}
				_ = pub.publish(state.root()+"/hub/availability", "offline", true)
				_ = pub.publish(state.root()+"/availability", "online", true)
				s.poll(ctx)
			case <-ticker.C:
				if client.IsConnectionOpen() {
					h.update(func(status *healthStatus) { status.MQTTConnected = true })
					_ = pub.publish(state.root()+"/availability", "online", true)
					s.poll(ctx)
				}
			}
		}
		ticker.Stop()
		_ = pub.publish(state.root()+"/availability", "offline", true)
		client.Disconnect(250)
	}
	return nil
}
func (s *service) poll(ctx context.Context) {
	if err := s.sync(ctx); err != nil {
		// Failed inventories cannot count toward automatic discovery removal.
		if len(s.state.MissingSince) > 0 {
			s.state.MissingSince = map[string]time.Time{}
			if saveErr := s.state.save(s.dir); saveErr != nil {
				slog.Warn("Cannot reset inventory removal grace", "reason", saveErr)
			}
		}
		_ = s.pub.publish(s.state.root()+"/hub/availability", "offline", true)
		s.health.update(func(h *healthStatus) { h.HubConnected = false; h.Error = err.Error() })
		if ctx.Err() == nil {
			slog.Warn("Bridge sync is waiting", "reason", err)
		}
	}
}
func pause(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func cleanup(ctx context.Context, c Config, dir string) error {
	state, err := loadState(dir)
	if err != nil {
		return err
	}
	if state.Namespace == "" {
		return errors.New("no Bridge discovery manifest exists")
	}
	mqttConfig, err := resolveMQTT(ctx, c)
	if err != nil {
		return err
	}
	if state.Broker != brokerAddress(mqttConfig) {
		return errors.New("cleanup requires the original MQTT broker configuration")
	}
	client, err := connectMQTT(mqttConfig, state, make(chan bool, 1))
	if err != nil {
		return err
	}
	defer client.Disconnect(250)
	pub := mqttPublisher{client: client}
	if err = pub.publish(state.root()+"/availability", "offline", true); err != nil {
		return err
	}
	for topic := range state.Topics {
		if err = state.removeEntity(pub, dir, topic); err != nil {
			return err
		}
	}
	for _, topic := range []string{state.root() + "/availability", state.root() + "/hub/availability"} {
		if err = pub.publish(topic, "", true); err != nil {
			return err
		}
	}
	if err = os.Remove(filepath.Join(dir, "bridge-state.json")); err != nil {
		return err
	}
	slog.Info("Removed this Bridge's discovery; next start will establish its Hub identity")
	return nil
}
