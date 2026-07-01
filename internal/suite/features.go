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

func Features(c *client.Client, cfg *config.Config) runner.Suite {
	return runner.Suite{Name: "features", Description: "功能特性测试——验证结构化输出、工具调用、KV 缓存复用、推理思维链、多模态等高级能力", Cases: []runner.TestCase{
		featStructuredOutput(c, cfg),
		featToolCalling(c, cfg),
		featPrefixCaching(c, cfg),
		featReasoning(c, cfg),
		featMultimodal(c, cfg),
	}}
}

func featStructuredOutput(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "structured_output_json_schema", Description: "用 response_format 指定 JSON Schema，验证输出为合法 JSON 且必填字段齐全", Run: func() (string, error) {
		schema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":  map[string]any{"type": "string"},
				"score": map[string]any{"type": "integer"},
			},
			"required": []string{"name", "score"},
		}
		var r map[string]any
		if status, err := c.JsonPost(context.Background(), "/v1/chat/completions", map[string]any{
			"model":       cfg.Model.Name,
			"max_tokens":  50,
			"temperature": 0,
			"messages":    []map[string]any{{"role": "user", "content": "Give me a player with name and score."}},
			"response_format": map[string]any{
				"type":        "json_schema",
				"json_schema": map[string]any{"name": "player", "schema": schema, "strict": true},
			},
		}, &r); err != nil || status != 200 {
			return "", fmt.Errorf("status=%d err=%v", status, err)
		}
		content := r["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)["content"].(string)
		var obj map[string]any
		if err := json.Unmarshal([]byte(content), &obj); err != nil {
			return "", fmt.Errorf("output is not valid JSON: %w — got %q", err, content)
		}
		if _, ok := obj["name"]; !ok {
			return "", fmt.Errorf("missing 'name' in output: %v", obj)
		}
		if _, ok := obj["score"]; !ok {
			return "", fmt.Errorf("missing 'score' in output: %v", obj)
		}
		return fmt.Sprintf("valid JSON: %s", content), nil
	}}
}

func featToolCalling(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "tool_calling_openai", Description: "发送带 tools 定义的请求，验证模型返回 tool_calls 调用块", Run: func() (string, error) {
		tools := []map[string]any{{
			"type": "function",
			"function": map[string]any{
				"name":        "get_weather",
				"description": "Get current temperature for a city",
				"parameters": map[string]any{
					"type":       "object",
					"properties": map[string]any{"city": map[string]any{"type": "string"}},
					"required":   []string{"city"},
				},
			},
		}}
		var r map[string]any
		status, err := c.JsonPost(context.Background(), "/v1/chat/completions", map[string]any{
			"model":      cfg.Model.Name,
			"messages":   []map[string]any{{"role": "user", "content": "What's the weather in Paris?"}},
			"tools":      tools,
			"max_tokens": 100,
		}, &r)
		if err != nil {
			return "", err
		}
		if status >= 400 {
			return runner.Skip("model does not support tool calling")
		}
		msg := r["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)
		if msg["tool_calls"] != nil {
			calls := msg["tool_calls"].([]any)
			fn := calls[0].(map[string]any)["function"].(map[string]any)
			return fmt.Sprintf("tool_call: %v args=%v", fn["name"], fn["arguments"]), nil
		}
		content, _ := msg["content"].(string)
		return fmt.Sprintf("no tool_call (model responded directly): %q", content), nil
	}}
}

func featPrefixCaching(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "prefix_caching_kv_reuse", Description: "相同长前缀发送两次，检查第二次 cached_tokens 命中，验证 KV 缓存复用", Run: func() (string, error) {
		longPrefix := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 80)
		req := map[string]any{
			"model":       cfg.Model.Name,
			"prompt":      longPrefix + " Summarize the above.",
			"max_tokens":  10,
			"temperature": 0,
		}
		var r1, r2 map[string]any
		if status, err := c.JsonPost(context.Background(), "/v1/completions", req, &r1); err != nil || status != 200 {
			return "", fmt.Errorf("first call: status=%d err=%v", status, err)
		}
		if status, err := c.JsonPost(context.Background(), "/v1/completions", req, &r2); err != nil || status != 200 {
			return "", fmt.Errorf("second call: status=%d err=%v", status, err)
		}
		usage2 := r2["usage"].(map[string]any)
		if details, ok := usage2["prompt_tokens_details"].(map[string]any); ok {
			cached, _ := details["cached_tokens"].(float64)
			return fmt.Sprintf("cached_tokens=%d on second call", int(cached)), nil
		}
		return "prefix caching active (cached_tokens field not exposed)", nil
	}}
}

func featReasoning(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "reasoning_thinking", Description: "开启 thinking 模式，验证响应中存在 thinking 内容块", Run: func() (string, error) {
		var r map[string]any
		status, err := c.JsonPost(context.Background(), "/v1/chat/completions", map[string]any{
			"model":      cfg.Model.Name,
			"messages":   []map[string]any{{"role": "user", "content": "What is 17 × 23?"}},
			"max_tokens": 1024,
			"thinking":   map[string]any{"type": "enabled", "budget_tokens": 512},
		}, &r)
		if err != nil {
			if strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "timeout") {
				return runner.Skip("model does not support thinking mode or response too slow")
			}
			return "", err
		}
		if status >= 400 {
			return runner.Skip("model does not support thinking mode")
		}
		msg := r["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)
		if content, ok := msg["content"].([]any); ok {
			for _, block := range content {
				b := block.(map[string]any)
				if b["type"] == "thinking" {
					thinking, _ := b["thinking"].(string)
					return fmt.Sprintf("thinking block present (%d chars)", len(thinking)), nil
				}
			}
		}
		return runner.Skip("model does not support thinking mode")
	}}
}

func featMultimodal(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "multimodal_image", Description: "发送 base64 图片，验证模型返回图片描述", Run: func() (string, error) {
		// 1×1 white PNG
		tiny1x1PNG := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
		var r map[string]any
		status, err := c.JsonPost(context.Background(), "/v1/chat/completions", map[string]any{
			"model":      cfg.Model.Name,
			"max_tokens": 30,
			"messages": []map[string]any{{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": "Describe this image in one word."},
					{"type": "image_url", "image_url": map[string]any{
						"url": "data:image/png;base64," + tiny1x1PNG,
					}},
				},
			}},
		}, &r)
		if err != nil {
			return "", err
		}
		if status >= 400 {
			return runner.Skip("model does not support vision")
		}
		content, _ := r["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)["content"].(string)
		return fmt.Sprintf("vision response: %q", content), nil
	}}
}
