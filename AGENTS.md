# infer-tester 项目规格文档

用于重新生成本项目的完整提示词，描述项目的目标、架构、功能和所有设计决策。

---

## 项目目标

用 Go 编写一个命令行工具，对 OpenAI 兼容推理引擎（vLLM、SGLang 等）做**部署验收测试**，验证服务功能正常、API 兼容、性能基线达标。工具需支持：

- 离线运行，无任何 Python 依赖
- 读取 JSON 配置文件
- 运行 6 个套件共 36 个测试用例
- 生成中文 Markdown + JSON 双格式报告
- 交叉编译到 linux/darwin/windows × amd64/arm64 共 6 个平台

---

## 技术选型

- 语言：Go，标准项目布局（`cmd/` + `internal/`）
- 模块名：`infer-tester`
- 无第三方依赖，只用标准库
- 构建脚本：`build.sh`，产物输出到 `dist/`

---

## 目录结构

```
cmd/infer-tester/main.go
internal/
  config/config.go
  client/client.go
  runner/runner.go
  report/report.go
  suite/smoke.go
  suite/api.go
  suite/sampling.go
  suite/features.go
  suite/performance.go
  suite/concurrency.go
build.sh
config.json
USAGE.md
AGENTS.md
```

---

## 配置文件（config.json）

```json
{
  "server": {
    "base_url": "http://127.0.0.1:8000",
    "api_key": "EMPTY",
    "timeout_seconds": 120
  },
  "model": {
    "name": "Qwen/Qwen2.5-7B-Instruct"
  },
  "performance": {
    "latency_requests": 5,
    "throughput_duration_seconds": 15
  },
  "concurrency": {
    "num_requests": 16
  },
  "cases": {
    "smoke":       { "basic_inference": true },
    "api":         { "openai_completions_fields": true, ... },
    "sampling":    { "determinism_same_seed": true, ... },
    "features":    { "structured_output_json_schema": true, ... },
    "performance": { "latency_ttft": true, ... },
    "concurrency": { "concurrent_requests": true, ... }
  },
  "output": {
    "report_file": "report.md",
    "verbose": true
  }
}
```

- `server.base_url` 和 `model.name` 必填
- 默认值：`timeout_seconds=120`、`latency_requests=10`、`throughput_duration_seconds=30`、`num_requests=20`、`report_file="report.md"`、`verbose=true`
- `cases` 按 suite 分组，每个用例名对应 bool 开关；未出现在配置中的用例默认启用
- **不允许**在 Config 里加 `supports_vision` 等能力开关字段，能力判断统一由 HTTP 响应码在运行时决定

---

## CLI 参数

```
-config <path>    配置文件路径（默认：当前目录 config.json）
-report <path>    报告路径，覆盖配置中的 output.report_file
-suite  <names>   只运行指定套件，逗号分隔，子串匹配不区分大小写
-case   <names>   只运行指定用例，逗号分隔，子串匹配不区分大小写
-list             列出所有套件和用例后退出，不执行测试
-v                详细模式，通过的用例也打印 detail
```

退出码：0=全部通过，1=至少一条 FAIL。

---

## 核心包设计

### internal/config

`Config` 结构体 + `Load(path string) (*Config, error)`。Load 先设置默认值再解析 JSON。

### internal/client

`Client` 封装 HTTP 请求，方法：

- `Do(ctx, method, path, body) (*RawResp, error)`
- `Get(ctx, path) (*RawResp, error)`
- `Post(ctx, path, body) (*RawResp, error)`
- `JsonPost(ctx, path, reqBody, out) (statusCode int, error)` — 最常用，自动 marshal/unmarshal
- `Stream(ctx, path, body) ([]SSEEvent, duration, error)` — 收集完整 SSE 事件流
- `StreamTTFT(ctx, path, body) (ttft, total duration, chunks int, error)` — 只记首 token 时间，记录后继续排干流

`RawResp` 含 `Status int` 和 `Body []byte`。

### internal/runner

核心类型：

```go
type TestCase struct {
    Name        string
    Description string
    Run         func() (detail string, err error)
}

type Suite struct {
    Name        string
    Description string
    Cases       []TestCase
}
```

`runner.Skip(reason string) (string, error)` 返回 `(reason, ErrSkip)`，runner 将用例记为 SKIP 而非 FAIL。

`Runner.Run()` 执行逻辑：
1. 按 suite 名过滤（`-suite`）
2. 按用例名过滤（`-case`）
3. 检查 `config.cases` 开关，关闭则记 SKIP
4. 执行 `Run()`，err==nil→PASS，err==ErrSkip→SKIP，其他→FAIL

### internal/report

`Write(path string, r Report) error` 同时生成 `.md` 和 `.json`，路径从后缀自动推导。

Markdown 报告格式（中文）：
- 标题：`# 推理引擎测试报告`
- 元信息表：时间、服务地址、模型、总耗时
- 若有失败：`> [!CAUTION]` 警告块列出所有失败用例
- 无 Summary 汇总表
- 每个 suite 一个二级标题（`## ✅/❌ suiteName`），含"通过/失败/跳过"统计和用例明细表
- 表头：`状态 | 用例 | 耗时 | 详情`，状态用 ✅ ❌ ⏭ 图标

---

## 自动跳过规则

所有能力判断在运行时通过 HTTP 响应码决定，不依赖配置开关：

| 场景 | 触发条件 | 跳过消息 |
|---|---|---|
| vLLM 特有端点（`/ping`、`/tokenize`、`/metrics`） | status == 404 | `"endpoint not available"` |
| Embedding 模型不支持 | status >= 400 | `"model does not support embeddings"` |
| Rerank 模型不支持 | status >= 400 | `"model does not support rerank"` |
| Tool calling 不支持 | status >= 400 | `"model does not support tool calling"` |
| Vision 不支持 | status >= 400 | `"model does not support vision"` |
| Thinking 模式不支持 | status >= 400 **或** 响应中无 thinking 块 | `"model does not support thinking mode"` |
| Thinking 请求超时 | err 含 "context deadline exceeded"/"timeout" | `"model does not support thinking mode or response too slow"` |
| 服务有默认模型（缺 model 字段返回 200） | status < 400 | `"server has a default model configured..."` |

---

## 测试套件与用例（36 个）

### smoke（1 个）

| 用例 | 端点 | 验证 | 自动跳过 |
|---|---|---|---|
| `basic_inference` | POST /v1/completions | 返回非空文本 | 无 |

### api（18 个）

| 用例 | 端点 | 验证 | 自动跳过条件 |
|---|---|---|---|
| `openai_completions_fields` | POST /v1/completions | 响应含 id/object/model/choices/usage 及子字段 | 无 |
| `openai_chat_roles` | POST /v1/chat/completions | role=assistant，content 非空 | 无 |
| `openai_models_list` | GET /v1/models | 返回 list 格式，每个模型含 id/object/owned_by | 无 |
| `openai_streaming_chunks` | POST /v1/chat/completions (stream) | 收到多个合法 JSON chunk | 无 |
| `openai_streaming_done_signal` | POST /v1/completions (stream) | 最后事件为 [DONE] | 无 |
| `openai_error_format` | POST /v1/completions（省略 model） | 返回 4xx 且含 error 字段 | status<400 时跳过（服务有默认模型） |
| `openai_embeddings` | POST /v1/embeddings | 返回 2 个向量，维度一致 | status>=400 跳过 |
| `openai_rerank` | POST /v1/rerank | 返回重排结果 | status>=400 跳过 |
| `anthropic_messages_basic` | POST /v1/messages | 含 id/type/role/content/usage | 无 |
| `anthropic_count_tokens` | POST /v1/messages/count_tokens | input_tokens > 0 | 无 |
| `anthropic_streaming_events` | POST /v1/messages (stream) | 包含 message_start 事件类型 | 无 |
| `anthropic_tool_use` | POST /v1/messages（带 tools） | 返回 tool_use 块或正常回答 | status>=400 跳过 |
| `common_health` | GET /health | 返回 200 | 无 |
| `common_ping` | GET /ping | 返回 200 | status==404 跳过 |
| `common_tokenize_roundtrip` | POST /tokenize + /detokenize | 往返一致 | status==404 跳过 |
| `common_metrics_prometheus` | GET /metrics | 包含核心推理指标 | status==404 跳过 |
| `common_error_response_format` | POST /v1/completions 和 /v1/messages（各触发错误） | 两种协议的错误格式合规 | 任一返回 200 时跳过（服务有默认模型） |
| `cross_compat_output_consistency` | OpenAI chat + Anthropic messages | 同一 prompt 两者均能正常响应 | 无 |

### sampling（4 个）

| 用例 | 验证 | 失败条件 |
|---|---|---|
| `determinism_same_seed` | temperature=0 seed=42 两次输出完全相同 | 输出不同 |
| `temperature_zero_greedy` | temperature=0 返回非空输出 | 空输出或报错 |
| `max_tokens_respected` | max_tokens=5 时 completion_tokens≤5 | 超出限制 |
| `stop_sequence_honored` | stop=["END"] 输出不含 "END"，finish_reason 正确 | 含 stop 词或 finish_reason 错误 |

### features（5 个）

| 用例 | 验证 | 自动跳过条件 |
|---|---|---|
| `structured_output_json_schema` | 输出为合法 JSON，必填字段齐全 | 无 |
| `tool_calling_openai` | 返回 tool_calls 调用块 | status>=400 跳过 |
| `prefix_caching_kv_reuse` | 相同长前缀两次请求，第二次有 cached_tokens | 无（cached_tokens 字段不存在时输出提示但不失败） |
| `reasoning_thinking` | 开启 thinking 模式，响应含 thinking 内容块 | status>=400、无 thinking 块、或超时时跳过 |
| `multimodal_image` | 发送 base64 图片（1×1 白色 PNG），返回描述文本 | status>=400 跳过 |

### performance（5 个）

性能用例只收集数据，不设通过/失败阈值，Run 函数不对数值做断言。

| 用例 | 逻辑 | 参数来源 | 失败条件 |
|---|---|---|---|
| `latency_ttft` | 串行 N 次流式请求，统计 TTFT avg/min/max | `performance.latency_requests`（默认 10） | 任一请求失败 |
| `throughput_tokens` | 4 并发持续 N 秒，统计 completion_tokens 总量和 tokens/s | `performance.throughput_duration_seconds`（默认 30） | completed==0（全部失败） |
| `latency_single_request` | 串行 N 次非流式请求，统计端到端 avg/min/max | `performance.latency_requests` | 任一请求失败 |
| `prefix_caching_speedup` | 冷热前缀各一次，计算延迟加速比百分比 | 无 | warm-up 或冷热请求失败 |
| `batch_scaling` | batch=1/2/4 并发，各记录 tokens/s | 无 | 所有 batch 的 totalTokens==0 |

### concurrency（4 个）

| 用例 | 逻辑 | 失败条件 |
|---|---|---|
| `concurrent_requests` | N 个请求同时发出，验证全部成功 | 任一失败 |
| `continuous_batching_mixed` | 混合短(5 token)和长(150 token)请求并发，验证短请求不饿死 | 任一失败 |
| `preemption_short_wins` | 先发长请求，50ms 后发 4 个短请求，验证短请求正常完成 | 短请求未全部成功 |
| `cancellation_mid_stream` | 流式请求 3s 后取消，验证连接正常关闭 | 0 events（未收到任何数据即断开）或非 context 类错误 |

---

## 构建脚本（build.sh）

交叉编译 6 个目标，Windows 加 `.exe` 后缀，产物统一放 `dist/`，编译后自动复制 `config.json` 和 `USAGE.md` 到 `dist/`：

```
linux/amd64   linux/arm64
darwin/amd64  darwin/arm64
windows/amd64 windows/arm64
```

---

## 用户文档（USAGE.md）

随 dist 发布的用户手册，包含：
- 前提条件与快速开始
- 完整 config.json 示例（含全部 36 个用例开关）
- 字段说明表
- 命令行参数说明
- 测试套件与用例说明表（中文）
- 报告输出格式说明
- 常见使用场景示例

---

## 关键设计决策

1. **自动跳过而非失败**：模型能力不足（embedding、rerank、tool calling、vision、thinking）时用 `runner.Skip()` 跳过，不计入 FAIL，让工具适配任意模型
2. **无能力开关字段**：不在 Config 里加 `supports_vision` 等字段，运行时通过 HTTP 响应码判断
3. **用例开关按 suite 分组**：`config.cases` 是 `map[string]map[string]bool`，按 suite 名分组，可读性好
4. **报告用中文**：标题、表头、统计文字全用中文；无 Summary 汇总表，顶部只在有失败时显示警告块
5. **性能用例不设阈值**：只记录测量值，Run 函数只在服务完全不可用（completed==0）时返回 error
6. **吞吐量用 tokens/s**：`throughput_sustained`（req/s）已移除，只保留 `throughput_tokens`（tokens/s），因为 tokens/s 是业界标准指标
7. **模型名自动解析**：`model.name` 可留空；若配置名在 `/v1/models` 列表中找不到，自动回退到第一个可用模型，启动时打印提示

---

## Skill：infer-test

`skill/infer-test.md` 是供 AI 模型（如 Claude Code）调用的技能文件。

### 作用

当用户要求对某个推理服务做验收测试时，模型读取该 skill 文件后，可以自主完成以下操作：

1. 修改 `dist/config.json` 中的服务地址和模型名称
2. 选择对应平台的二进制文件运行测试
3. 读取生成的 `report.md`
4. 分析失败/跳过用例、解读性能数据，给出上线建议

### 调用方式

在 Claude Code 中，将 `skill/infer-test.md` 的内容作为上下文提示，或直接在对话中引用：

```
请执行 skill/infer-test.md 中的操作，目标服务为 http://192.168.0.190:8100，模型名称留空自动解析。
```

也可以将其注册为 Claude Code 项目级 slash command，放到 `.claude/commands/infer-test.md`，之后通过 `/infer-test` 触发。

### 文件位置

```
skill/
  infer-test.md   # 测试执行与报告分析的完整操作指南
```
