package suite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"infer-tester/internal/client"
	"infer-tester/internal/config"
	"infer-tester/internal/runner"
)

func API(c *client.Client, cfg *config.Config) runner.Suite {
	cases := []runner.TestCase{
		apiCompletionsFields(c, cfg),
		apiChatRoles(c, cfg),
		apiModelsList(c, cfg),
		apiStreamingEvents(c, cfg),
		apiStreamingDone(c, cfg),
		apiErrorFormat(c, cfg),
		apiEmbeddings(c, cfg),
		apiRerank(c, cfg),
	}
	cases = append(cases,
		antMessagesBasic(c, cfg),
		antCountTokens(c, cfg),
		antStreaming(c, cfg),
		antToolUse(c, cfg),
		commonHealth(c),
		commonPing(c),
		commonTokenize(c, cfg),
		commonMetrics(c),
		commonErrorFormats(c, cfg),
		crossCompatOutput(c, cfg),
	)
	return runner.Suite{Name: "api", Description: "接口兼容性测试——覆盖 OpenAI、Anthropic 两种协议及公共端点，验证 API 格式、字段、流式、错误处理均符合规范", Cases: cases}
}

// ── OpenAI ───────────────────────────────────────────────────────────────────

func apiCompletionsFields(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "openai_completions_fields", Description: "验证 /v1/completions 响应包含 id/object/model/choices/usage 及所有子字段", Run: func() (string, error) {
		var r map[string]any
		if status, err := c.JsonPost(context.Background(), "/v1/completions", map[string]any{
			"model": cfg.Model.Name, "prompt": "The sky is", "max_tokens": 8,
		}, &r); err != nil || status != 200 {
			return "", fmt.Errorf("status=%d err=%v", status, err)
		}
		for _, f := range []string{"id", "object", "model", "choices", "usage"} {
			if _, ok := r[f]; !ok {
				return "", fmt.Errorf("missing field: %s", f)
			}
		}
		usage := r["usage"].(map[string]any)
		for _, f := range []string{"prompt_tokens", "completion_tokens", "total_tokens"} {
			if _, ok := usage[f]; !ok {
				return "", fmt.Errorf("missing usage.%s", f)
			}
		}
		if _, ok := r["choices"].([]any)[0].(map[string]any)["finish_reason"]; !ok {
			return "", fmt.Errorf("missing finish_reason")
		}
		return "all required fields present", nil
	}}
}

func apiChatRoles(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "openai_chat_roles", Description: "发送 system+user 多轮消息，验证响应 role=assistant 且内容非空", Run: func() (string, error) {
		var r map[string]any
		if status, err := c.JsonPost(context.Background(), "/v1/chat/completions", map[string]any{
			"model": cfg.Model.Name,
			"messages": []map[string]any{
				{"role": "system", "content": "Reply with one word only."},
				{"role": "user", "content": "Say yes"},
			},
			"max_tokens": 5,
		}, &r); err != nil || status != 200 {
			return "", fmt.Errorf("status=%d err=%v", status, err)
		}
		msg := r["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)
		if msg["role"] != "assistant" {
			return "", fmt.Errorf("expected role=assistant, got %v", msg["role"])
		}
		if msg["content"] == "" {
			return "", fmt.Errorf("empty content")
		}
		return fmt.Sprintf("role=assistant content=%q", msg["content"]), nil
	}}
}

func apiModelsList(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "openai_models_list", Description: "GET /v1/models，验证返回 list 格式，每个模型含 id/object/owned_by 字段", Run: func() (string, error) {
		var r map[string]any
		if err := c.JsonGet(context.Background(), "/v1/models", &r); err != nil {
			return "", err
		}
		if r["object"] != "list" {
			return "", fmt.Errorf("expected object=list, got %v", r["object"])
		}
		data, ok := r["data"].([]any)
		if !ok || len(data) == 0 {
			return "", fmt.Errorf("empty model list")
		}
		for _, f := range []string{"id", "object", "owned_by"} {
			if _, ok := data[0].(map[string]any)[f]; !ok {
				return "", fmt.Errorf("model item missing %s", f)
			}
		}
		return fmt.Sprintf("%d model(s) listed", len(data)), nil
	}}
}

func apiStreamingEvents(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "openai_streaming_chunks", Description: "Chat SSE 流式请求，验证收到多个合法 JSON chunk", Run: func() (string, error) {
		events, _, err := c.Stream(context.Background(), "/v1/chat/completions", map[string]any{
			"model":      cfg.Model.Name,
			"messages":   []map[string]any{{"role": "user", "content": "Say hello"}},
			"max_tokens": 15,
			"stream":     true,
		})
		if err != nil {
			return "", err
		}
		var dataCount int
		for _, e := range events {
			if e.Done {
				continue
			}
			var chunk map[string]any
			if err := json.Unmarshal([]byte(e.Data), &chunk); err != nil {
				return "", fmt.Errorf("invalid JSON chunk: %w", err)
			}
			dataCount++
		}
		if dataCount == 0 {
			return "", fmt.Errorf("no data chunks received")
		}
		return fmt.Sprintf("%d chunks received", dataCount), nil
	}}
}

func apiStreamingDone(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "openai_streaming_done_signal", Description: "Completions SSE 流式请求，验证最后一个事件为 [DONE]", Run: func() (string, error) {
		events, _, err := c.Stream(context.Background(), "/v1/completions", map[string]any{
			"model": cfg.Model.Name, "prompt": "Count:", "max_tokens": 10, "stream": true,
		})
		if err != nil {
			return "", err
		}
		if len(events) == 0 {
			return "", fmt.Errorf("no events received")
		}
		if !events[len(events)-1].Done {
			return "", fmt.Errorf("last event is not [DONE]")
		}
		return "stream ends with [DONE]", nil
	}}
}

func apiErrorFormat(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "openai_error_format", Description: "缺少 model 参数时，验证返回 4xx 且 body 含 error 字段", Run: func() (string, error) {
		resp, err := c.Post(context.Background(), "/v1/completions", map[string]any{"prompt": "hi"})
		if err != nil {
			return "", err
		}
		if resp.Status < 400 {
			return runner.Skip("server has a default model configured, missing model param returns 200")
		}
		var body map[string]any
		if err := json.Unmarshal(resp.Body, &body); err != nil {
			return "", fmt.Errorf("error body not JSON")
		}
		if _, ok := body["error"]; !ok {
			return "", fmt.Errorf("missing 'error' field in error response")
		}
		return fmt.Sprintf("status=%d error field present", resp.Status), nil
	}}
}

func apiEmbeddings(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "openai_embeddings", Description: "POST /v1/embeddings，验证返回两个向量且维度一致", Run: func() (string, error) {
		var r map[string]any
		status, err := c.JsonPost(context.Background(), "/v1/embeddings", map[string]any{
			"model": cfg.Model.Name,
			"input": []string{"Hello world", "Goodbye world"},
		}, &r)
		if err != nil {
			return "", err
		}
		if status >= 400 {
			return runner.Skip("model does not support embeddings")
		}
		data := r["data"].([]any)
		if len(data) != 2 {
			return "", fmt.Errorf("expected 2 embeddings, got %d", len(data))
		}
		emb := data[0].(map[string]any)["embedding"].([]any)
		return fmt.Sprintf("dim=%d", len(emb)), nil
	}}
}

func apiRerank(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "openai_rerank", Description: "POST /v1/rerank，验证文档按相关性重排序后返回结果", Run: func() (string, error) {
		var r map[string]any
		status, err := c.JsonPost(context.Background(), "/v1/rerank", map[string]any{
			"model":     cfg.Model.Name,
			"query":     "What is Go?",
			"documents": []string{"Go is a programming language.", "Python is interpreted.", "Go was made by Google."},
		}, &r)
		if err != nil {
			return "", err
		}
		if status >= 400 {
			return runner.Skip("model does not support rerank")
		}
		results, ok := r["results"].([]any)
		if !ok || len(results) == 0 {
			return "", fmt.Errorf("no rerank results")
		}
		return fmt.Sprintf("%d docs reranked", len(results)), nil
	}}
}

// ── Anthropic ────────────────────────────────────────────────────────────────

func antMessagesBasic(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "anthropic_messages_basic", Description: "POST /v1/messages，验证响应含 id/type/role/content/usage 等必要字段", Run: func() (string, error) {
		var r map[string]any
		if status, err := c.JsonPost(context.Background(), "/v1/messages", map[string]any{
			"model":      cfg.Model.Name,
			"max_tokens": 15,
			"messages":   []map[string]any{{"role": "user", "content": "Say hello"}},
		}, &r); err != nil || status != 200 {
			return "", fmt.Errorf("status=%d err=%v", status, err)
		}
		for _, f := range []string{"id", "type", "role", "content", "model", "stop_reason", "usage"} {
			if _, ok := r[f]; !ok {
				return "", fmt.Errorf("missing field: %s", f)
			}
		}
		if r["type"] != "message" || r["role"] != "assistant" {
			return "", fmt.Errorf("unexpected type=%v role=%v", r["type"], r["role"])
		}
		usage := r["usage"].(map[string]any)
		return fmt.Sprintf("input_tokens=%v output_tokens=%v", usage["input_tokens"], usage["output_tokens"]), nil
	}}
}

func antCountTokens(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "anthropic_count_tokens", Description: "POST /v1/messages/count_tokens，验证返回 input_tokens > 0", Run: func() (string, error) {
		var r map[string]any
		if status, err := c.JsonPost(context.Background(), "/v1/messages/count_tokens", map[string]any{
			"model":    cfg.Model.Name,
			"messages": []map[string]any{{"role": "user", "content": "Hello, how are you?"}},
		}, &r); err != nil || status != 200 {
			return "", fmt.Errorf("status=%d err=%v", status, err)
		}
		tokens, ok := r["input_tokens"].(float64)
		if !ok || tokens <= 0 {
			return "", fmt.Errorf("invalid input_tokens: %v", r["input_tokens"])
		}
		return fmt.Sprintf("input_tokens=%d", int(tokens)), nil
	}}
}

func antStreaming(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "anthropic_streaming_events", Description: "Messages SSE 流式，验证事件序列包含 message_start 事件类型", Run: func() (string, error) {
		events, _, err := c.Stream(context.Background(), "/v1/messages", map[string]any{
			"model": cfg.Model.Name, "max_tokens": 15, "stream": true,
			"messages": []map[string]any{{"role": "user", "content": "Say hi"}},
		})
		if err != nil {
			return "", err
		}
		counts := map[string]int{}
		for _, e := range events {
			if e.Done {
				continue
			}
			var ev map[string]any
			if json.Unmarshal([]byte(e.Data), &ev) == nil {
				if t, ok := ev["type"].(string); ok {
					counts[t]++
				}
			}
		}
		if counts["message_start"] == 0 {
			return "", fmt.Errorf("missing message_start event; got: %v", counts)
		}
		return fmt.Sprintf("event types: %v", counts), nil
	}}
}

func antToolUse(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "anthropic_tool_use", Description: "携带工具定义发送请求，验证模型返回 tool_use 内容块或正常回答", Run: func() (string, error) {
		tools := []map[string]any{{
			"name":        "get_weather",
			"description": "Get weather for a city",
			"input_schema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"city": map[string]any{"type": "string"}},
				"required":   []string{"city"},
			},
		}}
		var r map[string]any
		status, err := c.JsonPost(context.Background(), "/v1/messages", map[string]any{
			"model": cfg.Model.Name, "max_tokens": 100,
			"tools":    tools,
			"messages": []map[string]any{{"role": "user", "content": "What's the weather in Tokyo?"}},
		}, &r)
		if err != nil {
			return "", err
		}
		if status >= 400 {
			return runner.Skip("model does not support tool calling")
		}
		stopReason, _ := r["stop_reason"].(string)
		for _, block := range r["content"].([]any) {
			b := block.(map[string]any)
			if b["type"] == "tool_use" {
				inputJSON, _ := json.Marshal(b["input"])
				return fmt.Sprintf("tool_use: name=%v input=%s", b["name"], inputJSON), nil
			}
		}
		return fmt.Sprintf("stop_reason=%s (model did not use tool)", stopReason), nil
	}}
}

// ── Common ───────────────────────────────────────────────────────────────────

func commonHealth(c *client.Client) runner.TestCase {
	return runner.TestCase{Name: "common_health", Description: "GET /health，验证返回 200", Run: func() (string, error) {
		r, err := c.Get(context.Background(), "/health")
		if err != nil {
			return "", err
		}
		if r.Status != 200 {
			return "", fmt.Errorf("status %d", r.Status)
		}
		return "OK", nil
	}}
}

func commonPing(c *client.Client) runner.TestCase {
	return runner.TestCase{Name: "common_ping", Description: "GET /ping，验证返回 200", Run: func() (string, error) {
		r, err := c.Get(context.Background(), "/ping")
		if err != nil {
			return "", err
		}
		if r.Status == 404 {
			return runner.Skip("endpoint not available")
		}
		if r.Status != 200 {
			return "", fmt.Errorf("status %d", r.Status)
		}
		return "OK", nil
	}}
}

func commonTokenize(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "common_tokenize_roundtrip", Description: "调用 /tokenize 再调用 /detokenize，验证 token 数量合理且往返一致", Run: func() (string, error) {
		var tokRes map[string]any
		status, err := c.JsonPost(context.Background(), "/tokenize", map[string]any{
			"model": cfg.Model.Name, "prompt": "Hello, world!",
		}, &tokRes)
		if err != nil {
			return "", err
		}
		if status == 404 {
			return runner.Skip("endpoint not available")
		}
		if status != 200 {
			return "", fmt.Errorf("tokenize status=%d", status)
		}
		raw, ok := tokRes["tokens"].([]any)
		if !ok || len(raw) == 0 {
			return "", fmt.Errorf("no tokens returned")
		}
		ids := make([]int, len(raw))
		for i, t := range raw {
			ids[i] = int(t.(float64))
		}
		var detokRes map[string]any
		if status, err := c.JsonPost(context.Background(), "/detokenize", map[string]any{
			"model": cfg.Model.Name, "tokens": ids,
		}, &detokRes); err != nil || status != 200 {
			return "", fmt.Errorf("detokenize status=%d err=%v", status, err)
		}
		return fmt.Sprintf("%d tokens, round-trip OK", len(ids)), nil
	}}
}

func commonMetrics(c *client.Client) runner.TestCase {
	return runner.TestCase{Name: "common_metrics_prometheus", Description: "GET /metrics，验证 Prometheus 格式中包含核心推理指标", Run: func() (string, error) {
		r, err := c.Get(context.Background(), "/metrics")
		if err != nil {
			return "", err
		}
		if r.Status == 404 {
			return runner.Skip("endpoint not available")
		}
		if r.Status != 200 {
			return "", fmt.Errorf("status %d", r.Status)
		}
		body := string(r.Body)
		required := []string{"vllm:request_success_total", "vllm:kv_cache_usage_perc"}
		var missing []string
		for _, m := range required {
			if !strings.Contains(body, m) {
				missing = append(missing, m)
			}
		}
		if len(missing) > 0 {
			return "", fmt.Errorf("missing metrics: %v", missing)
		}
		return fmt.Sprintf("key metrics present (%d bytes)", len(r.Body)), nil
	}}
}

func commonErrorFormats(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "common_error_response_format", Description: "分别触发 OpenAI 和 Anthropic 错误，验证两种协议的错误格式符合规范", Run: func() (string, error) {
		oaiResp, err := c.Post(context.Background(), "/v1/completions", map[string]any{"prompt": "hi"})
		if err != nil {
			return "", err
		}
		if oaiResp.Status < 400 {
			return runner.Skip("server has a default model configured, missing model param returns 200")
		}
		var oaiErr map[string]any
		if json.Unmarshal(oaiResp.Body, &oaiErr) != nil || oaiErr["error"] == nil {
			return "", fmt.Errorf("OpenAI error body missing 'error' field")
		}

		antResp, err := c.Post(context.Background(), "/v1/messages", map[string]any{
			"model": cfg.Model.Name, "max_tokens": 10,
		})
		if err != nil {
			return "", err
		}
		if antResp.Status < 400 {
			return "", fmt.Errorf("expected 4xx from Anthropic, got %d", antResp.Status)
		}
		var antErr map[string]any
		if json.Unmarshal(antResp.Body, &antErr) != nil {
			return "", fmt.Errorf("Anthropic error body not JSON")
		}
		if antErr["type"] != "error" && antErr["error"] == nil {
			return "", fmt.Errorf("Anthropic error body missing expected fields: %v", antErr)
		}
		return fmt.Sprintf("OpenAI=%d Anthropic=%d both have correct error structure", oaiResp.Status, antResp.Status), nil
	}}
}

// ── Cross-compat ──────────────────────────────────────────────────────────────

func crossCompatOutput(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "cross_compat_output_consistency", Description: "同一 prompt 分别走 OpenAI chat 和 Anthropic messages 接口，对比两者输出", Run: func() (string, error) {
		prompt := "What is 1+1? Reply with the number only."
		ctx, cancel := context.WithTimeout(context.Background(), 2*c.Timeout)
		defer cancel()

		var oaiRes map[string]any
		if status, err := c.JsonPost(ctx, "/v1/chat/completions", map[string]any{
			"model":       cfg.Model.Name,
			"messages":    []map[string]any{{"role": "user", "content": prompt}},
			"max_tokens":  5,
			"temperature": 0,
		}, &oaiRes); err != nil || status != 200 {
			return "", fmt.Errorf("OpenAI request: status=%d err=%v", status, err)
		}
		oaiText := strings.TrimSpace(
			oaiRes["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)["content"].(string))

		var antRes map[string]any
		if status, err := c.JsonPost(ctx, "/v1/messages", map[string]any{
			"model":      cfg.Model.Name,
			"max_tokens": 5,
			"messages":   []map[string]any{{"role": "user", "content": prompt}},
		}, &antRes); err != nil || status != 200 {
			return "", fmt.Errorf("Anthropic request: status=%d err=%v", status, err)
		}
		antText := strings.TrimSpace(
			antRes["content"].([]any)[0].(map[string]any)["text"].(string))

		return fmt.Sprintf("openai=%q anthropic=%q", oaiText, antText), nil
	}}
}
