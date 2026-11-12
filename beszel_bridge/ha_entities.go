package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var recordedEntityID = regexp.MustCompile(`^[a-z0-9_]+\.[a-z0-9_]+$`)

// MQTT's enabled_by_default only affects new registry entries. Enable existing
// integration-disabled entries through Core, leaving user/device choices intact.
func (s *service) enableEntities(ctx context.Context) {
	if !s.cfg.EnableAllEntities {
		return
	}
	address, token := supervisorCoreAPI()
	count, err := enableRegisteredEntities(ctx, address, token, s.ownedEntityUniqueIDs())
	if err != nil {
		if s.entityEnableWarning != err.Error() {
			slog.Warn("Cannot enable existing Home Assistant entities; retrying on the next poll", "reason", err)
			s.entityEnableWarning = err.Error()
		}
		return
	}
	if count > 0 || s.entityEnableWarning != "" {
		slog.Info("Home Assistant entity enabling completed", "enabled", count)
	}
	s.entityEnableWarning = ""
}

func (s *service) ownedEntityUniqueIDs() map[string]bool {
	owned := map[string]bool{}
	for topic, entity := range s.state.Topics {
		if !s.state.owns(topic) {
			continue
		}
		system := entity.System
		if system == "" {
			system = "bridge"
		} else if !s.cfg.selected(system) {
			continue
		} else if entity.Definition != nil {
			if entity.Family != "diagnostics" && !s.cfg.enabled(entity.Family) {
				continue
			}
		} else if _, metric := metricDefinitions[entity.Metric]; metric && !s.cfg.enabled(entity.Metric) {
			continue
		}
		owned[s.state.Namespace+"_"+system+"_"+entity.Metric] = true
	}
	return owned
}

func supervisorCoreAPI() (string, string) {
	base := os.Getenv("BESZEL_BRIDGE_SUPERVISOR_URL")
	if base == "" {
		base = "http://supervisor"
	}
	address := strings.TrimRight(base, "/") + "/core/websocket"
	address = strings.Replace(address, "http://", "ws://", 1)
	address = strings.Replace(address, "https://", "wss://", 1)
	return address, os.Getenv("SUPERVISOR_TOKEN")
}

func enableRegisteredEntities(ctx context.Context, address, token string, owned map[string]bool) (int, error) {
	return manageRegisteredEntities(ctx, address, token, owned, true, 0)
}

func purgeRegisteredEntityHistory(ctx context.Context, address, token string, owned map[string]bool, keepDays int) (int, error) {
	if keepDays < 1 || keepDays > 365 {
		return 0, errors.New("history retention cleanup requires between 1 and 365 days")
	}
	return manageRegisteredEntities(ctx, address, token, owned, false, keepDays)
}

func manageRegisteredEntities(ctx context.Context, address, token string, owned map[string]bool, enable bool, keepDays int) (int, error) {
	if token == "" {
		return 0, errors.New("the app's Home Assistant API access through Supervisor is required")
	}
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return 0, errors.New("invalid Home Assistant API endpoint")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	connection, response, err := dialer.DialContext(ctx, address, nil)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if err != nil {
		return 0, errors.New("cannot connect to Home Assistant's API through Supervisor")
	}
	defer connection.Close()
	stop := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer stop()
	connection.SetReadLimit(32 << 20)
	read := func(value any) error {
		_ = connection.SetReadDeadline(time.Now().Add(10 * time.Second))
		if err := connection.ReadJSON(value); err != nil {
			return errors.New("cannot read a valid Home Assistant API response")
		}
		return nil
	}
	write := func(value any) error {
		_ = connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := connection.WriteJSON(value); err != nil {
			return errors.New("cannot send a Home Assistant API request")
		}
		return nil
	}
	var auth struct {
		Type string `json:"type"`
	}
	if err := read(&auth); err != nil {
		return 0, err
	}
	if auth.Type != "auth_required" {
		return 0, errors.New("unexpected Home Assistant API authentication response")
	}
	if err := write(map[string]any{"type": "auth", "access_token": token}); err != nil {
		return 0, err
	}
	if err := read(&auth); err != nil {
		return 0, err
	}
	if auth.Type != "auth_ok" {
		return 0, errors.New("Home Assistant API authentication failed; check the app's API access")
	}
	sequence := 0
	command := func(kind string, fields map[string]any, result any) error {
		sequence++
		if fields == nil {
			fields = map[string]any{}
		}
		fields["id"], fields["type"] = sequence, kind
		if err := write(fields); err != nil {
			return err
		}
		var reply struct {
			ID      int             `json:"id"`
			Type    string          `json:"type"`
			Success bool            `json:"success"`
			Result  json.RawMessage `json:"result"`
		}
		if err := read(&reply); err != nil {
			return err
		}
		if reply.Type != "result" || reply.ID != sequence || !reply.Success {
			return fmt.Errorf("Home Assistant API command %s failed", kind)
		}
		if result != nil && json.Unmarshal(reply.Result, result) != nil {
			return errors.New("Home Assistant returned an invalid entity registry")
		}
		return nil
	}
	var entries []struct {
		EntityID   string `json:"entity_id"`
		UniqueID   string `json:"unique_id"`
		Platform   string `json:"platform"`
		DisabledBy string `json:"disabled_by"`
	}
	if err := command("config/entity_registry/list", nil, &entries); err != nil {
		return 0, err
	}
	count := 0
	targets := map[string]bool{}
	for _, entity := range entries {
		if entity.Platform != "mqtt" || !owned[entity.UniqueID] {
			continue
		}
		if keepDays > 0 && recordedEntityID.MatchString(entity.EntityID) {
			targets[entity.EntityID] = true
		}
		if !enable || entity.DisabledBy != "integration" {
			continue
		}
		if err := command("config/entity_registry/update", map[string]any{"entity_id": entity.EntityID, "disabled_by": nil}, nil); err != nil {
			return count, err
		}
		count++
	}
	if keepDays > 0 && len(targets) > 0 {
		ids := make([]string, 0, len(targets))
		for id := range targets {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		// Never pass a domain, glob or empty entity list to Recorder cleanup.
		if err := command("call_service", map[string]any{
			"domain": "recorder", "service": "purge_entities",
			"service_data": map[string]any{"entity_id": ids, "keep_days": keepDays},
		}, nil); err != nil {
			return 0, err
		}
		return len(ids), nil
	}
	return count, nil
}
