package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// MQTT's enabled_by_default only affects new registry entries. Enable existing
// integration-disabled entries through Core, leaving user/device choices intact.
func (s *service) enableEntities(ctx context.Context) {
	if !s.cfg.EnableAllEntities {
		return
	}
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
	base := os.Getenv("BESZEL_BRIDGE_SUPERVISOR_URL")
	if base == "" {
		base = "http://supervisor"
	}
	address := strings.TrimRight(base, "/") + "/core/websocket"
	address = strings.Replace(address, "http://", "ws://", 1)
	address = strings.Replace(address, "https://", "wss://", 1)
	count, err := enableRegisteredEntities(ctx, address, os.Getenv("SUPERVISOR_TOKEN"), owned)
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

func enableRegisteredEntities(ctx context.Context, address, token string, owned map[string]bool) (int, error) {
	if token == "" {
		return 0, errors.New("enable_all_entities needs the app's Home Assistant API access through Supervisor")
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
	for _, entity := range entries {
		if entity.Platform != "mqtt" || !owned[entity.UniqueID] || entity.DisabledBy != "integration" {
			continue
		}
		if err := command("config/entity_registry/update", map[string]any{"entity_id": entity.EntityID, "disabled_by": nil}, nil); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}
