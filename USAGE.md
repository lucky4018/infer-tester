# infer-tester 使用说明

针对 OpenAI 兼容接口推理引擎（vLLM、SGLang 等）的功能、接口兼容性与性能测试工具。

## 目录

- [前提条件](#前提条件)
- [快速开始](#快速开始)
- [配置文件](#配置文件)
- [命令行参数](#命令行参数)
- [测试套件](#测试套件)
- [报告输出](#报告输出)
- [常见场景](#常见场景)

---

## 前提条件

- 推理引擎实例已启动并可访问（`/health` 返回 200）
- 不需要任何 Python 环境或外部依赖

---

## 快速开始

```bash
# 1. 编辑配置文件（至少填写 server.base_url 和 model.name）
vi config.json

# 2. 查看所有测试套件和用例
./infer-tester -list

# 3. 运行全部测试
./infer-tester

# 4. 查看报告
cat report.md
```

程序默认从**当前目录**读取 `config.json`，无需额外参数。

---

## 配置文件

所有配置通过 JSON 文件传入。若不指定 `-config` 参数，程序自动读取当前目录的 `config.json`。

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
    "smoke": {
      "basic_inference": true
    },
    "api": {
      "openai_completions_fields": true,
      "openai_chat_roles": true,
      "openai_models_list": true,
      "openai_streaming_chunks": true,
      "openai_streaming_done_signal": true,
      "openai_error_format": true,
      "openai_embeddings": true,
      "openai_rerank": true,
      "anthropic_messages_basic": true,
      "anthropic_count_tokens": true,
      "anthropic_streaming_events": true,
      "anthropic_tool_use": true,
      "common_health": true,
      "common_ping": true,
      "common_tokenize_roundtrip": true,
      "common_metrics_prometheus": true,
      "common_error_response_format": true,
      "cross_compat_output_consistency": true
    },
    "sampling": {
      "determinism_same_seed": true,
      "temperature_zero_greedy": true,
      "max_tokens_respected": true,
      "stop_sequence_honored": true
    },
    "features": {
      "structured_output_json_schema": true,
      "tool_calling_openai": true,
      "prefix_caching_kv_reuse": true,
      "reasoning_thinking": true,
      "multimodal_image": true
    },
    "performance": {
      "latency_ttft": true,
      "throughput_tokens": true,
      "latency_single_request": true,
      "prefix_caching_speedup": true,
      "batch_scaling": true
    },
    "concurrency": {
      "concurrent_requests": true,
      "continuous_batching_mixed": true,
      "preemption_short_wins": true,
      "cancellation_mid_stream": true
    }
  },
  "output": {
    "report_file": "report.md",
    "verbose": true
  }
}
```

### 字段说明

#### `server`

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|:---:|---|---|
| `base_url` | string | ✅ | — | 推理引擎地址，不含路径，如 `http://127.0.0.1:8000` |
| `api_key` | string | | `""` | API Key，未设置鉴权时填 `EMPTY` 即可 |
| `timeout_seconds` | int | | `120` | 单次 HTTP 请求超时时间（秒） |

#### `model`

| 字段 | 类型 | 必填 | 说明 |
|---|---|:---:|---|
| `name` | string | ✅ | 模型 ID，需与推理引擎启动时 `--model` 参数一致 |

#### `performance`

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `latency_requests` | int | `10` | 延迟测试发送的串行请求次数 |
| `throughput_duration_seconds` | int | `30` | 吞吐测试持续时间（秒） |

#### `concurrency`

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `num_requests` | int | `20` | 并发请求测试同时发送的请求数量 |

#### `cases`

全部 36 个用例的开关，按 suite 分组，`true` 启用，`false` 禁用。未出现在配置中的用例默认启用。

全部用例默认启用。模型不支持某项能力时，将对应用例改为 `false` 即可：

| Suite | 用例 | 所需能力 |
|---|---|---|
| `api` | `openai_embeddings` | Embedding 模型 |
| `api` | `openai_rerank` | Rerank 模型 |
| `features` | `reasoning_thinking` | 支持 thinking 模式的推理模型 |
| `features` | `multimodal_image` | 多模态视觉模型 |

#### `output`

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `report_file` | string | `report.md` | 报告路径；同目录自动生成同名 `.json` 报告，支持 `.md` 或 `.json` 后缀 |
| `verbose` | bool | `true` | 为 `true` 时在终端打印每条通过用例的详情 |

---

## 命令行参数

```
./infer-tester [选项]

选项：
  -config  <path>    配置文件路径（默认：当前目录的 config.json）
  -report  <path>    报告输出路径，覆盖配置文件中的 output.report_file
  -suite   <names>   只运行指定套件，逗号分隔，子串匹配不区分大小写
  -case    <names>   只运行指定用例，逗号分隔，子串匹配不区分大小写
  -list              列出所有套件和用例后退出，不执行测试
  -v                 详细输出，通过的用例也打印 detail 信息
```

`-suite` 按套件名过滤，`-case` 按用例名子串过滤，临时使用；持久化开关在 `config.json` 的 `cases` 中配置：

```bash
# 只跑冒烟测试
./infer-tester -suite smoke

# 同时跑多个套件
./infer-tester -suite api,sampling

# api 套件中只跑 OpenAI 协议用例（利用用例名前缀）
./infer-tester -case openai

# api 套件中只跑 Anthropic 协议用例
./infer-tester -case anthropic

# 查看所有套件和用例（不执行）
./infer-tester -list
```

---

## 测试套件

共 6 个套件、36 个用例。运行 `-list` 可查看完整列表及每条用例的说明。

### smoke — 冒烟测试

最小化验证推理引擎是否正常启动并能响应请求。

| 用例 | 说明 |
|---|---|
| `basic_inference` | 发送 Hello，验证模型返回非空文本，确认推理引擎基本可用 |

### api — 接口兼容性测试

覆盖 OpenAI、Anthropic 两种协议及公共端点，验证 API 格式、字段、流式、错误处理均符合规范。用例名前缀即协议分类，可配合 `-case` 过滤：`-case openai`、`-case anthropic`、`-case common`。

| 用例 | 说明 |
|---|---|
| `openai_completions_fields` | 验证 `/v1/completions` 响应包含 id/object/model/choices/usage 及所有子字段 |
| `openai_chat_roles` | 发送 system+user 多轮消息，验证响应 role=assistant 且内容非空 |
| `openai_models_list` | GET `/v1/models`，验证返回 list 格式，每个模型含 id/object/owned_by 字段 |
| `openai_streaming_chunks` | Chat SSE 流式请求，验证收到多个合法 JSON chunk |
| `openai_streaming_done_signal` | Completions SSE 流式请求，验证最后一个事件为 `[DONE]` |
| `openai_error_format` | 缺少 model 参数时，验证返回 4xx 且 body 含 error 字段 |
| `openai_embeddings` | POST `/v1/embeddings`，验证返回两个向量且维度一致（非 Embedding 模型自动跳过） |
| `openai_rerank` | POST `/v1/rerank`，验证文档按相关性重排序后返回结果（非 Rerank 模型自动跳过） |
| `anthropic_messages_basic` | POST `/v1/messages`，验证响应含 id/type/role/content/usage 等必要字段 |
| `anthropic_count_tokens` | POST `/v1/messages/count_tokens`，验证返回 input_tokens > 0 |
| `anthropic_streaming_events` | Messages SSE 流式，验证事件序列包含 message_start 事件类型 |
| `anthropic_tool_use` | 携带工具定义发送请求，验证模型返回 tool_use 内容块或正常回答 |
| `common_health` | GET `/health`，验证返回 200 |
| `common_ping` | GET `/ping`，验证返回 200（端点不存在时自动跳过） |
| `common_tokenize_roundtrip` | 调用 `/tokenize` 再调用 `/detokenize`，验证往返一致（端点不存在时自动跳过） |
| `common_metrics_prometheus` | GET `/metrics`，验证 Prometheus 格式中包含核心推理指标（端点不存在时自动跳过） |
| `common_error_response_format` | 分别触发 OpenAI 和 Anthropic 错误，验证两种协议的错误格式符合规范 |
| `cross_compat_output_consistency` | 同一 prompt 分别走 OpenAI chat 和 Anthropic messages 接口，对比两者输出 |

### sampling — 采样参数测试

验证 temperature、seed、max_tokens、stop 等生成控制参数行为正确。

| 用例 | 说明 |
|---|---|
| `determinism_same_seed` | temperature=0 seed=42 连续调用两次，验证输出完全相同 |
| `temperature_zero_greedy` | temperature=0 贪心解码，验证返回非空确定性输出 |
| `max_tokens_respected` | 设置 max_tokens=5，验证实际 completion_tokens 不超限 |
| `stop_sequence_honored` | 设置 stop=["END"]，验证输出中不含该词且 finish_reason 正确 |

### features — 功能特性测试

验证结构化输出、工具调用、KV 缓存复用、推理思维链、多模态等高级能力。

| 用例 | 说明 |
|---|---|
| `structured_output_json_schema` | 用 response_format 指定 JSON Schema，验证输出为合法 JSON 且必填字段齐全 |
| `tool_calling_openai` | 发送带 tools 定义的请求，验证模型返回 tool_calls 调用块 |
| `prefix_caching_kv_reuse` | 相同长前缀发送两次，检查第二次 cached_tokens 命中，验证 KV 缓存复用 |
| `reasoning_thinking` | 开启 thinking 模式，验证响应中存在 thinking 内容块（不支持时自动跳过） |
| `multimodal_image` | 发送 base64 图片，验证模型返回图片描述（非视觉模型自动跳过） |

### performance — 性能测试

量化延迟、吞吐、KV 缓存加速比等指标，结果仅供参考，不设通过阈值。

| 用例 | 说明 |
|---|---|
| `latency_ttft` | 流式请求，统计首 token 到达时间（TTFT）的平均/最小/最大值 |
| `throughput_tokens` | 4 并发持续 N 秒，统计输出 token 总数和平均 tokens/s 吞吐量 |
| `latency_single_request` | 串行发送 N 次请求，统计端到端平均/最小/最大延迟 |
| `prefix_caching_speedup` | 对比冷热前缀请求耗时，量化 KV 缓存带来的延迟加速比 |
| `batch_scaling` | batch=1/2/4 并发，观察 tokens/s 随批大小的变化趋势 |

### concurrency — 并发与调度测试

验证引擎在多请求并发时的正确性，覆盖连续批处理、请求抢占和流式取消。

| 用例 | 说明 |
|---|---|
| `concurrent_requests` | N 个请求同时发出，验证全部成功无报错（N 由 `concurrency.num_requests` 配置） |
| `continuous_batching_mixed` | 混合短请求(5 token)和长请求(150 token)并发，验证短请求不被长请求饿死 |
| `preemption_short_wins` | 先发长请求，50ms 后发 4 个短请求，验证短请求能正常完成不被阻塞 |
| `cancellation_mid_stream` | 流式请求中途取消，验证连接正常关闭，服务端不 hang 不报错 |

---

## 报告输出

运行结束后生成两个文件：

| 文件 | 格式 | 用途 |
|---|---|---|
| `report.md` | Markdown | 人类可读，可直接在 GitHub / GitLab / Obsidian 等渲染 |
| `report.json` | JSON | 机器可读，包含每条用例的状态、耗时、详情和错误信息 |

文件名由 `output.report_file` 配置，支持 `.md` 或 `.json` 后缀，另一个文件自动同目录生成。

### 终端退出码

| 退出码 | 含义 |
|---|---|
| `0` | 全部启用的用例通过（禁用的用例不计入失败） |
| `1` | 存在至少一条 FAIL |

---

## 常见场景

**快速验证服务是否正常**

```bash
./infer-tester -suite smoke
```

**只验证接口兼容性，不跑性能测试**

```bash
./infer-tester -suite smoke,api,sampling,features
```

**关掉某个用例（永久生效）**

在 `config.json` 中将对应用例改为 `false`，例如不跑性能测试中的吞吐测试：

```json
"throughput_sustained": false
```

**启用多模态测试**

将 `multimodal_image` 改为 `true`，同时更新模型名称：

```json
"model": { "name": "Qwen/Qwen2.5-VL-7B-Instruct" },
"cases": { ..., "multimodal_image": true, ... }
```

**启用推理模型 thinking 测试**

```json
"model": { "name": "deepseek-ai/DeepSeek-R1" },
"cases": { ..., "reasoning_thinking": true, ... }
```

**指定报告路径**

```bash
./infer-tester -report /tmp/report-$(date +%Y%m%d).md
```

**在 CI 中使用**

```bash
./infer-tester -report ci-report.md
# 退出码非 0 时 CI 自动标记失败
```

