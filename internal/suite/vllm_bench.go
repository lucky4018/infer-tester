package suite

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"infer-tester/internal/client"
	"infer-tester/internal/config"
	"infer-tester/internal/runner"
)


func VllmBench(c *client.Client, cfg *config.Config) runner.Suite {
	return runner.Suite{
		Name:        "vllm_bench",
		Description: "在临时 Docker 容器中运行 vllm bench serve 精确压测，完成后自动销毁容器",
		Cases: []runner.TestCase{
			vllmBenchServe(c, cfg),
		},
	}
}

func vllmBenchServe(c *client.Client, cfg *config.Config) runner.TestCase {
	return runner.TestCase{
		Name:        "vllm_bench_serve",
		Description: "检测宿主机 vllm 镜像 → 逐步提升 request-rate 运行 vllm bench serve → 找到吞吐量饱和点 → 解析结果",
		Run: func() (string, error) {
			// 1. 检测 docker 是否可用（二进制 + daemon + 权限）
			if _, err := exec.LookPath("docker"); err != nil {
				return runner.Skip("docker 未安装（未在 PATH 中找到 docker 命令），跳过 vllm bench serve 测试")
			}

			// 2. 确定使用的 vllm 镜像
			// 用 CombinedOutput 同时捕获 stdout 和 stderr，daemon 异常时能拿到真实错误
			listRaw, err := exec.Command("docker", "images",
				"--format", "{{.Repository}}:{{.Tag}}").CombinedOutput()
			listStr := strings.TrimSpace(string(listRaw))
			if err != nil {
				reason := dockerErrReason(listStr)
				return runner.Skip(fmt.Sprintf("docker 不可用（%s），跳过 vllm bench serve 测试", reason))
			}
			available := strings.Split(listStr, "\n")

			findVllmImage := func() string {
				for _, line := range available {
					line = strings.TrimSpace(line)
					low := strings.ToLower(line)
					if (strings.HasPrefix(low, "vllm") || strings.Contains(low, "/vllm")) &&
						!strings.Contains(line, "<none>") {
						return line
					}
				}
				return ""
			}

			image := cfg.VllmBench.Image
			if image != "" {
				// 验证配置的镜像是否实际存在
				found := false
				for _, line := range available {
					if strings.TrimSpace(line) == image {
						found = true
						break
					}
				}
				if !found {
					// 配置的镜像不存在，回退到自动检测
					fmt.Printf("  [vllm_bench] 配置的镜像 %q 不存在，尝试自动检测...\n", image)
					image = findVllmImage()
					if image != "" {
						fmt.Printf("  [vllm_bench] 自动选择镜像: %s\n", image)
					}
				}
			} else {
				// 未配置，直接自动检测
				image = findVllmImage()
				if image != "" {
					fmt.Printf("  [vllm_bench] 自动选择镜像: %s\n", image)
				}
			}

			if image == "" {
				return runner.Skip(
					"宿主机上未找到 vllm Docker 镜像，无法运行 vllm bench serve。\n" +
						"请在 config.json 中配置镜像名，例如：\n" +
						`  "docker": {"vllm_image": "vllm-ascend:v0.20.2rc"}` + "\n" +
						"或先拉取镜像：docker pull quay.io/ascend/vllm-ascend:v0.22.1rc1")
			}

			// 3. 解析服务地址
			u, err := url.Parse(cfg.Server.BaseURL)
			if err != nil {
				return "", fmt.Errorf("解析 base_url 失败: %v", err)
			}
			host := u.Hostname()
			port := u.Port()
			if port == "" {
				if u.Scheme == "https" {
					port = "443"
				} else {
					port = "80"
				}
			}

			// 4. 构造 docker run 基础参数（--rm 确保容器运行完自动删除）
			const containerModelDir = "/mnt/models"
			dockerArgs := []string{"run", "--rm", "--network=host"}
			if cfg.VllmBench.ModelDir != "" {
				dockerArgs = append(dockerArgs, "-v", cfg.VllmBench.ModelDir+":"+containerModelDir)
			}
			dockerArgs = append(dockerArgs, image,
				"python3", "-m", "vllm.entrypoints.cli.main", "bench", "serve")

			// 5. 解析用户配置的 args，过滤掉 --request-rate（探测时动态注入）
			userArgs := strings.Fields(cfg.VllmBench.Args)
			var filteredArgs []string
			for i := 0; i < len(userArgs); i++ {
				if userArgs[i] == "--request-rate" || userArgs[i] == "--rate" {
					i++ // 跳过值
					continue
				}
				filteredArgs = append(filteredArgs, userArgs[i])
			}

			// tokenizer 参数处理
			tokenizerArg := ""
			if cfg.VllmBench.Tokenizer != "" {
				tokenizerPath := cfg.VllmBench.Tokenizer
				if cfg.VllmBench.ModelDir != "" && strings.HasPrefix(tokenizerPath, cfg.VllmBench.ModelDir) {
					tokenizerPath = containerModelDir + strings.TrimPrefix(tokenizerPath, cfg.VllmBench.ModelDir)
				}
				tokenizerArg = tokenizerPath
			}

			// 6. 探测阶段：逐步提升 request-rate 找吞吐量饱和点
			rates := []int{8, 16, 32, 64, 128, 256}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			var probeEntries []string
			bestRate := 0
			bestTps := 0.0
			prevTps := 0.0
			var bestDetail string
			bestLogFile := "vllm_bench_serve.log"

			for i, rate := range rates {
				fmt.Printf("  [vllm_bench] 探测 request-rate=%d ...\n", rate)

				// 构造 bench 参数
				benchArgs := []string{
					"--host", host, "--port", port,
					"--model", cfg.Model.Name,
					"--disable-tqdm",
					"--request-rate", strconv.Itoa(rate),
				}
				if tokenizerArg != "" {
					benchArgs = append(benchArgs, "--tokenizer", tokenizerArg)
				}
				benchArgs = append(benchArgs, filteredArgs...)

				args := make([]string, 0, len(dockerArgs)+len(benchArgs))
				args = append(args, dockerArgs...)
				args = append(args, benchArgs...)

				cmd := exec.CommandContext(ctx, "docker", args...)
				out, err := cmd.CombinedOutput()

				logFile := fmt.Sprintf("vllm_bench_serve_rate%d.log", rate)
				os.WriteFile(logFile, out, 0644)

				if err != nil {
					if ctx.Err() != nil {
						fmt.Printf("  [vllm_bench] 整体超时（30min），停止探测\n")
						break
					}
					fmt.Printf("  [vllm_bench] request-rate=%d 失败: %v\n", rate, err)
					continue
				}

				detail, parseErr := parseBenchServeOutput(string(out), image)
				if parseErr != nil {
					fmt.Printf("  [vllm_bench] request-rate=%d 解析失败: %v\n", rate, parseErr)
					continue
				}

				tps := extractTokPerSec(detail)
				probeEntries = append(probeEntries, fmt.Sprintf("rate=%d:%.2ftok/s", rate, tps))
				fmt.Printf("  [vllm_bench] request-rate=%d → %.2f tok/s\n", rate, tps)

				if tps > bestTps {
					bestTps = tps
					bestRate = rate
					bestDetail = detail
					bestLogFile = logFile
				}

				// 判断是否停止（从第二轮开始比较）
				if i > 0 && prevTps > 0 {
					improvement := (tps - prevTps) / prevTps
					if improvement < 0.1 {
						fmt.Printf("  [vllm_bench] 吞吐量提升不足 10%%，停止探测\n")
						break
					}
				}
				prevTps = tps
			}

			if bestDetail == "" {
				return "", fmt.Errorf("所有探测轮次均失败，原始日志见 vllm_bench_serve_rate*.log")
			}

			probesStr := fmt.Sprintf("probes{%s} best_rate=%d",
				strings.Join(probeEntries, " "), bestRate)
			return probesStr + " | " + bestDetail + " | raw_log=" + bestLogFile, nil
		},
	}
}

// extractTokPerSec 从 bench detail 中提取 tok/s 值
func extractTokPerSec(detail string) float64 {
	re := regexp.MustCompile(`tok/s=([\d.]+)`)
	if m := re.FindStringSubmatch(detail); len(m) >= 2 {
		val, _ := strconv.ParseFloat(m[1], 64)
		return val
	}
	return 0
}

// parseBenchServeOutput 从 vllm bench serve 的标准输出中提取关键指标。
func parseBenchServeOutput(output, image string) (string, error) {
	ex := func(key string) string {
		re := regexp.MustCompile(`(?m)` + regexp.QuoteMeta(key) + `\s+([\d.]+)`)
		if m := re.FindStringSubmatch(output); len(m) >= 2 {
			return m[1]
		}
		return "n/a"
	}

	ok := ex("Successful requests:")
	dur := ex("Benchmark duration (s):")
	reqPS := ex("Request throughput (req/s):")
	tokPS := ex("Output token throughput (tok/s):")
	ttftMean := ex("Mean TTFT (ms):")
	ttftP50 := ex("Median TTFT (ms):")
	ttftP99 := ex("P99 TTFT (ms):")
	tpotMean := ex("Mean TPOT (ms):")
	tpotP50 := ex("Median TPOT (ms):")
	tpotP99 := ex("P99 TPOT (ms):")
	itlMean := ex("Mean ITL (ms):")
	itlP50 := ex("Median ITL (ms):")
	itlP99 := ex("P99 ITL (ms):")

	if ok == "n/a" && dur == "n/a" {
		return "", fmt.Errorf("无法解析 vllm bench serve 输出:\n%s", tailLines(output, 40))
	}

	// 只保留镜像名（去掉仓库前缀）
	shortImage := image
	if idx := strings.LastIndex(image, "/"); idx >= 0 {
		shortImage = image[idx+1:]
	}

	return fmt.Sprintf(
		"image=%s ok=%s dur=%ss req/s=%s tok/s=%s | TTFT(ms) mean=%s p50=%s p99=%s | TPOT(ms) mean=%s p50=%s p99=%s | ITL(ms) mean=%s p50=%s p99=%s",
		shortImage, ok, dur, reqPS, tokPS,
		ttftMean, ttftP50, ttftP99,
		tpotMean, tpotP50, tpotP99,
		itlMean, itlP50, itlP99,
	), nil
}

// dockerErrReason 从 docker 命令的输出（含 stderr）中提取人类可读的错误原因。
func dockerErrReason(output string) string {
	low := strings.ToLower(output)
	switch {
	case strings.Contains(low, "cannot connect") || strings.Contains(low, "is the docker daemon running"):
		return "Docker daemon 未启动，请执行 systemctl start docker 或 sudo dockerd"
	case strings.Contains(low, "permission denied"):
		return "当前用户无 Docker 权限，请将用户加入 docker 组：sudo usermod -aG docker $USER"
	case strings.Contains(low, "not found") || strings.Contains(low, "no such file"):
		return "docker 命令不存在，请安装 Docker"
	case output != "":
		return output
	default:
		return "未知错误，请手动运行 docker images 确认"
	}
}

// tailLines 返回字符串末尾最多 n 行，用于错误信息截断。
func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
		return "...\n" + strings.Join(lines, "\n")
	}
	return s
}
