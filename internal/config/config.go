package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Server      ServerConfig               `json:"server"`
	Model       ModelConfig                `json:"model"`
	Performance PerfConfig                 `json:"performance"`
	Concurrency ConcurrencyConfig          `json:"concurrency"`
	VllmBench   VllmBenchConfig            `json:"vllm_bench"`
	Cases       map[string]map[string]bool `json:"cases"`
	Output      OutputConfig               `json:"output"`
}

type VllmBenchConfig struct {
	Image     string `json:"image"`      // Docker 镜像名，留空则自动检测宿主机上第一个含 vllm 的镜像
	Args      string `json:"args"`       // 原样透传给 vllm bench serve 的参数，--host/--port/--model 自动注入
	Tokenizer string `json:"tokenizer"`  // 可选：指定 tokenizer 路径或 HF repo id，解决非标准模型名无法加载 tokenizer 的问题
	ModelDir  string `json:"model_dir"` // 可选：宿主机模型目录根路径，挂载到容器内供 tokenizer 加载
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
	LatencyRequests           int     `json:"latency_requests"`
	ThroughputDurationSeconds int     `json:"throughput_duration_seconds"`
	BenchNumPrompts           int     `json:"bench_num_prompts"`  // Go bench_serving 用
	BenchRequestRate          float64 `json:"bench_request_rate"` // Go bench_serving 用
	BenchInputLen             int     `json:"bench_input_len"`    // Go bench_serving 用
	BenchOutputLen            int     `json:"bench_output_len"`   // Go bench_serving 用
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
			BenchNumPrompts:           100,
			BenchRequestRate:          0,
			BenchInputLen:             512,
			BenchOutputLen:            128,
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
