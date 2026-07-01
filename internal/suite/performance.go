package suite

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"infer-tester/internal/client"
	"infer-tester/internal/config"
	"infer-tester/internal/runner"
)

func Performance(c *client.Client, cfg *config.Config) runner.Suite {
	return runner.Suite{
		Name:        "performance",
		Description: "性能测试——量化延迟、吞吐、KV 缓存加速比等指标，结果仅供参考，不设通过阈值",
		Cases: []runner.TestCase{
			perfTTFT(c, cfg),
			perfTokenThroughput(c, cfg),
			perfLatency(c, cfg),
			perfPrefixCachingSpeedup(c, cfg),
			perfBatchScaling(c, cfg),
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

func perfBatchScaling(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "batch_scaling", Description: "batch=1/2/4 并发，观察 tokens/s 随批大小的变化趋势", Run: func() (string, error) {
		type result struct {
			batch    int
			dur      time.Duration
			tokensPS float64
		}
		var results []result

		for _, batchSize := range []int{1, 2, 4} {
			t0 := time.Now()
			var wg sync.WaitGroup
			var mu sync.Mutex
			var totalTokens int
			for i := 0; i < batchSize; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					var r map[string]any
					if status, err := c.JsonPost(context.Background(), "/v1/completions", map[string]any{
						"model": cfg.Model.Name, "prompt": "Tell me a fact.", "max_tokens": 32, "temperature": 0,
					}, &r); err == nil && status == 200 {
						usage := r["usage"].(map[string]any)
						mu.Lock()
						totalTokens += int(usage["completion_tokens"].(float64))
						mu.Unlock()
					}
				}()
			}
			wg.Wait()
			dur := time.Since(t0)
			tps := float64(totalTokens) / dur.Seconds()
			results = append(results, result{batchSize, dur, tps})
		}

		allZero := true
		for _, r := range results {
			if r.tokensPS > 0 {
				allZero = false
				break
			}
		}
		if allZero {
			return "", fmt.Errorf("all batch requests failed, server may be unavailable")
		}
		var sb strings.Builder
		for _, r := range results {
			fmt.Fprintf(&sb, "batch=%d dur=%s tok/s=%.1f  ", r.batch, r.dur.Round(time.Millisecond), r.tokensPS)
		}
		return strings.TrimSpace(sb.String()), nil
	}}
}
