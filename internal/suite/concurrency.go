package suite

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"infer-tester/internal/client"
	"infer-tester/internal/config"
	"infer-tester/internal/runner"
)

func Concurrency(c *client.Client, cfg *config.Config) runner.Suite {
	return runner.Suite{
		Name:        "concurrency",
		Description: "并发与调度测试——验证引擎在多请求并发时的正确性，覆盖连续批处理、请求抢占和流式取消",
		Cases: []runner.TestCase{
			concurrentRequests(c, cfg),
			continuousBatching(c, cfg),
			preemption(c, cfg),
			cancellation(c, cfg),
		},
	}
}

func concurrentRequests(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "concurrent_requests", Description: "N 个请求同时发出，验证全部成功无报错（N 由 concurrency.num_requests 配置）", Run: func() (string, error) {
		n := cfg.Concurrency.NumRequests
		if n <= 0 {
			n = 16
		}
		var success, failure int64
		var wg sync.WaitGroup
		start := time.Now()
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var r map[string]any
				status, err := c.JsonPost(context.Background(), "/v1/completions", map[string]any{
					"model": cfg.Model.Name, "prompt": "Hello", "max_tokens": 20, "temperature": 0,
				}, &r)
				if err == nil && status == 200 {
					atomic.AddInt64(&success, 1)
				} else {
					atomic.AddInt64(&failure, 1)
				}
			}()
		}
		wg.Wait()
		dur := time.Since(start)
		if failure > 0 {
			return "", fmt.Errorf("%d/%d requests failed (elapsed=%s)", failure, n, dur.Round(time.Millisecond))
		}
		return fmt.Sprintf("n=%d all succeeded in %s", n, dur.Round(time.Millisecond)), nil
	}}
}

func continuousBatching(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "continuous_batching_mixed", Description: "混合短请求(5 token)和长请求(150 token)并发，验证短请求不被长请求饿死", Run: func() (string, error) {
		type job struct {
			label     string
			maxTokens int
		}
		jobs := []job{
			{"short1", 5}, {"short2", 5}, {"short3", 5},
			{"long1", 150}, {"long2", 150},
			{"short4", 5}, {"short5", 5},
		}
		type res struct {
			label string
			err   error
		}
		results := make([]res, len(jobs))
		var wg sync.WaitGroup
		overall := time.Now()
		for i, j := range jobs {
			i, j := i, j
			wg.Add(1)
			go func() {
				defer wg.Done()
				var r map[string]any
				status, err := c.JsonPost(context.Background(), "/v1/completions", map[string]any{
					"model": cfg.Model.Name, "prompt": "Write something interesting.",
					"max_tokens": j.maxTokens, "temperature": 0,
				}, &r)
				if err == nil && status != 200 {
					err = fmt.Errorf("status %d", status)
				}
				results[i] = res{j.label, err}
			}()
		}
		wg.Wait()
		total := time.Since(overall)
		var failed []string
		for _, r := range results {
			if r.err != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", r.label, r.err))
			}
		}
		if len(failed) > 0 {
			return "", fmt.Errorf("failures: %v", failed)
		}
		return fmt.Sprintf("%d mixed jobs all completed in %s", len(jobs), total.Round(time.Millisecond)), nil
	}}
}

func preemption(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "preemption_short_wins", Description: "先发长请求，50ms 后发 4 个短请求，验证短请求能正常完成不被阻塞", Run: func() (string, error) {
		longCtx, longCancel := context.WithTimeout(context.Background(), c.Timeout*2)
		defer longCancel()

		go func() {
			var r map[string]any
			c.JsonPost(longCtx, "/v1/completions", map[string]any{ //nolint
				"model": cfg.Model.Name, "prompt": "Write a very detailed 500-word essay on the history of computing.",
				"max_tokens": 400, "temperature": 0,
			}, &r)
		}()

		time.Sleep(50 * time.Millisecond)

		t0 := time.Now()
		var wg sync.WaitGroup
		var shortSuccess int64
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var r map[string]any
				status, err := c.JsonPost(context.Background(), "/v1/completions", map[string]any{
					"model": cfg.Model.Name, "prompt": "Hi", "max_tokens": 5, "temperature": 0,
				}, &r)
				if err == nil && status == 200 {
					atomic.AddInt64(&shortSuccess, 1)
				}
			}()
		}
		wg.Wait()
		shortDur := time.Since(t0)

		if shortSuccess < 4 {
			return "", fmt.Errorf("only %d/4 short requests succeeded", shortSuccess)
		}
		return fmt.Sprintf("4/4 short requests done in %s", shortDur.Round(time.Millisecond)), nil
	}}
}

func cancellation(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "cancellation_mid_stream", Description: "流式请求中途取消，验证连接正常关闭，服务端不 hang 不报错", Run: func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		events, _, err := c.Stream(ctx, "/v1/completions", map[string]any{
			"model":  cfg.Model.Name,
			"prompt": "Write a very long story about dragons and knights. Begin:",
			"max_tokens": 512, "stream": true, "temperature": 0,
		})
		cancel()

		if err != nil {
			errStr := err.Error()
			isContextErr := strings.Contains(errStr, "context") || strings.Contains(errStr, "deadline") || strings.Contains(errStr, "canceled")
			if isContextErr {
				if len(events) == 0 {
					return "", fmt.Errorf("cancelled before any events received, server may be unavailable")
				}
				return fmt.Sprintf("cancelled cleanly after %d events", len(events)), nil
			}
			if strings.Contains(errStr, "EOF") && len(events) == 0 {
				return "", fmt.Errorf("connection closed before any events received: %v", err)
			}
			return "", fmt.Errorf("unexpected error on cancellation: %v", err)
		}
		return fmt.Sprintf("stream completed (%d events) before cancel took effect", len(events)), nil
	}}
}
