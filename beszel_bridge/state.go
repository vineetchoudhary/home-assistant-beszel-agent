package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type ownedEntity struct {
	System     string            `json:"system"`
	Metric     string            `json:"metric"`
	Family     string            `json:"family,omitempty"`
	Component  string            `json:"component,omitempty"`
	Definition *entityDefinition `json:"definition,omitempty"`
}

func acquireLock(dir string) (func(), error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, errors.New("cannot create Bridge data directory")
	}
	file, err := os.OpenFile(filepath.Join(dir, ".bridge.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, errors.New("cannot open Bridge state lock")
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, errors.New("another Bridge process is using this data directory; restart in cleanup mode instead")
	}
	return func() { _ = file.Close() }, nil
}

type State struct {
	Namespace        string                 `json:"namespace"`
	HubKeyHash       string                 `json:"hub_key_hash"`
	DiscoveryPrefix  string                 `json:"discovery_prefix"`
	Topics           map[string]ownedEntity `json:"topics"`
	MissingSince     map[string]time.Time   `json:"missing_since"`
	ComponentMissing map[string]time.Time   `json:"component_missing,omitempty"`
	Broker           string                 `json:"broker"`
}

func loadState(dir string) (*State, error) {
	s := &State{Topics: map[string]ownedEntity{}, MissingSince: map[string]time.Time{}, ComponentMissing: map[string]time.Time{}}
	data, err := os.ReadFile(filepath.Join(dir, "bridge-state.json"))
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, errors.New("cannot read Bridge state")
	}
	if err = json.Unmarshal(data, s); err != nil {
		return nil, errors.New("Bridge state is invalid; restore its backup before starting")
	}
	if s.Namespace != "" && !topicPart.MatchString(s.Namespace) {
		return nil, errors.New("invalid Bridge namespace in state")
	}
	if s.Topics == nil {
		s.Topics = map[string]ownedEntity{}
	}
	if s.MissingSince == nil {
		s.MissingSince = map[string]time.Time{}
	}
	if s.ComponentMissing == nil {
		s.ComponentMissing = map[string]time.Time{}
	}
	return s, nil
}
func (s *State) initialize(info HubInfo, prefix, broker string) error {
	hash := sha256.Sum256([]byte(strings.TrimSpace(info.Key)))
	key := hex.EncodeToString(hash[:])
	if s.HubKeyHash != "" && s.HubKeyHash != key {
		return errors.New("Hub identity changed; clean up the old Bridge discovery before connecting a different Hub")
	}
	if s.DiscoveryPrefix != "" && s.DiscoveryPrefix != prefix {
		return errors.New("discovery prefix changed; clean up the old Bridge discovery first")
	}
	if s.Broker != "" && s.Broker != broker {
		return errors.New("MQTT broker changed; clean up the old broker's Bridge discovery first")
	}
	if s.Namespace == "" {
		s.Namespace = "beszel_" + key[:16]
	}
	s.HubKeyHash = key
	s.DiscoveryPrefix = prefix
	s.Broker = broker
	return nil
}
func (s *State) save(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.New("cannot create Bridge data directory")
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := filepath.Join(dir, "bridge-state.json")
	// Polling normally leaves identity and discovery unchanged. Avoid writing
	// and syncing the same manifest on every poll, especially on flash storage.
	if previous, readErr := os.ReadFile(path); readErr == nil && bytes.Equal(previous, data) {
		if err = os.Chmod(path, 0600); err != nil {
			return errors.New("cannot protect Bridge state file")
		}
		return nil
	}
	tmp, err := os.CreateTemp(dir, ".bridge-state-*")
	if err != nil {
		return errors.New("cannot create Bridge state file")
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		return errors.New("cannot persist Bridge identity and discovery manifest")
	}
	return nil
}
func (s *State) root() string { return "beszel_bridge/" + s.Namespace }
func (s *State) owns(topic string) bool {
	return strings.HasPrefix(topic, s.DiscoveryPrefix+"/sensor/"+s.Namespace+"_") || strings.HasPrefix(topic, s.DiscoveryPrefix+"/binary_sensor/"+s.Namespace+"_")
}
