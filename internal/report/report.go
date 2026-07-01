package report

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"infer-tester/internal/runner"
)

// Write saves a Markdown report and a JSON report side by side.
// path may end in .md or .json; the sibling file is derived automatically.
func Write(path string, r runner.Report) error {
	if path == "" {
		return nil
	}
	var mdPath, jsonPath string
	switch {
	case strings.HasSuffix(path, ".md"):
		mdPath = path
		jsonPath = strings.TrimSuffix(path, ".md") + ".json"
	case strings.HasSuffix(path, ".json"):
		jsonPath = path
		mdPath = strings.TrimSuffix(path, ".json") + ".md"
	default:
		mdPath = path + ".md"
		jsonPath = path + ".json"
	}
	if err := os.WriteFile(mdPath, []byte(markdown(r)), 0o644); err != nil {
		return fmt.Errorf("write markdown report: %w", err)
	}
	jsonData, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if err := os.WriteFile(jsonPath, jsonData, 0o644); err != nil {
		return fmt.Errorf("write JSON report: %w", err)
	}
	return nil
}

// Paths returns the md and json paths that Write would produce for the given path.
func Paths(path string) (mdPath, jsonPath string) {
	switch {
	case strings.HasSuffix(path, ".md"):
		return path, strings.TrimSuffix(path, ".md") + ".json"
	case strings.HasSuffix(path, ".json"):
		return strings.TrimSuffix(path, ".json") + ".md", path
	default:
		return path + ".md", path + ".json"
	}
}

func PrintSummary(r runner.Report) {
	sep := strings.Repeat("─", 72)
	fmt.Printf("\n%s\n", sep)
	label := "PASSED"
	if r.Summary.Failed > 0 {
		label = "FAILED"
	}
	fmt.Printf(" %s  total=%-4d passed=%-4d failed=%-4d skipped=%-4d  (%s)\n",
		label, r.Summary.Total, r.Summary.Passed, r.Summary.Failed, r.Summary.Skipped, r.Duration)
	if r.Summary.Failed > 0 {
		fmt.Println("\n Failed cases:")
		for _, suite := range r.Suites {
			for _, c := range suite.Cases {
				if c.Status == runner.StatusFail {
					fmt.Printf("   [%s] %s\n     %s\n", suite.Name, c.Name, c.Error)
				}
			}
		}
	}
}

func markdown(r runner.Report) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# 推理引擎测试报告\n\n")

	fmt.Fprintf(&b, "| | |\n|---|---|\n")
	fmt.Fprintf(&b, "| **时间** | `%s` |\n", r.Timestamp)
	fmt.Fprintf(&b, "| **服务地址** | `%s` |\n", r.Server)
	fmt.Fprintf(&b, "| **模型** | `%s` |\n", r.Model)
	fmt.Fprintf(&b, "| **总耗时** | `%s` |\n", r.Duration)
	fmt.Fprintln(&b)

	if r.Summary.Failed > 0 {
		fmt.Fprintf(&b, "> [!CAUTION]\n> **失败用例：**\n")
		for _, suite := range r.Suites {
			for _, c := range suite.Cases {
				if c.Status == runner.StatusFail {
					fmt.Fprintf(&b, "> - `[%s] %s` — %s\n", suite.Name, c.Name, c.Error)
				}
			}
		}
		fmt.Fprintln(&b)
	}

	for _, suite := range r.Suites {
		suiteStatus := "✅"
		if suite.Failed > 0 {
			suiteStatus = "❌"
		}
		fmt.Fprintf(&b, "## %s %s\n\n", suiteStatus, suite.Name)
		fmt.Fprintf(&b, "通过：**%d** &nbsp; 失败：**%d** &nbsp; 跳过：**%d**\n\n",
			suite.Passed, suite.Failed, suite.Skipped)

		fmt.Fprintf(&b, "| 状态 | 用例 | 耗时 | 详情 |\n")
		fmt.Fprintf(&b, "|:---:|---|---:|---|\n")
		for _, c := range suite.Cases {
			icon := map[runner.Status]string{
				runner.StatusPass: "✅",
				runner.StatusFail: "❌",
				runner.StatusSkip: "⏭",
			}[c.Status]
			detail := c.Detail
			if c.Status == runner.StatusFail {
				detail = "`" + c.Error + "`"
			}
			fmt.Fprintf(&b, "| %s | `%s` | %dms | %s |\n", icon, c.Name, c.DurationMs, detail)
		}
		fmt.Fprintln(&b)
	}

	return b.String()
}
