package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Config struct {
	Cleanup              bool            `json:"cleanup"`
	HubURL               string          `json:"hub_url"`
	HubEmail             string          `json:"hub_email"`
	HubPassword          string          `json:"hub_password"`
	HubCAFile            string          `json:"hub_ca_file"`
	MQTTSource           string          `json:"mqtt_source"`
	MQTTHost             string          `json:"mqtt_host"`
	MQTTPort             int             `json:"mqtt_port"`
	MQTTUsername         string          `json:"mqtt_username"`
	MQTTPassword         string          `json:"mqtt_password"`
	MQTTTLS              bool            `json:"mqtt_tls"`
	MQTTCAFile           string          `json:"mqtt_ca_file"`
	IncludeSystems       []string        `json:"include_systems"`
	ExcludeSystems       []string        `json:"exclude_systems"`
	Metrics              []string        `json:"metrics"`
	EnableAllEntities    bool            `json:"enable_all_entities"`
	HistoryRetentionDays int             `json:"history_retention_days"`
	PollInterval         int             `json:"poll_interval"`
	StaleAfter           int             `json:"stale_after"`
	DetailStaleAfter     int             `json:"detail_stale_after"`
	DiskAggregateExclude []diskExclusion `json:"disk_aggregate_exclude"`
	DiscoveryPrefix      string          `json:"discovery_prefix"`
	HAStatusTopic        string          `json:"ha_status_topic"`
}

type diskExclusion struct {
	System     string `json:"system"`
	Filesystem string `json:"filesystem"`
}

var topicPart = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var recordID = regexp.MustCompile(`^[a-z0-9]{15}$`)

func loadConfig(filename string) (Config, error) {
	c := Config{MQTTSource: "supervisor", MQTTPort: 1883, PollInterval: 30,
		DiscoveryPrefix: "homeassistant", HAStatusTopic: "homeassistant/status"}
	data, err := os.ReadFile(filename)
	if err != nil {
		return c, errors.New("cannot read configuration file")
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, errors.New("configuration must be valid JSON")
	}
	return c, c.validate()
}

func (c *Config) validate() error {
	u, err := url.Parse(c.HubURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("hub_url must be an HTTP or HTTPS URL")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
		return errors.New("hub_url must contain only an address and optional plain base path")
	}
	if strings.Contains(u.Path, "..") || strings.Contains(u.Path, "/api/hassio_ingress/") {
		return errors.New("hub_url must use the direct Hub endpoint, including its configured base path")
	}
	c.HubURL = strings.TrimRight(c.HubURL, "/")
	if c.HubEmail == "" || c.HubPassword == "" {
		return errors.New("hub_email and hub_password are required")
	}
	if c.PollInterval < 15 || c.PollInterval > 300 {
		return errors.New("poll_interval must be between 15 and 300 seconds")
	}
	if c.HistoryRetentionDays < 0 || c.HistoryRetentionDays > 365 {
		return errors.New("history_retention_days must be 0 (Home Assistant default) or between 1 and 365 days")
	}
	minimumStale := max(180, 3*c.PollInterval)
	if c.StaleAfter == 0 {
		c.StaleAfter = minimumStale
	}
	if c.StaleAfter < minimumStale || c.StaleAfter > 86400 {
		return fmt.Errorf("stale_after must be at least %d seconds and at most 86400", minimumStale)
	}
	if c.DetailStaleAfter == 0 {
		c.DetailStaleAfter = 172800
	}
	if c.DetailStaleAfter < 3600 || c.DetailStaleAfter > 31536000 {
		return errors.New("detail_stale_after must be between 3600 and 31536000 seconds")
	}
	for _, exclusion := range c.DiskAggregateExclude {
		if !recordID.MatchString(exclusion.System) || exclusion.Filesystem == "" || len(exclusion.Filesystem) > 1024 {
			return errors.New("disk_aggregate_exclude requires a system ID and filesystem key")
		}
	}
	if !topicPart.MatchString(c.DiscoveryPrefix) {
		return errors.New("discovery_prefix must contain only letters, numbers, underscores or hyphens")
	}
	if c.HAStatusTopic == "" || strings.ContainsAny(c.HAStatusTopic, "+#\x00") {
		return errors.New("ha_status_topic must be a specific MQTT topic")
	}
	for _, id := range append(append([]string{}, c.IncludeSystems...), c.ExcludeSystems...) {
		if !recordID.MatchString(id) {
			return errors.New("system filters must contain 15-character Beszel record IDs")
		}
	}
	for _, key := range c.Metrics {
		if _, ok := metricDefinitions[key]; !ok && !metricFamilies[key] {
			return fmt.Errorf("unsupported metric selection: %q", key)
		}
	}
	for _, cert := range []string{c.HubCAFile, c.MQTTCAFile} {
		if cert != "" && (!filepath.IsAbs(cert) || !strings.HasPrefix(filepath.Clean(cert), "/ssl/")) {
			return errors.New("CA files must be absolute paths inside /ssl")
		}
	}
	if c.MQTTSource != "supervisor" && c.MQTTSource != "manual" {
		return errors.New("mqtt_source must be supervisor or manual")
	}
	if c.MQTTSource == "manual" {
		if c.MQTTHost == "" || strings.ContainsAny(c.MQTTHost, "/@?#\x00 ") {
			return errors.New("mqtt_host must be a hostname or IP address")
		}
		if c.MQTTPort < 1 || c.MQTTPort > 65535 {
			return errors.New("mqtt_port must be between 1 and 65535")
		}
	}
	return nil
}

func (c Config) selected(id string) bool {
	for _, excluded := range c.ExcludeSystems {
		if id == excluded {
			return false
		}
	}
	if len(c.IncludeSystems) == 0 {
		return true
	}
	for _, included := range c.IncludeSystems {
		if id == included {
			return true
		}
	}
	return false
}

func (c Config) enabled(key string) bool {
	if len(c.Metrics) == 0 {
		return true
	}
	for _, metric := range c.Metrics {
		if metric == key || metric == legacyFamilies[key] {
			return true
		}
	}
	return false
}

func (c Config) detailExpiry() int {
	if c.DetailStaleAfter == 0 {
		return 172800
	}
	return c.DetailStaleAfter
}

func (c Config) aggregateIncludes(system, filesystem string) bool {
	for _, exclusion := range c.DiskAggregateExclude {
		if exclusion.System == system && exclusion.Filesystem == filesystem {
			return false
		}
	}
	return true
}

func tlsConfig(caFile string) (*tls.Config, error) {
	c := &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, errors.New("cannot read configured CA file")
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errors.New("configured CA file contains no valid PEM certificates")
		}
		c.RootCAs = pool
	}
	return c, nil
}

func httpClient(caFile string) (*http.Client, error) {
	tlsSettings, err := tlsConfig(caFile)
	if err != nil {
		return nil, err
	}
	return &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment, DialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSClientConfig: tlsSettings, TLSHandshakeTimeout: 10 * time.Second, MaxIdleConnsPerHost: 4,
	}}, nil
}
