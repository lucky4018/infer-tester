# 推理引擎测试报告

| | |
|---|---|
| **时间** | `2026-07-01T06:23:23Z` |
| **服务地址** | `http://192.168.0.190:8100` |
| **模型** | `ds` |
| **总耗时** | `9m48.805s` |

> [!CAUTION]
> **失败用例：**
> - `[api] openai_error_format` — expected 4xx, got 200
> - `[api] anthropic_tool_use` — status=400 err=<nil>
> - `[api] common_error_response_format` — expected 4xx from OpenAI, got 200
> - `[sampling] determinism_same_seed` — outputs differ:
  call1=" Paris.\",\n    \"The capital of France is Paris"
  call2=" Paris.\nThe capital of France is Paris.\nThe"
> - `[features] tool_calling_openai` — status=400 err=<nil>
> - `[features] reasoning_thinking` — Post "http://192.168.0.190:8100/v1/chat/completions": context deadline exceeded (Client.Timeout exceeded while awaiting headers)

## ✅ smoke

通过：**1** &nbsp; 失败：**0** &nbsp; 跳过：**0**

| 状态 | 用例 | 耗时 | 详情 |
|:---:|---|---:|---|
| ✅ | `basic_inference` | 3769ms | output: "` to `World`.\n\n``` console\n$ echo" |

## ❌ api

通过：**13** &nbsp; 失败：**3** &nbsp; 跳过：**2**

| 状态 | 用例 | 耗时 | 详情 |
|:---:|---|---:|---|
| ✅ | `openai_completions_fields` | 2932ms | all required fields present |
| ✅ | `openai_chat_roles` | 1830ms | role=assistant content=" in a French translation?" |
| ✅ | `openai_models_list` | 7ms | 1 model(s) listed |
| ✅ | `openai_streaming_chunks` | 3701ms | 11 chunks received |
| ✅ | `openai_streaming_done_signal` | 4084ms | stream ends with [DONE] |
| ❌ | `openai_error_format` | 6039ms | `expected 4xx, got 200` |
| ⏭ | `openai_embeddings` | 7ms | model does not support embeddings |
| ⏭ | `openai_rerank` | 81ms | model does not support rerank |
| ✅ | `anthropic_messages_basic` | 5572ms | input_tokens=6 output_tokens=15 |
| ✅ | `anthropic_count_tokens` | 11ms | input_tokens=10 |
| ✅ | `anthropic_streaming_events` | 5570ms | event types: map[content_block_delta:15 content_block_start:1 content_block_stop:1 message_delta:1 message_start:1 message_stop:1] |
| ❌ | `anthropic_tool_use` | 9ms | `status=400 err=<nil>` |
| ✅ | `common_health` | 7ms | OK |
| ✅ | `common_ping` | 7ms | OK |
| ✅ | `common_tokenize_roundtrip` | 22ms | 4 tokens, round-trip OK |
| ✅ | `common_metrics_prometheus` | 36ms | key metrics present (55128 bytes) |
| ❌ | `common_error_response_format` | 6043ms | `expected 4xx from OpenAI, got 200` |
| ✅ | `cross_compat_output_consistency` | 3650ms | openai="## 1." anthropic=":.\n\nEnce [" |

## ❌ sampling

通过：**3** &nbsp; 失败：**1** &nbsp; 跳过：**0**

| 状态 | 用例 | 耗时 | 详情 |
|:---:|---|---:|---|
| ❌ | `determinism_same_seed` | 7463ms | `outputs differ:
  call1=" Paris.\",\n    \"The capital of France is Paris"
  call2=" Paris.\nThe capital of France is Paris.\nThe"` |
| ✅ | `temperature_zero_greedy` | 1848ms | output="2\n1 +" |
| ✅ | `max_tokens_respected` | 1836ms | completion_tokens=5 finish_reason=length |
| ✅ | `stop_sequence_honored` | 31716ms | finish_reason=length text="RNALD and ANGERMALLY causes in mild _e.g._ cases of\n**newborn** n. an infant from birth to a week age. **Associated animals.** whooping cough.\n**new combination** n. an organism that results from a cross between two **Nicotiana** _n,_ genus of plants including tobacco. **Nicotiana tabacum**\ngenetically identical" |

## ❌ features

通过：**2** &nbsp; 失败：**2** &nbsp; 跳过：**1**

| 状态 | 用例 | 耗时 | 详情 |
|:---:|---|---:|---|
| ✅ | `structured_output_json_schema` | 7040ms | valid JSON: { "name": "John", "score": 4 } |
| ❌ | `tool_calling_openai` | 9ms | `status=400 err=<nil>` |
| ✅ | `prefix_caching_kv_reuse` | 8481ms | prefix caching active (cached_tokens field not exposed) |
| ❌ | `reasoning_thinking` | 120001ms | `Post "http://192.168.0.190:8100/v1/chat/completions": context deadline exceeded (Client.Timeout exceeded while awaiting headers)` |
| ⏭ | `multimodal_image` | 23ms | model does not support vision |

## ✅ performance

通过：**5** &nbsp; 失败：**0** &nbsp; 跳过：**0**

| 状态 | 用例 | 耗时 | 详情 |
|:---:|---|---:|---|
| ✅ | `latency_ttft` | 106163ms | n=5 avg=576ms min=430ms max=1.129s |
| ✅ | `throughput_tokens` | 44893ms | duration=15s concurrency=4 completed=4 failed=0 total_tokens=512 throughput=34.1 tok/s |
| ✅ | `latency_single_request` | 100792ms | n=5 avg=20.158s min=19.924s max=20.886s |
| ✅ | `prefix_caching_speedup` | 10057ms | cold=2.044s warm=2.036s speedup=0.4% |
| ✅ | `batch_scaling` | 37213ms | batch=1 dur=13.472s tok/s=2.4  batch=2 dur=11.129s tok/s=5.8  batch=4 dur=12.613s tok/s=10.1 |

## ✅ concurrency

通过：**4** &nbsp; 失败：**0** &nbsp; 跳过：**0**

| 状态 | 用例 | 耗时 | 详情 |
|:---:|---|---:|---|
| ✅ | `concurrent_requests` | 7083ms | n=16 all succeeded in 7.083s |
| ✅ | `continuous_batching_mixed` | 55516ms | 7 mixed jobs all completed in 55.516s |
| ✅ | `preemption_short_wins` | 2273ms | 4/4 short requests done in 2.221s |
| ✅ | `cancellation_mid_stream` | 3001ms | cancelled cleanly after 6 events |

