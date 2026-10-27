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
	"strings"
)

//go:embed compatibility.json
var compatibilityJSON []byte

type Compatibility struct {
	HubVersions []string `json:"hub_versions"`
}

func supportedVersion(version string) bool {
	var policy Compatibility
	if json.Unmarshal(compatibilityJSON, &policy) != nil {
		return false
	}
	for _, candidate := range policy.HubVersions {
		if version == candidate {
			return true
		}
	}
	return false
}

type HubInfo struct {
	Key     string `json:"key"`
	Version string `json:"v"`
}
type System struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Updated string `json:"updated"`
	Info    struct {
		Uptime  *uint64 `json:"u"`
		Version string  `json:"v"`
	} `json:"info"`
}
type Sample struct {
	ID      string `json:"id"`
	System  string `json:"system"`
	Type    string `json:"type"`
	Created string `json:"created"`
	Stats   struct {
		CPU          *float64             `json:"cpu"`
		Memory       *float64             `json:"mp"`
		Disk         *float64             `json:"dp"`
		Bandwidth    []float64            `json:"b"`
		Interfaces   map[string][]float64 `json:"ni"`
		Temperatures map[string]float64   `json:"t"`
	} `json:"stats"`
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
	cfg    Config
	client *http.Client
	token  string
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
	return &Hub{cfg: c, client: client}, nil
}

func (h *Hub) raw(ctx context.Context, method, endpoint string, payload any, output any) error {
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
	if h.token != "" {
		req.Header.Set("Authorization", h.token)
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
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(output); err != nil {
		return errors.New("Hub returned invalid API JSON; check its base path, authentication gateway, and supported version")
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
		MFAID string `json:"mfaId"`
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
	h.token = result.Token
	return nil
}

func (h *Hub) get(ctx context.Context, endpoint string, output any) error {
	if h.token == "" {
		if err := h.authenticate(ctx, false); err != nil {
			return err
		}
	}
	err := h.raw(ctx, "GET", endpoint, nil, output)
	var api *apiError
	if errors.As(err, &api) && api.Status == http.StatusUnauthorized {
		if refreshErr := h.authenticate(ctx, true); refreshErr != nil {
			h.token = ""
			if loginErr := h.authenticate(ctx, false); loginErr != nil {
				return loginErr
			}
		}
		return h.raw(ctx, "GET", endpoint, nil, output)
	}
	return err
}

func (h *Hub) info(ctx context.Context) (HubInfo, error) {
	var info HubInfo
	if err := h.get(ctx, "/api/beszel/info", &info); err != nil {
		return info, err
	}
	if !supportedVersion(info.Version) {
		return info, fmt.Errorf("unsupported Hub version %q; check the Bridge compatibility table", info.Version)
	}
	if !strings.HasPrefix(info.Key, "ssh-") {
		return info, errors.New("Hub did not return its stable public-key identity")
	}
	return info, nil
}

func (h *Hub) systems(ctx context.Context) ([]System, error) {
	var systems []System
	seen := map[string]bool{}
	expected := -1
	for number := 1; number <= 10000; number++ {
		var result page[System]
		endpoint := fmt.Sprintf("/api/collections/systems/records?page=%d&perPage=100&sort=id", number)
		if err := h.get(ctx, endpoint, &result); err != nil {
			return nil, err
		}
		if expected == -1 {
			expected = result.TotalItems
		}
		if result.TotalItems != expected {
			return nil, errors.New("Hub inventory changed during pagination; retrying complete inventory")
		}
		if result.Page != number || result.TotalPages < 0 || result.TotalPages > 10000 || (len(result.Items) == 0 && number < result.TotalPages) {
			return nil, errors.New("Hub returned an incomplete system inventory")
		}
		for _, system := range result.Items {
			if !recordID.MatchString(system.ID) || seen[system.ID] {
				return nil, errors.New("Hub returned invalid or duplicate system identifiers")
			}
			seen[system.ID] = true
			systems = append(systems, system)
		}
		if number >= result.TotalPages {
			if len(systems) != expected {
				return nil, errors.New("Hub returned an incomplete system inventory")
			}
			return systems, nil
		}
	}
	return nil, errors.New("Hub inventory exceeds the supported pagination limit")
}

func (h *Hub) latestSample(ctx context.Context, id string) (*Sample, error) {
	q := url.Values{"filter": {fmt.Sprintf("system='%s' && type='1m'", id)}, "sort": {"-created"}, "perPage": {"1"}}
	var result page[Sample]
	if err := h.get(ctx, "/api/collections/system_stats/records?"+q.Encode(), &result); err != nil {
		return nil, err
	}
	if len(result.Items) == 0 {
		return nil, nil
	}
	sample := result.Items[0]
	if sample.System != id || sample.Type != "1m" {
		return nil, errors.New("Hub sample does not match the requested system and interval")
	}
	return &sample, nil
}
