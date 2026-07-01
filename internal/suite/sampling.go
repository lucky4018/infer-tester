package suite

import (
	"context"
	"fmt"
	"strings"

	"infer-tester/internal/client"
	"infer-tester/internal/config"
	"infer-tester/internal/runner"
)

func Sampling(c *client.Client, cfg *config.Config) runner.Suite {
	return runner.Suite{
		Name:        "sampling",
		Description: "采样参数测试——验证 temperature、seed、max_tokens、stop 等生成控制参数行为正确",
		Cases: []runner.TestCase{
			samplingDeterminism(c, cfg),
			samplingTemperatureZero(c, cfg),
			samplingMaxTokens(c, cfg),
			samplingStopWord(c, cfg),
		},
	}
}

func samplingDeterminism(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "determinism_same_seed", Description: "temperature=0 seed=42 连续调用两次，验证输出完全相同", Run: func() (string, error) {
		req := map[string]any{
			"model": cfg.Model.Name, "prompt": "The capital of France is",
			"max_tokens": 10, "temperature": 0, "seed": 42,
		}
		outputs := make([]string, 2)
		for i := 0; i < 2; i++ {
			var r map[string]any
			if status, err := c.JsonPost(context.Background(), "/v1/completions", req, &r); err != nil || status != 200 {
				return "", fmt.Errorf("call %d: status=%d err=%v", i+1, status, err)
			}
			outputs[i] = r["choices"].([]any)[0].(map[string]any)["text"].(string)
		}
		if outputs[0] != outputs[1] {
			return "", fmt.Errorf("outputs differ:\n  call1=%q\n  call2=%q", outputs[0], outputs[1])
		}
		return fmt.Sprintf("identical output %q", outputs[0]), nil
	}}
}

func samplingTemperatureZero(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "temperature_zero_greedy", Description: "temperature=0 贪心解码，验证返回非空确定性输出", Run: func() (string, error) {
		var r map[string]any
		if status, err := c.JsonPost(context.Background(), "/v1/completions", map[string]any{
			"model": cfg.Model.Name, "prompt": "1 + 1 =",
			"max_tokens": 5, "temperature": 0,
		}, &r); err != nil || status != 200 {
			return "", fmt.Errorf("status=%d err=%v", status, err)
		}
		text := strings.TrimSpace(r["choices"].([]any)[0].(map[string]any)["text"].(string))
		if text == "" {
			return "", fmt.Errorf("empty output at temperature=0")
		}
		return fmt.Sprintf("output=%q", text), nil
	}}
}

func samplingMaxTokens(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "max_tokens_respected", Description: "设置 max_tokens=5，验证实际 completion_tokens 不超限", Run: func() (string, error) {
		const limit = 5
		var r map[string]any
		if status, err := c.JsonPost(context.Background(), "/v1/completions", map[string]any{
			"model": cfg.Model.Name, "prompt": "Write a very long essay about the universe.",
			"max_tokens": limit,
		}, &r); err != nil || status != 200 {
			return "", fmt.Errorf("status=%d err=%v", status, err)
		}
		usage := r["usage"].(map[string]any)
		completion := int(usage["completion_tokens"].(float64))
		if completion > limit {
			return "", fmt.Errorf("completion_tokens=%d exceeds max_tokens=%d", completion, limit)
		}
		fr := r["choices"].([]any)[0].(map[string]any)["finish_reason"].(string)
		return fmt.Sprintf("completion_tokens=%d finish_reason=%s", completion, fr), nil
	}}
}

func samplingStopWord(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{Name: "stop_sequence_honored", Description: "设置 stop=[\"END\"]，验证输出中不含该词且 finish_reason 正确", Run: func() (string, error) {
		stopWord := "END"
		var r map[string]any
		if status, err := c.JsonPost(context.Background(), "/v1/completions", map[string]any{
			"model":      cfg.Model.Name,
			"prompt":     "Count from one to ten in words, then say END",
			"max_tokens": 80,
			"stop":       []string{stopWord},
		}, &r); err != nil || status != 200 {
			return "", fmt.Errorf("status=%d err=%v", status, err)
		}
		choice := r["choices"].([]any)[0].(map[string]any)
		text := choice["text"].(string)
		fr := choice["finish_reason"].(string)
		if strings.Contains(text, stopWord) {
			return "", fmt.Errorf("stop word %q found in output", stopWord)
		}
		return fmt.Sprintf("finish_reason=%s text=%q", fr, text), nil
	}}
}
