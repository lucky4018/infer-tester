package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Server      ServerConfig      `json:"server"`
	Model       ModelConfig       `json:"model"`
	Performance PerfConfig        `json:"performance"`
	Concurrency ConcurrencyConfig `json:"concurrency"`
	Cases       map[string]map[string]bool `json:"cases"`
	Output      OutputConfig      `json:"output"`
}

type ServerConfig struct {
	BaseURL        string `json:"base_url"`
	APIKey         string `json:"api_key"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type ModelConfig struct {
	Name string `json:"name"`
}

type PerfConfig struct {
	LatencyRequests           int `json:"latency_requests"`
	ThroughputDurationSeconds int `json:"throughput_duration_seconds"`
}

type ConcurrencyConfig struct {
	NumRequests int `json:"num_requests"`
}

type OutputConfig struct {
	ReportFile string `json:"report_file"`
	Verbose    bool   `json:"verbose"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	cfg := &Config{
		Server: ServerConfig{
			BaseURL:        "http://localhost:8000",
			TimeoutSeconds: 120,
		},
		Performance: PerfConfig{
			LatencyRequests:           10,
			ThroughputDurationSeconds: 30,
		},
		Concurrency: ConcurrencyConfig{NumRequests: 20},
		Output:      OutputConfig{ReportFile: "report.md", Verbose: true},
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Server.BaseURL == "" {
		return nil, fmt.Errorf("server.base_url is required")
	}
	return cfg, nil
}
