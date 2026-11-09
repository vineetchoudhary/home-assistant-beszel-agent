package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

//go:embed compatibility.json
var compatibilityJSON []byte

type Compatibility struct {
	TestedHubVersions []string `json:"tested_hub_versions"`
}

var releaseVersion = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+(?:[-+][A-Za-z0-9.-]+)?$`)

type HubInfo struct {
	Key     string `json:"key"`
	Version string `json:"v"`
}
type System struct {
	Raw     rawObject `json:"-"`
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Status  string    `json:"status"`
	Updated string    `json:"updated"`
	Info    struct {
		Uptime  *uint64 `json:"u"`
		Version string  `json:"v"`
	} `json:"info"`
	Issues []string `json:"-"`
}
type Sample struct {
	RawStats rawObject `json:"-"`
	ID       string    `json:"id"`
	System   string    `json:"system"`
	Type     string    `json:"type"`
	Created  string    `json:"created"`
	Stats    struct {
		CPU          *float64             `json:"cpu"`
		Memory       *float64             `json:"mp"`
		Disk         *float64             `json:"dp"`
		Bandwidth    []float64            `json:"b"`
		Interfaces   map[string][]float64 `json:"ni"`
		Temperatures map[string]float64   `json:"t"`
	} `json:"stats"`
	Issues []string `json:"-"`
}
type page[T any] struct {
	Page       int `json:"page"`
	TotalPages int `json:"totalPages"`
	TotalItems int `json:"totalItems"`
	Items      []T `json:"items"`
}
type apiError struct {
	Status    int
	Operation string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("Hub %s failed (HTTP %d)", e.Operation, e.Status)
}

type Hub struct {
	cfg     Config
	client  *http.Client
	token   string
	tokenMu sync.RWMutex
	authMu  sync.Mutex
	slots   chan struct{}
}

func newHub(c Config) (*Hub, error) {
	client, err := httpClient(c.HubCAFile)
	if err != nil {
		return nil, err
	}
	origin, _ := url.Parse(c.HubURL)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || req.URL.Scheme != origin.Scheme || req.URL.Host != origin.Host || !strings.HasPrefix(req.URL.Path, origin.Path+"/api/") {
			return errors.New("Hub redirected outside its API; configure its direct API address")
		}
		return nil
	}
	return &Hub{cfg: c, client: client, slots: make(chan struct{}, 4)}, nil
}

func (h *Hub) raw(ctx context.Context, method, endpoint string, payload any, output any) error {
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	case <-ctx.Done():
		return ctx.Err()
	}
	var reader io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return errors.New("cannot encode Hub request")
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, h.cfg.HubURL+endpoint, reader)
	if err != nil {
		return errors.New("cannot construct Hub API request")
	}
	req.Header.Set("Content-Type", "application/json")
	if token := h.sessionToken(); token != "" {
		req.Header.Set("Authorization", token)
	}
	response, err := h.client.Do(req)
	if err != nil {
		// Do not include URLs, response bodies, or credentials in diagnostic errors.
		return fmt.Errorf("Hub network/TLS request failed: %w", safeNetworkError(err))
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return &apiError{Status: response.StatusCode, Operation: strings.Split(endpoint, "?")[0]}
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 8<<20))
	if err := decoder.Decode(output); err != nil {
		return errors.New("Hub returned invalid API JSON; check its base path and authentication gateway")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("Hub returned invalid API JSON after its response")
	}
	return nil
}

func safeNetworkError(err error) error {
	var u *url.Error
	if errors.As(err, &u) {
		return u.Err
	}
	return err
}

func (h *Hub) authenticate(ctx context.Context, refresh bool) error {
	var result struct {
		Token string `json:"token"`
	}
	endpoint := "/api/collections/users/auth-with-password"
	var payload any = map[string]string{"identity": h.cfg.HubEmail, "password": h.cfg.HubPassword}
	if refresh {
		endpoint = "/api/collections/users/auth-refresh"
		payload = map[string]string{}
	}
	if err := h.raw(ctx, "POST", endpoint, payload, &result); err != nil {
		return err
	}
	if result.Token == "" {
		return errors.New("Hub password login did not return a session; OAuth-only and interactive MFA logins are unsupported")
	}
	h.tokenMu.Lock()
	h.token = result.Token
	h.tokenMu.Unlock()
	return nil
}

func (h *Hub) get(ctx context.Context, endpoint string, output any) error {
	h.authMu.Lock()
	if h.sessionToken() == "" {
		if err := h.authenticate(ctx, false); err != nil {
			h.authMu.Unlock()
			return err
		}
	}
	token := h.sessionToken()
	h.authMu.Unlock()
	err := h.raw(ctx, "GET", endpoint, nil, output)
	var api *apiError
	if errors.As(err, &api) && api.Status == http.StatusUnauthorized {
		h.authMu.Lock()
		defer h.authMu.Unlock()
		if h.sessionToken() != token {
			return h.raw(ctx, "GET", endpoint, nil, output)
		}
		if refreshErr := h.authenticate(ctx, true); refreshErr != nil {
			h.tokenMu.Lock()
			h.token = ""
			h.tokenMu.Unlock()
			if loginErr := h.authenticate(ctx, false); loginErr != nil {
				return loginErr
			}
		}
		return h.raw(ctx, "GET", endpoint, nil, output)
	}
	return err
}

func (h *Hub) sessionToken() string {
	h.tokenMu.RLock()
	defer h.tokenMu.RUnlock()
	return h.token
}

func (h *Hub) info(ctx context.Context) (HubInfo, error) {
	var info HubInfo
	if err := h.get(ctx, "/api/beszel/info", &info); err != nil {
		return info, err
	}
	if !strings.HasPrefix(info.Key, "ssh-") {
		return info, errors.New("Hub did not return its stable public-key identity")
	}
	return info, nil
}

type Inventory struct {
	Systems  []System
	Complete bool
	Issues   []string
}

func (h *Hub) systems(ctx context.Context) (Inventory, error) {
	inventory := Inventory{Complete: true}
	seen := map[string]bool{}
	expected := -1
	for number := 1; number <= 10000; number++ {
		var raw json.RawMessage
		endpoint := fmt.Sprintf("/api/collections/systems/records?page=%d&perPage=100&sort=id", number)
		err := h.get(ctx, endpoint, &raw)
		var result recordsPage
		if err == nil {
			result, err = decodeRecordsPage(raw)
		}
		if err != nil {
			if number == 1 || ctx.Err() != nil {
				return inventory, err
			}
			inventory.Complete = false
			inventory.Issues = append(inventory.Issues, "Hub inventory pagination failed")
			return inventory, nil
		}
		if expected == -1 {
			expected = result.TotalItems
		}
		if result.TotalItems != expected {
			inventory.Complete = false
			inventory.Issues = append(inventory.Issues, "Hub inventory changed during pagination")
		}
		for _, record := range result.Items {
			var system System
			if json.Unmarshal(record, &system) != nil || !recordID.MatchString(system.ID) || seen[system.ID] {
				inventory.Complete = false
				inventory.Issues = append(inventory.Issues, "Hub inventory contains an invalid or duplicate system identifier")
				continue
			}
			seen[system.ID] = true
			inventory.Systems = append(inventory.Systems, system)
		}
		if !result.Valid || result.Page != number || result.TotalPages > 10000 ||
			(result.TotalPages == 0 && (len(result.Items) > 0 || result.TotalItems != 0)) ||
			(len(result.Items) == 0 && number < result.TotalPages) {
			inventory.Complete = false
			inventory.Issues = append(inventory.Issues, "Hub returned incomplete inventory pagination metadata")
			return inventory, nil
		}
		if number >= result.TotalPages {
			if len(inventory.Systems) != expected {
				inventory.Complete = false
				inventory.Issues = append(inventory.Issues, "Hub inventory count does not match its records")
			}
			return inventory, nil
		}
	}
	inventory.Complete = false
	inventory.Issues = append(inventory.Issues, "Hub inventory exceeds the pagination limit")
	return inventory, nil
}

func (h *Hub) latestSample(ctx context.Context, id string) (*Sample, error) {
	return h.sample(ctx, id, "1m")
}

func (h *Hub) sample(ctx context.Context, id, kind string) (*Sample, error) {
	q := url.Values{"filter": {fmt.Sprintf("system='%s' && type='%s'", id, kind)}, "sort": {"-created"}, "perPage": {"1"}}
	var raw json.RawMessage
	if err := h.get(ctx, "/api/collections/system_stats/records?"+q.Encode(), &raw); err != nil {
		return nil, err
	}
	result, err := decodeRecordsPage(raw)
	if err != nil {
		return nil, err
	}
	if len(result.Items) == 0 {
		return nil, nil
	}
	var sample Sample
	if json.Unmarshal(result.Items[0], &sample) != nil {
		return nil, errors.New("Hub returned an invalid sample record")
	}
	if sample.System != id || sample.Type != kind {
		return nil, errors.New("Hub sample does not match the requested system and interval")
	}
	return &sample, nil
}
