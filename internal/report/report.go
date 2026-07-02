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

	// === 一、功能性测试 ===
	functionalSuites := map[string]bool{"smoke": true, "api": true, "sampling": true, "features": true, "concurrency": true}
	hasFunctional := false
	for _, suite := range r.Suites {
		if functionalSuites[suite.Name] {
			hasFunctional = true
			break
		}
	}
	if hasFunctional {
		fmt.Fprintf(&b, "## 一、功能性测试\n\n")
		for _, suite := range r.Suites {
			if !functionalSuites[suite.Name] {
				continue
			}
			suiteStatus := "✅"
			if suite.Failed > 0 {
				suiteStatus = "❌"
			}
			fmt.Fprintf(&b, "### %s %s\n\n", suiteStatus, suite.Name)
			fmt.Fprintf(&b, "通过：**%d**  失败：**%d**  跳过：**%d**\n\n",
				suite.Passed, suite.Failed, suite.Skipped)

			fmt.Fprintf(&b, "| 状态 | 用例 | 耗时 | 详情 |\n")
			fmt.Fprintf(&b, "|:---:|---|---:|---|\n")
			for _, c := range suite.Cases {
				icon := statusIcon(c.Status)
				detail := c.Detail
				if c.Status == runner.StatusFail {
					detail = "`" + c.Error + "`"
				}
				fmt.Fprintf(&b, "| %s | `%s` | %dms | %s |\n", icon, c.Name, c.DurationMs, detail)
			}
			fmt.Fprintln(&b)
		}
	}

	// === 二、轻量性能度量 ===
	for _, suite := range r.Suites {
		if suite.Name != "performance" {
			continue
		}
		var lightCases []runner.CaseResult
		for _, c := range suite.Cases {
			if !isBenchCase(c.Name) {
				lightCases = append(lightCases, c)
			}
		}
		if len(lightCases) == 0 {
			break
		}
		passed, failed, skipped := 0, 0, 0
		for _, c := range lightCases {
			switch c.Status {
			case runner.StatusPass:
				passed++
			case runner.StatusFail:
				failed++
			case runner.StatusSkip:
				skipped++
			}
		}
		fmt.Fprintf(&b, "## 二、轻量性能度量\n\n")
		fmt.Fprintf(&b, "通过：**%d**  失败：**%d**  跳过：**%d**\n\n", passed, failed, skipped)
		fmt.Fprintf(&b, "| 状态 | 用例 | 耗时 | 详情 |\n")
		fmt.Fprintf(&b, "|:---:|---|---:|---|\n")
		for _, c := range lightCases {
			icon := statusIcon(c.Status)
			detail := c.Detail
			if c.Status == runner.StatusFail {
				detail = "`" + c.Error + "`"
			}
			fmt.Fprintf(&b, "| %s | `%s` | %dms | %s |\n", icon, c.Name, c.DurationMs, detail)
		}
		fmt.Fprintln(&b)
		break
	}

	// === 三、饱和压测 ===
	var benchCases []runner.CaseResult
	for _, suite := range r.Suites {
		if suite.Name == "vllm_bench" {
			for _, c := range suite.Cases {
				if isBenchCase(c.Name) {
					benchCases = append(benchCases, c)
				}
			}
		}
	}
	for _, suite := range r.Suites {
		if suite.Name == "performance" {
			for _, c := range suite.Cases {
				if isBenchCase(c.Name) {
					benchCases = append(benchCases, c)
				}
			}
		}
	}
	if len(benchCases) > 0 {
		fmt.Fprintf(&b, "## 三、饱和压测\n\n")
		fmt.Fprintf(&b, "| 状态 | 用例 | 耗时 | 详情 |\n")
		fmt.Fprintf(&b, "|:---:|---|---:|---|\n")
		for _, c := range benchCases {
			icon := statusIcon(c.Status)
			detail := c.Detail
			if c.Status == runner.StatusFail {
				detail = "`" + c.Error + "`"
			} else if detail != "" {
				detail = benchShortSummary(detail)
			}
			fmt.Fprintf(&b, "| %s | `%s` | %dms | %s |\n", icon, c.Name, c.DurationMs, detail)
		}
		fmt.Fprintln(&b)

		// 渲染每个 bench 用例的详细报告
		for _, c := range benchCases {
			if c.Status == runner.StatusPass && c.Detail != "" {
				b.WriteString(renderBenchDetail(c.Name, c.Detail))
			}
		}
	}

	return b.String()
}

// statusIcon 返回状态对应的图标
func statusIcon(s runner.Status) string {
	switch s {
	case runner.StatusPass:
		return "✅"
	case runner.StatusFail:
		return "❌"
	case runner.StatusSkip:
		return "⏭"
	default:
		return "?"
	}
}

// isBenchCase 判断用例是否为 bench 相关用例
func isBenchCase(name string) bool {
	return strings.Contains(name, "bench_serving") || strings.Contains(name, "bench_serve")
}

// benchShortSummary 从 detail 中提取简短摘要用于表格行
func benchShortSummary(detail string) string {
	for _, field := range strings.Fields(detail) {
		if strings.HasPrefix(field, "tok/s=") {
			return "吞吐量 " + strings.TrimPrefix(field, "tok/s=") + " tok/s"
		}
	}
	if len(detail) > 80 {
		return detail[:80] + "..."
	}
	return detail
}

type kvPair struct {
	key string
	val string
}

// parseSpaceKV 解析空格分隔的 key=value 对
func parseSpaceKV(s string) []kvPair {
	var pairs []kvPair
	for _, field := range strings.Fields(s) {
		if idx := strings.Index(field, "="); idx > 0 {
			pairs = append(pairs, kvPair{key: field[:idx], val: field[idx+1:]})
		}
	}
	return pairs
}

// renderBenchDetail 将 bench detail 格式化为 markdown 可折叠表格区块
func renderBenchDetail(caseName, detail string) string {
	segments := strings.Split(detail, " | ")

	var vllmCfg, probes, summary, rawLog string
	var latencySegs []string

	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		switch {
		case strings.HasPrefix(seg, "vllm{"):
			vllmCfg = seg
		case strings.HasPrefix(seg, "probes{"):
			probes = seg
		case strings.HasPrefix(seg, "TTFT(ms)"),
			strings.HasPrefix(seg, "TPOT(ms)"),
			strings.HasPrefix(seg, "ITL(ms)"):
			latencySegs = append(latencySegs, seg)
		case strings.HasPrefix(seg, "raw_log="):
			rawLog = seg
		default:
			summary = seg
		}
	}

	var b strings.Builder

	// 从 summary 中提取 tok/s 作为最终吞吐量
	tokPerSec := ""
	if summary != "" {
		for _, kv := range parseSpaceKV(summary) {
			if kv.key == "tok/s" {
				tokPerSec = kv.val
			}
		}
	}

	// vllm_bench_serve: vLLM 官方工具压测
	if strings.Contains(caseName, "vllm_bench_serve") {
		b.WriteString("### vLLM 官方 Benchmark Result\n\n")
		// 提取成功请求数
		numPrompts := ""
		if summary != "" {
			for _, kv := range parseSpaceKV(summary) {
				if kv.key == "ok" {
					numPrompts = kv.val
				}
			}
		}
		// 提取 best_rate
		bestRate := ""
		if probes != "" {
			closeIdx := strings.Index(probes, "}")
			if closeIdx >= 0 {
				rest := strings.TrimSpace(probes[closeIdx+1:])
				for _, kv := range parseSpaceKV(rest) {
					if kv.key == "best_rate" {
						bestRate = kv.val
					}
				}
			}
		}
		b.WriteString("**测试说明**\n\n")
		b.WriteString("> 本测试通过 Docker 容器调用 vLLM 官方 bench serve 工具进行压测，目标是将 GPU/NPU 算力压至饱和状态，测量推理服务的最大吞吐性能。")
		b.WriteString("该工具使用 vLLM 内置的 Poisson 流量模型模拟真实请求到达模式。测试过程中逐步提升请求速率（request-rate: 8→16→32→...），每轮运行一次 bench serve，观察吞吐量随速率的变化趋势，")
		b.WriteString("当继续提升速率而吞吐量不再增长（提升不足 10%）时，判定硬件已达到性能瓶颈。")
		if bestRate != "" {
			b.WriteString(fmt.Sprintf("最优请求速率为 **%s**，", bestRate))
		}
		if numPrompts != "" && numPrompts != "0" {
			b.WriteString(fmt.Sprintf("每轮并发请求数为 **%s**，", numPrompts))
		}
		if tokPerSec != "" && tokPerSec != "0.00" {
			b.WriteString(fmt.Sprintf("Token 吞吐量最终达到 **%s tok/s**，即为该配置下的性能基线。", tokPerSec))
		}
		b.WriteString("\n\n")
	} else {
		// bench_serving: 本程序内置实现
		b.WriteString("### 自适应 Benchmark Result\n\n")
		if probes != "" {
			b.WriteString("**测试说明**\n\n")
			b.WriteString("> 本测试由 infer-tester 内置实现，无需 Docker，通过 HTTP 接口直接对推理服务进行自适应并发度探测压测，目标是将 GPU/NPU 算力压至饱和状态。")
			b.WriteString("测试过程从低并发开始阶梯式加倍（8→16→32→...），每轮发送探测请求测量吞吐量，")
			b.WriteString("当吞吐量提升不足 10% 或出现下降时判定到达拐点，")
			b.WriteString("以该最优并发度执行正式压测，获取最终性能数据。\n\n")
		}
	}

	// vLLM 运行时配置
	if vllmCfg != "" {
		openIdx := strings.Index(vllmCfg, "{")
		closeIdx := strings.LastIndex(vllmCfg, "}")
		if openIdx >= 0 && closeIdx > openIdx {
			inner := vllmCfg[openIdx+1 : closeIdx]
			kvs := parseSpaceKV(inner)
			b.WriteString("**vLLM 运行时配置**\n\n")
			b.WriteString("| 参数 | 值 |\n|---|---|\n")
			labels := map[string]string{
				"gpu_mem":      "GPU 显存利用率",
				"kv_blocks":    "KV Cache Block 数",
				"prefix_cache": "前缀缓存",
			}
			for _, kv := range kvs {
				if label, ok := labels[kv.key]; ok {
					b.WriteString(fmt.Sprintf("| %s | `%s` |\n", label, kv.val))
				}
			}
			b.WriteString("\n")
		}
	}

	// 探测过程（vllm_bench_serve 和 bench_serving 共用）
	if probes != "" {
		openIdx := strings.Index(probes, "{")
		closeIdx := strings.Index(probes, "}")
		if openIdx >= 0 && closeIdx > openIdx {
			inner := probes[openIdx+1 : closeIdx]
			rest := strings.TrimSpace(probes[closeIdx+1:])
			bestVal := ""
			for _, kv := range parseSpaceKV(rest) {
				if kv.key == "best_c" || kv.key == "best_rate" {
					bestVal = kv.val
				}
			}

			isRate := strings.Contains(caseName, "vllm_bench_serve")
			if isRate {
				b.WriteString("**请求速率探测**\n\n")
				b.WriteString("| 请求速率 (req/s) | 吞吐量 (tok/s) |\n|---|---|\n")
			} else {
				b.WriteString("**自适应并发度探测**\n\n")
				b.WriteString("| 并发度 | 吞吐量 (tok/s) |\n|---|---|\n")
			}
			for _, entry := range strings.Fields(inner) {
				parts := strings.SplitN(entry, ":", 2)
				if len(parts) == 2 {
					key := parts[0]
					key = strings.TrimPrefix(key, "c=")
					key = strings.TrimPrefix(key, "rate=")
					tps := strings.TrimSuffix(parts[1], "tok/s")
					b.WriteString(fmt.Sprintf("| %s | %s |\n", key, tps))
				}
			}
			if bestVal != "" {
				if isRate {
					b.WriteString(fmt.Sprintf("\n最优请求速率：**%s**\n", bestVal))
				} else {
					b.WriteString(fmt.Sprintf("\n最优并发度：**%s**\n", bestVal))
				}
			}
			b.WriteString("\n")
		}
	}

	// 压测汇总
	if summary != "" {
		kvs := parseSpaceKV(summary)
		b.WriteString("**压测汇总**\n\n")
		b.WriteString("| 指标 | 值 |\n|---|---|\n")
		labels := map[string]string{
			"image": "镜像",
			"n":     "总请求数",
			"ok":    "成功请求数",
			"fail":  "失败请求数",
			"dur":   "持续时间",
			"req/s": "请求吞吐 (req/s)",
			"tok/s": "Token 吞吐 (tok/s)",
		}
		for _, kv := range kvs {
			if label, ok := labels[kv.key]; ok {
				b.WriteString(fmt.Sprintf("| %s | `%s` |\n", label, kv.val))
			}
		}
		if rawLog != "" {
			logFile := strings.TrimPrefix(rawLog, "raw_log=")
			b.WriteString(fmt.Sprintf("| 原始日志 | `%s` |\n", logFile))
		}
		b.WriteString("\n")
	}

	// 延迟指标
	if len(latencySegs) > 0 {
		hasP90 := false
		for _, seg := range latencySegs {
			if strings.Contains(seg, "p90=") {
				hasP90 = true
				break
			}
		}

		b.WriteString("**延迟指标 (ms)**\n\n")
		headers := []string{"指标", "平均值", "P50 (中位)"}
		if hasP90 {
			headers = append(headers, "P90")
		}
		headers = append(headers, "P99")

		b.WriteString("| " + strings.Join(headers, " | ") + " |\n")
		b.WriteString("|" + strings.Repeat("---|", len(headers)) + "\n")

		latencyLabels := map[string]string{
			"TTFT": "首Token延迟 (TTFT)",
			"TPOT": "每Token延迟 (TPOT)",
			"ITL":  "Token间延迟 (ITL)",
		}
		for _, seg := range latencySegs {
			spaceIdx := strings.Index(seg, " ")
			if spaceIdx < 0 {
				continue
			}
			label := strings.TrimSuffix(seg[:spaceIdx], "(ms)")
			if cn, ok := latencyLabels[label]; ok {
				label = cn
			}
			kvs := parseSpaceKV(seg[spaceIdx+1:])

			var meanVal, p50Val, p90Val, p99Val string
			for _, kv := range kvs {
				switch kv.key {
				case "mean":
					meanVal = kv.val
				case "p50":
					p50Val = kv.val
				case "p90":
					p90Val = kv.val
				case "p99":
					p99Val = kv.val
				}
			}

			row := []string{label, meanVal, p50Val}
			if hasP90 {
				row = append(row, p90Val)
			}
			row = append(row, p99Val)
			b.WriteString("| " + strings.Join(row, " | ") + " |\n")
		}
		b.WriteString("\n> **指标说明**\n> - **TTFT** (Time To First Token): 从发送请求到收到第一个Token的延迟\n> - **TPOT** (Time Per Output Token): 生成阶段每个输出Token的平均耗时\n> - **ITL** (Inter-Token Latency): 相邻两个Token之间的时间间隔\n> - **平均值**: 所有请求该指标的算术平均，易被大量正常请求稀释\n> - **P50** (中位数): 50%的请求延迟低于此值，代表典型用户体验\n> - **P90**: 90%的请求延迟低于此值，10个请求中最慢的1个\n> - **P99**: 99%的请求延迟低于此值，100个请求中最慢的1个，暴露长尾延迟\n")
	}

	b.WriteString("\n")
	return b.String()
}
