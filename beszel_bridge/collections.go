package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"
)

type collectionResult struct {
	Rows     []rawObject
	Complete bool
	Err      error
	Fetched  time.Time
}

type collectionJob struct {
	key      string
	interval time.Duration
	fetch    func(context.Context) collectionResult
}

// Paginated inventories are authoritative only if every page and record agrees.
// A malformed record can never authorize removal of another component.
func (h *Hub) records(ctx context.Context, collection, system string) collectionResult {
	result := collectionResult{Complete: true}
	seen := map[string]bool{}
	expected := -1
	for number := 1; number <= 10000; number++ {
		q := url.Values{"filter": {fmt.Sprintf("system='%s'", system)}, "sort": {"id"}, "page": {fmt.Sprint(number)}, "perPage": {"100"}}
		var raw json.RawMessage
		err := h.get(ctx, "/api/collections/"+collection+"/records?"+q.Encode(), &raw)
		var page recordsPage
		if err == nil {
			page, err = decodeRecordsPage(raw)
		}
		if err != nil {
			result.Complete = false
			result.Err = err
			return result
		}
		if expected < 0 {
			expected = page.TotalItems
		}
		if !page.Valid || page.Page != number || page.TotalItems != expected || page.TotalPages > 10000 ||
			(page.TotalPages == 0 && (page.TotalItems != 0 || len(page.Items) != 0)) || (len(page.Items) == 0 && number < page.TotalPages) {
			result.Complete = false
			result.Err = errors.New("Hub optional inventory has incomplete pagination")
		}
		for _, raw := range page.Items {
			row := obj(raw)
			id := row.text("id")
			if row == nil || id == "" || len(id) > 1024 || seen[id] || row.text("system") != system {
				result.Complete = false
				result.Err = errors.New("Hub optional inventory contains an invalid or duplicate record")
				continue
			}
			seen[id] = true
			result.Rows = append(result.Rows, row)
		}
		if !page.Valid || page.Page != number || page.TotalPages > 10000 || number >= page.TotalPages {
			if len(result.Rows) != expected {
				result.Complete = false
				if result.Err == nil {
					result.Err = errors.New("Hub optional inventory count does not match its records")
				}
			}
			return result
		}
	}
	result.Complete = false
	result.Err = errors.New("Hub optional inventory exceeds pagination limit")
	return result
}

func (h *Hub) recent(ctx context.Context, collection, system, extra string) collectionResult {
	filter := fmt.Sprintf("system='%s'", system)
	if extra != "" {
		filter += " && " + extra
	}
	q := url.Values{"filter": {filter}, "sort": {"-created"}, "perPage": {"1"}}
	var raw json.RawMessage
	if err := h.get(ctx, "/api/collections/"+collection+"/records?"+q.Encode(), &raw); err != nil {
		return collectionResult{Err: err}
	}
	page, err := decodeRecordsPage(raw)
	if err != nil {
		return collectionResult{Err: err}
	}
	result := collectionResult{Complete: page.Valid}
	if !page.Valid {
		result.Err = errors.New("Hub optional sample has incomplete pagination")
	}
	if len(page.Items) > 0 {
		row := obj(page.Items[0])
		if row == nil || row.text("system") != system {
			return collectionResult{Err: errors.New("Hub optional sample does not match the requested system")}
		}
		result.Rows = []rawObject{row}
	}
	return result
}

// Workers only read the Hub. Cache updates, MQTT and manifest writes all happen
// on the service goroutine after the workers have joined.
func (s *service) fetchJobs(ctx context.Context, jobs []collectionJob) map[string]collectionResult {
	if s.collections == nil {
		s.collections = map[string]collectionResult{}
	}
	results := make([]collectionResult, len(jobs))
	pending := make(chan int, len(jobs))
	for i, job := range jobs {
		cached, exists := s.collections[job.key]
		if exists && job.interval > 0 && s.now().Before(cached.Fetched.Add(job.interval)) {
			results[i] = cached
		} else {
			pending <- i
		}
	}
	close(pending)
	var workers sync.WaitGroup
	for n := 0; n < min(4, len(jobs)); n++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range pending {
				results[i] = jobs[i].fetch(ctx)
				results[i].Fetched = s.now()
			}
		}()
	}
	workers.Wait()
	byKey := map[string]collectionResult{}
	for i, job := range jobs {
		s.collections[job.key] = results[i]
		byKey[job.key] = results[i]
	}
	return byKey
}

func optionalUnsupported(err error) bool {
	var api *apiError
	return errors.As(err, &api) && (api.Status == 404 || api.Status == 403)
}

func (s *service) expandedTelemetry(ctx context.Context, system System, sample *Sample) *telemetry {
	b := newTelemetry(s.cfg, system, s.now())
	for _, entity := range s.state.Topics {
		if entity.System == system.ID {
			b.known[entity.Metric] = true
		}
	}
	b.systemMetrics(sample)
	jobs := []collectionJob{}
	slow := time.Duration(max(60, s.cfg.PollInterval)) * time.Second
	prefix := system.ID + ":"
	add := func(collection string, interval time.Duration, families ...string) {
		selected := false
		for _, family := range families {
			selected = selected || s.cfg.enabled(family)
		}
		if !selected {
			return
		}
		jobs = append(jobs, collectionJob{key: prefix + collection, interval: interval, fetch: func(ctx context.Context) collectionResult { return s.hub.records(ctx, collection, system.ID) }})
	}
	add("smart_devices", slow, "smart", "smart_attributes")
	add("containers", 0, "containers")
	add("systemd_services", 0, "services")
	add("zfs_pools", slow, "storage_pools")
	add("network_monitors", 0, "network_monitors")
	add("alerts", 0, "alerts")
	add("system_details", slow, "system_info")
	if s.cfg.enabled("containers") {
		jobs = append(jobs, collectionJob{key: prefix + "container_stats", fetch: func(ctx context.Context) collectionResult {
			return s.hub.recent(ctx, "container_stats", system.ID, "type='1m'")
		}})
	}
	if s.cfg.enabled("alerts") {
		jobs = append(jobs, collectionJob{key: prefix + "alerts_history", fetch: func(ctx context.Context) collectionResult { return s.hub.recent(ctx, "alerts_history", system.ID, "") }})
	}
	if s.cfg.enabled("cpu") || s.cfg.enabled("memory") || s.cfg.enabled("network") || s.cfg.enabled("disk_io") {
		jobs = append(jobs, collectionJob{key: prefix + "peaks", interval: slow, fetch: func(ctx context.Context) collectionResult {
			peak, err := s.hub.sample(ctx, system.ID, "10m")
			result := collectionResult{Complete: err == nil, Err: err}
			if peak != nil {
				raw, _ := json.Marshal(map[string]any{"system": peak.System, "type": peak.Type, "created": peak.Created, "stats": peak.RawStats})
				result.Rows = []rawObject{obj(raw)}
			}
			return result
		}})
	}
	if s.cfg.enabled("package_updates") && len(array(obj(system.Raw["info"])["pu"])) > 0 {
		jobs = append(jobs, collectionJob{key: prefix + "package_updates", interval: 5 * time.Minute, fetch: func(ctx context.Context) collectionResult {
			var raw json.RawMessage
			err := s.hub.get(ctx, "/api/beszel/package-updates?"+url.Values{"system": {system.ID}}.Encode(), &raw)
			if err != nil {
				return collectionResult{Err: err}
			}
			row := obj(raw)
			if row == nil {
				return collectionResult{Err: errors.New("Hub package inventory is invalid")}
			}
			return collectionResult{Complete: true, Rows: []rawObject{row}}
		}})
	}
	results := s.fetchJobs(ctx, jobs)
	get := func(key string) collectionResult { return results[prefix+key] }
	for _, job := range jobs {
		result := results[job.key]
		if result.Err != nil && !optionalUnsupported(result.Err) {
			b.issues = append(b.issues, result.Err)
		}
	}
	probeJobs := []collectionJob{}
	for _, monitor := range get("network_monitors").Rows {
		id := monitor.text("id")
		if id == "" {
			continue
		}
		// Record IDs are server supplied data. Escape before constructing a filter.
		filter := fmt.Sprintf("monitor='%s' && type='1m'", filterLiteral(id))
		probeJobs = append(probeJobs, collectionJob{key: prefix + "probe:" + id, fetch: func(ctx context.Context) collectionResult {
			return s.hub.recent(ctx, "network_monitor_stats", system.ID, filter)
		}})
	}
	probes := s.fetchJobs(ctx, probeJobs)
	b.smartMetrics(get("smart_devices"))
	b.containerMetrics(get("containers"), get("container_stats"))
	b.serviceMetrics(get("systemd_services"))
	b.poolMetrics(get("zfs_pools"), sample)
	b.monitorMetrics(get("network_monitors"), probes, prefix)
	b.packageMetrics(get("package_updates"))
	b.alertMetrics(get("alerts"), get("alerts_history"))
	b.systemInfo(get("system_details"), sample)
	if rows := get("peaks").Rows; len(rows) > 0 {
		row := rows[0]
		b.peakMetrics(&Sample{System: system.ID, Type: row.text("type"), Created: row.text("created"), RawStats: obj(row["stats"])})
	}
	return b
}

func filterLiteral(value string) string {
	result := ""
	for _, r := range value {
		if r == '\\' || r == '\'' {
			result += "\\"
		}
		result += string(r)
	}
	return result
}
