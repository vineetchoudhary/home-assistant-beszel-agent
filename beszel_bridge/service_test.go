package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type publication struct {
	topic, value string
	retain       bool
}
type recordingPublisher struct{ messages []publication }

func (p *recordingPublisher) publish(topic string, payload any, retain bool) error {
	value := fmt.Sprint(payload)
	if bytes, ok := payload.([]byte); ok {
		value = string(bytes)
	}
	p.messages = append(p.messages, publication{topic, value, retain})
	return nil
}
func (p *recordingPublisher) last(topic string) string {
	for i := len(p.messages) - 1; i >= 0; i-- {
		if p.messages[i].topic == topic {
			return p.messages[i].value
		}
	}
	return ""
}

func TestPublicationFreshnessDiscoveryAndCleanup(t *testing.T) {
	system := fixture[page[System]](t, "0.20.0", "systems.json").Items[0]
	sample := fixture[page[Sample]](t, "0.20.0", "system_stats.json").Items[0]
	created, _ := sampleTime(sample.Created)
	now := created.Add(time.Second)
	inventory := []System{system}
	fail := false
	info := HubInfo{Version: "0.20.0", Key: "ssh-ed25519 stable-fixture"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(503)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/info"):
			respond(w, info)
		case strings.HasSuffix(r.URL.Path, "/systems/records"):
			respond(w, page[System]{Page: 1, TotalPages: 1, TotalItems: len(inventory), Items: inventory})
		case strings.HasSuffix(r.URL.Path, "/system_stats/records"):
			respond(w, page[Sample]{Page: 1, TotalPages: 1, TotalItems: 1, Items: []Sample{sample}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := testConfig()
	c.HubURL = server.URL
	hub, _ := newHub(c)
	hub.token = "fixture"
	state, _ := loadState(t.TempDir())
	_ = state.initialize(info, c.DiscoveryPrefix, "tcp://broker:1883")
	p := &recordingPublisher{}
	s := &service{cfg: c, dir: t.TempDir(), hub: hub, state: state, pub: p, health: &health{}, now: func() time.Time { return now }, published: map[string]string{}}
	if err := s.sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	availability := state.hostRoot(system.ID) + "/cpu_usage/availability"
	if p.last(availability) != "online" {
		t.Fatal("fresh metric unavailable")
	}
	initialTopics := len(state.Topics)
	for _, message := range p.messages {
		if strings.HasSuffix(message.topic, "/cpu_usage") && message.retain {
			t.Error("live metric retained and can replay stale after restart")
		}
	}
	configTopic := state.discoveryTopic("sensor", system.ID, "cpu_usage")
	config := p.last(configTopic)
	if !strings.Contains(config, `"state_class":"measurement"`) || !strings.Contains(config, `"availability_mode":"all"`) {
		t.Error("discovery missing statistics or layered availability")
	}
	now = created.Add(181 * time.Second)
	inventory[0].Updated = now.Format(time.RFC3339)
	if err := s.sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if p.last(availability) != "offline" {
		t.Error("repeated old sample remained available")
	}
	if p.last(state.hostRoot(system.ID)+"/connectivity/availability") != "offline" {
		t.Error("stale system still claimed connectivity")
	}
	fail = true
	s.poll(context.Background())
	if p.last(state.root()+"/hub/availability") != "offline" || len(state.Topics) != initialTopics {
		t.Error("Hub outage failed to mark unavailable or deleted entities")
	}
	fail = false
	inventory = nil
	if err := s.sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(state.Topics) != initialTopics {
		t.Error("temporary absence immediately deleted devices")
	}
	if p.last(state.hostRoot(system.ID)+"/system_status/availability") != "offline" {
		t.Error("missing system status remained available")
	}
	now = now.Add(301 * time.Second)
	if err := s.sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if p.last(configTopic) != "" || len(state.Topics) != 4 {
		t.Error("confirmed removal did not clean only system discovery")
	}
	if err := state.removeEntity(p, s.dir, "homeassistant/sensor/unrelated/config"); err == nil {
		t.Error("cleanup accepted unrelated topics")
	}
}

func TestLivenessDoesNotDependOnHubReadiness(t *testing.T) {
	h := &health{status: healthStatus{Alive: true, Error: "Hub unavailable"}}
	response := httptest.NewRecorder()
	h.serve(response, httptest.NewRequest("GET", "/health", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"alive":true`) {
		t.Error("upstream outage would restart a healthy Bridge")
	}
}
