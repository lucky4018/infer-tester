package suite

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"infer-tester/internal/client"
	"infer-tester/internal/config"
	"infer-tester/internal/runner"
)

// benchStatLine formats mean/p50/p90/p99 from a slice of float64 (milliseconds).
func benchStatLine(data []float64) string {
	if len(data) == 0 {
		return "n/a"
	}
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)
	n := len(sorted)
	pct := func(p float64) float64 {
		idx := int(math.Ceil(p/100*float64(n))) - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= n {
			idx = n - 1
		}
		return sorted[idx]
	}
	sum := 0.0
	for _, v := range sorted {
		sum += v
	}
	return fmt.Sprintf("mean=%.1f p50=%.1f p90=%.1f p99=%.1f", sum/float64(n), pct(50), pct(90), pct(99))
}

// makeBenchPrompt generates a prompt of approximately targetTokens tokens (heuristic: 4 chars/token).
func makeBenchPrompt(targetTokens int) string {
	base := "In the field of artificial intelligence and machine learning, researchers continue to develop sophisticated models capable of understanding and generating human language with remarkable accuracy. These systems have found applications in healthcare diagnostics, educational tutoring, scientific research, and creative industries. The rapid advancement of large language models raises important questions about safety, alignment, and the future relationship between humans and intelligent machines. Engineers and scientists work to ensure these powerful tools remain beneficial and controllable. "
	target := targetTokens * 4
	for len(base) < target {
		base += base
	}
	return base[:target]
}

func Performance(c *client.Client, cfg *config.Config) runner.Suite {
	return runner.Suite{
		Name:        "performance",
		Description: "性能测试——量化延迟、吞吐、KV 缓存加速比等指标，结果仅供参考，不设通过阈值",
		Cases: []runner.TestCase{
			perfTTFT(c, cfg),
			perfTokenThroughput(c, cfg),
			perfLatency(c, cfg),
			perfPrefixCachingSpeedup(c, cfg),
			perfBenchServing(c, cfg),
		},
	}
}

func perfTTFT(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "latency_ttft", Description: "流式请求，统计首 token 到达时间（TTFT）的平均/最小/最大值", Run: func() (string, error) {
		n := cfg.Performance.LatencyRequests
		if n <= 0 {
			n = 5
		}
		req := map[string]any{
			"model": cfg.Model.Name, "prompt": "Hello", "max_tokens": 50,
			"temperature": 0, "stream": true,
		}
		var total time.Duration
		var min, max time.Duration = 1<<62, 0
		for i := 0; i < n; i++ {
			ttft, _, _, err := c.StreamTTFT(context.Background(), "/v1/completions", req)
			if err != nil {
				return "", fmt.Errorf("call %d: %v", i+1, err)
			}
			if ttft == 0 {
				return "", fmt.Errorf("call %d: no tokens received", i+1)
			}
			total += ttft
			if ttft < min {
				min = ttft
			}
			if ttft > max {
				max = ttft
			}
		}
		avg := total / time.Duration(n)
		return fmt.Sprintf("n=%d avg=%s min=%s max=%s",
			n, avg.Round(time.Millisecond), min.Round(time.Millisecond), max.Round(time.Millisecond)), nil
	}}
}

func perfTokenThroughput(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "throughput_tokens", Description: "4 并发持续 N 秒，统计输出 token 总数和平均 tokens/s 吞吐量", Run: func() (string, error) {
		dur := time.Duration(cfg.Performance.ThroughputDurationSeconds) * time.Second
		if dur <= 0 {
			dur = 10 * time.Second
		}
		const concurrency = 4
		req := map[string]any{
			"model": cfg.Model.Name, "prompt": "Tell me about AI.", "max_tokens": 128, "temperature": 0,
		}
		deadline := time.Now().Add(dur)
		var mu sync.Mutex
		var totalTokens, completed, failed int

		var wg sync.WaitGroup
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for time.Now().Before(deadline) {
					var r map[string]any
					status, err := c.JsonPost(context.Background(), "/v1/completions", req, &r)
					mu.Lock()
					if err == nil && status == 200 {
						if usage, ok := r["usage"].(map[string]any); ok {
							totalTokens += int(usage["completion_tokens"].(float64))
						}
						completed++
					} else {
						failed++
					}
					mu.Unlock()
				}
			}()
		}
		wg.Wait()

		if completed == 0 {
			return "", fmt.Errorf("all %d requests failed, server may be unavailable", failed)
		}
		tps := float64(totalTokens) / dur.Seconds()
		return fmt.Sprintf("duration=%s concurrency=%d completed=%d failed=%d total_tokens=%d throughput=%.1f tok/s",
			dur, concurrency, completed, failed, totalTokens, tps), nil
	}}
}

func perfLatency(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "latency_single_request", Description: "串行发送 N 次请求，统计端到端平均/最小/最大延迟", Run: func() (string, error) {
		n := cfg.Performance.LatencyRequests
		if n <= 0 {
			n = 5
		}
		req := map[string]any{
			"model": cfg.Model.Name, "prompt": "Hello", "max_tokens": 50, "temperature": 0,
		}
		var total time.Duration
		var min, max time.Duration = 1<<62, 0
		for i := 0; i < n; i++ {
			t0 := time.Now()
			var r map[string]any
			if status, err := c.JsonPost(context.Background(), "/v1/completions", req, &r); err != nil || status != 200 {
				return "", fmt.Errorf("call %d: status=%d err=%v", i+1, status, err)
			}
			d := time.Since(t0)
			total += d
			if d < min {
				min = d
			}
			if d > max {
				max = d
			}
		}
		avg := total / time.Duration(n)
		return fmt.Sprintf("n=%d avg=%s min=%s max=%s",
			n, avg.Round(time.Millisecond), min.Round(time.Millisecond), max.Round(time.Millisecond)), nil
	}}
}

func perfPrefixCachingSpeedup(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "prefix_caching_speedup", Description: "对比冷热前缀请求耗时，量化 KV 缓存带来的延迟加速比", Run: func() (string, error) {
		prefix := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 120)
		req := map[string]any{
			"model": cfg.Model.Name, "max_tokens": 5, "temperature": 0,
		}

		req["prompt"] = prefix + " Repeat the first word."
		var warm map[string]any
		if status, err := c.JsonPost(context.Background(), "/v1/completions", req, &warm); err != nil || status != 200 {
			return "", fmt.Errorf("warm-up: status=%d err=%v", status, err)
		}

		req["prompt"] = prefix + " What comes after the lazy dog?"
		t0 := time.Now()
		var coldR map[string]any
		if status, err := c.JsonPost(context.Background(), "/v1/completions", req, &coldR); err != nil || status != 200 {
			return "", fmt.Errorf("cold call: status=%d err=%v", status, err)
		}
		coldDur := time.Since(t0)

		t1 := time.Now()
		var warmR map[string]any
		if status, err := c.JsonPost(context.Background(), "/v1/completions", req, &warmR); err != nil || status != 200 {
			return "", fmt.Errorf("warm call: status=%d err=%v", status, err)
		}
		warmDur := time.Since(t1)

		speedup := float64(coldDur-warmDur) / float64(coldDur) * 100
		return fmt.Sprintf("cold=%s warm=%s speedup=%.1f%%",
			coldDur.Round(time.Millisecond), warmDur.Round(time.Millisecond), speedup), nil
	}}
}


func perfBenchServing(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{
		Name:        "bench_serving",
		Description: "自适应并发压测：检测 vLLM 配置 → 阶梯式探测最优并发 → 正式测试",
		Run: func() (string, error) {
			// 1. 检测 vLLM 运行时配置
			vllmCfg := detectVllmConfig(c)
			cfgStr := ""
			if vllmCfg != nil {
				cfgStr = fmt.Sprintf("vllm{gpu_mem=%s kv_blocks=%s prefix_cache=%s} | ",
					vllmCfg.GpuMemoryUtilization, vllmCfg.NumGpuBlocks, vllmCfg.EnablePrefixCaching)
				fmt.Printf("  [bench_serving] vLLM config: gpu_mem=%s kv_blocks=%s prefix_cache=%s\n",
					vllmCfg.GpuMemoryUtilization, vllmCfg.NumGpuBlocks, vllmCfg.EnablePrefixCaching)
			}

			n := cfg.Performance.BenchNumPrompts
			if n <= 0 {
				n = 100
			}
			inputLen := cfg.Performance.BenchInputLen
			if inputLen <= 0 {
				inputLen = 512
			}
			outputLen := cfg.Performance.BenchOutputLen
			if outputLen <= 0 {
				outputLen = 128
			}
			prompt := makeBenchPrompt(inputLen)

			// 2. 阶梯式探测最优并发度
			probeLevels := []int{8, 16, 32, 64, 128, 256}
			type probeResult struct {
				concurrency int
				tokPS       float64
			}
			var probeResults []probeResult
			bestConcurrency := 32 // 默认值
			bestTokPS := 0.0

			for _, concurrency := range probeLevels {
				if concurrency > n*2 {
					break
				}
				// 每轮发 min(concurrency, 16) 个请求做快速探测
				probeN := concurrency
				if probeN > 16 {
					probeN = 16
				}
				tokPS := probeThroughput(c, cfg, prompt, probeN, concurrency, outputLen)
				fmt.Printf("  [bench_serving] probe concurrency=%d tok/s=%.1f\n", concurrency, tokPS)
				probeResults = append(probeResults, probeResult{concurrency, tokPS})

				if tokPS > bestTokPS*1.1 {
					// 提升 >10%，继续探测
					bestTokPS = tokPS
					bestConcurrency = concurrency
				} else if tokPS > bestTokPS {
					// 有提升但不显著，记下并停止
					bestTokPS = tokPS
					bestConcurrency = concurrency
					break
				} else {
					// 吞吐量下降，停止
					break
				}
			}

			fmt.Printf("  [bench_serving] best concurrency=%d, starting formal test with n=%d\n", bestConcurrency, n)

			// 3. 用最优并发度做正式测试（semaphore 限流）
			type reqResult struct {
				rec client.BenchRecord
				err error
			}
			results := make([]reqResult, n)
			var wg sync.WaitGroup
			sem := make(chan struct{}, bestConcurrency)

			launch := func(idx int) {
				sem <- struct{}{}
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
					defer cancel()
					body := map[string]any{
						"model":          cfg.Model.Name,
						"messages":       []map[string]string{{"role": "user", "content": prompt}},
						"max_tokens":     outputLen,
						"temperature":    0,
						"stream":         true,
						"stream_options": map[string]bool{"include_usage": true},
					}
					rec, err := c.StreamBench(ctx, "/v1/chat/completions", body)
					results[idx] = reqResult{rec, err}
				}()
			}

			t0 := time.Now()
			for i := 0; i < n; i++ {
				launch(i)
			}
			wg.Wait()
			duration := time.Since(t0)

			var (
				ttftsMs, tpotsMs, itlsMs []float64
				totalToks, completed, failed int
			)
			for _, r := range results {
				if r.err != nil {
					failed++
					continue
				}
				completed++
				rec := r.rec
				totalToks += rec.NumTokens
				ttftsMs = append(ttftsMs, float64(rec.TTFT.Milliseconds()))
				if rec.NumTokens > 1 {
					decodeMs := float64((rec.Total - rec.TTFT).Milliseconds())
					tpotsMs = append(tpotsMs, decodeMs/float64(rec.NumTokens-1))
				}
				for i := 1; i < len(rec.ChunkTimes); i++ {
					itlsMs = append(itlsMs, float64((rec.ChunkTimes[i]-rec.ChunkTimes[i-1]).Milliseconds()))
				}
			}

			if completed == 0 {
				return "", fmt.Errorf("all %d requests failed", n)
			}

			reqPerSec := float64(completed) / duration.Seconds()
			tokPerSec := float64(totalToks) / duration.Seconds()

			// 构建探测结果字符串
			probeStr := "probes{"
			for i, pr := range probeResults {
				if i > 0 {
					probeStr += " "
				}
				probeStr += fmt.Sprintf("c=%d:%.0ftok/s", pr.concurrency, pr.tokPS)
			}
			probeStr += fmt.Sprintf("} best_c=%d | ", bestConcurrency)

			return cfgStr + probeStr + fmt.Sprintf(
				"n=%d ok=%d fail=%d dur=%.1fs req/s=%.2f tok/s=%.1f | TTFT(ms) %s | TPOT(ms) %s | ITL(ms) %s",
				n, completed, failed, duration.Seconds(), reqPerSec, tokPerSec,
				benchStatLine(ttftsMs), benchStatLine(tpotsMs), benchStatLine(itlsMs),
			), nil
		},
	}
}

// vllmRuntimeConfig 从 /metrics 提取的 vLLM 运行时配置
type vllmRuntimeConfig struct {
	GpuMemoryUtilization string
	NumGpuBlocks         string
	EnablePrefixCaching  string
}

// detectVllmConfig 从 /metrics 端点获取 vLLM 的 cache_config_info
func detectVllmConfig(c *client.Client) *vllmRuntimeConfig {
	resp, err := c.Get(context.Background(), "/metrics")
	if err != nil || resp.Status != 200 {
		return nil
	}
	for _, line := range strings.Split(string(resp.Body), "\n") {
		if strings.HasPrefix(line, "vllm:cache_config_info{") {
			return &vllmRuntimeConfig{
				GpuMemoryUtilization: extractMetricLabel(line, "gpu_memory_utilization"),
				NumGpuBlocks:         extractMetricLabel(line, "num_gpu_blocks"),
				EnablePrefixCaching:  extractMetricLabel(line, "enable_prefix_caching"),
			}
		}
	}
	return nil
}

// extractMetricLabel 从 Prometheus 指标行中提取标签值
func extractMetricLabel(line, key string) string {
	idx := strings.Index(line, key+"=\"")
	if idx < 0 {
		return "n/a"
	}
	start := idx + len(key) + 2
	end := strings.Index(line[start:], "\"")
	if end < 0 {
		return "n/a"
	}
	return line[start : start+end]
}

// probeThroughput 发送 probeN 个请求（限制并发度为 concurrency），返回 tok/s
func probeThroughput(c *client.Client, cfg *config.Config, prompt string, probeN, concurrency, outputLen int) float64 {
	type probeResult struct {
		rec client.BenchRecord
		err error
	}
	results := make([]probeResult, probeN)
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	for i := 0; i < probeN; i++ {
		sem <- struct{}{}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
			defer cancel()
			body := map[string]any{
				"model":          cfg.Model.Name,
				"messages":       []map[string]string{{"role": "user", "content": prompt}},
				"max_tokens":     outputLen,
				"temperature":    0,
				"stream":         true,
				"stream_options": map[string]bool{"include_usage": true},
			}
			rec, err := c.StreamBench(ctx, "/v1/chat/completions", body)
			results[idx] = probeResult{rec, err}
		}(i)
	}
	wg.Wait()

	totalToks := 0
	completed := 0
	var maxTotal time.Duration
	for _, r := range results {
		if r.err != nil {
			continue
		}
		totalToks += r.rec.NumTokens
		completed++
		if r.rec.Total > maxTotal {
			maxTotal = r.rec.Total
		}
	}
	if maxTotal <= 0 {
		return 0
	}
	return float64(totalToks) / maxTotal.Seconds()
}
