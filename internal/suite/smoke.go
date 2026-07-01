package suite

import (
	"context"
	"fmt"

	"infer-tester/internal/client"
	"infer-tester/internal/config"
	"infer-tester/internal/runner"
)

func Smoke(c *client.Client, cfg *config.Config) runner.Suite {
	return runner.Suite{
		Name:        "smoke",
		Description: "最小化验证推理引擎是否正常启动并能响应请求，所有其他测试的前置检查",
		Cases: []runner.TestCase{
			{
				Name:        "basic_inference",
				Description: "发送 Hello，验证模型返回非空文本，确认推理引擎基本可用",
				Run: func() (string, error) {
					var result map[string]any
					status, err := c.JsonPost(context.Background(), "/v1/completions", map[string]any{
						"model":      cfg.Model.Name,
						"prompt":     "Hello",
						"max_tokens": 10,
					}, &result)
					if err != nil {
						return "", err
					}
					if status != 200 {
						return "", fmt.Errorf("status %d", status)
					}
					choices, ok := result["choices"].([]any)
					if !ok || len(choices) == 0 {
						return "", fmt.Errorf("no choices in response")
					}
					text, _ := choices[0].(map[string]any)["text"].(string)
					if text == "" {
						return "", fmt.Errorf("empty output text")
					}
					return fmt.Sprintf("output: %q", text), nil
				},
			},
		},
	}
}
