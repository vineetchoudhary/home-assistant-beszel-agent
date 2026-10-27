package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var version = "dev"

func main() {
	if err := execute(); err != nil {
		slog.Error("Beszel Bridge stopped", "reason", err)
		os.Exit(1)
	}
}
func execute() error {
	command := "run"
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command = args[0]
		args = args[1:]
	}
	flags := flag.NewFlagSet("bridge", flag.ContinueOnError)
	filename := flags.String("config", "/data/options.json", "Configuration file")
	dir := flags.String("data", "/data", "Private persistent data directory")
	address := flags.String("listen", ":8099", "Internal liveness/status listener")
	healthURL := flags.String("url", "http://127.0.0.1:8099/health", "Health URL")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if command == "version" {
		fmt.Println(version)
		return nil
	}
	if command == "health" || command == "status" {
		if command == "status" {
			*healthURL = strings.TrimSuffix(*healthURL, "/health") + "/status"
		}
		client := http.Client{Timeout: 2 * time.Second}
		response, err := client.Get(*healthURL)
		if err != nil {
			return fmt.Errorf("Bridge liveness endpoint unavailable")
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("Bridge liveness endpoint returned HTTP %d", response.StatusCode)
		}
		if command == "status" {
			_, err = io.Copy(os.Stdout, io.LimitReader(response.Body, 16384))
			return err
		}
		return nil
	}
	c, err := loadConfig(*filename)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if command == "run" || command == "cleanup" {
		unlock, err := acquireLock(*dir)
		if err != nil {
			return err
		}
		defer unlock()
	}
	switch command {
	case "run":
		if c.Cleanup {
			return cleanup(ctx, c, *dir)
		}
		return run(ctx, c, *dir, *address)
	case "cleanup":
		return cleanup(ctx, c, *dir)
	case "check":
		hub, err := newHub(c)
		if err != nil {
			return err
		}
		info, err := hub.info(ctx)
		if err != nil {
			return err
		}
		systems, err := hub.systems(ctx)
		if err != nil {
			return err
		}
		report := map[string]any{"hub_version": info.Version, "visible_systems": len(systems), "systems": []any{}}
		rows := []map[string]any{}
		for _, system := range systems {
			if !c.selected(system.ID) {
				continue
			}
			sample, err := hub.latestSample(ctx, system.ID)
			if err != nil {
				return err
			}
			values, created, err := metricValues(system, sample)
			if err != nil {
				return err
			}
			rows = append(rows, map[string]any{"id": system.ID, "name": system.Name, "status": system.Status, "sample_created": created, "fresh": fresh(created, time.Now(), c.StaleAfter), "available_metrics": len(values)})
		}
		report["systems"] = rows
		return json.NewEncoder(os.Stdout).Encode(report)
	default:
		return fmt.Errorf("command must be run, check, health, status, cleanup, or version")
	}
}
