# 推理引擎测试报告

| | |
|---|---|
| **时间** | `2026-07-01T07:16:09Z` |
| **服务地址** | `http://192.168.0.190:8100` |
| **模型** | `qwen` |
| **总耗时** | `3m39.383s` |

## ✅ smoke

通过：**1** &nbsp; 失败：**0** &nbsp; 跳过：**0**

| 状态 | 用例 | 耗时 | 详情 |
|:---:|---|---:|---|
| ✅ | `basic_inference` | 1641ms | output: "! Can you please give me a short summary of" |

## ✅ api

通过：**13** &nbsp; 失败：**0** &nbsp; 跳过：**5**

| 状态 | 用例 | 耗时 | 详情 |
|:---:|---|---:|---|
| ✅ | `openai_completions_fields` | 1309ms | all required fields present |
| ✅ | `openai_chat_roles` | 344ms | role=assistant content="Yes" |
| ✅ | `openai_models_list` | 9ms | 1 model(s) listed |
| ✅ | `openai_streaming_chunks` | 2109ms | 13 chunks received |
| ✅ | `openai_streaming_done_signal` | 1757ms | stream ends with [DONE] |
| ⏭ | `openai_error_format` | 2596ms | server has a default model configured, missing model param returns 200 |
| ⏭ | `openai_embeddings` | 8ms | model does not support embeddings |
| ⏭ | `openai_rerank` | 8ms | model does not support rerank |
| ✅ | `anthropic_messages_basic` | 1954ms | input_tokens=10 output_tokens=12 |
| ✅ | `anthropic_count_tokens` | 12ms | input_tokens=14 |
| ✅ | `anthropic_streaming_events` | 2080ms | event types: map[content_block_delta:10 content_block_start:1 content_block_stop:1 message_delta:1 message_start:1 message_stop:1] |
| ⏭ | `anthropic_tool_use` | 11ms | model does not support tool calling |
| ✅ | `common_health` | 7ms | OK |
| ✅ | `common_ping` | 6ms | OK |
| ✅ | `common_tokenize_roundtrip` | 20ms | 4 tokens, round-trip OK |
| ✅ | `common_metrics_prometheus` | 36ms | key metrics present (56716 bytes) |
| ⏭ | `common_error_response_format` | 2666ms | server has a default model configured, missing model param returns 200 |
| ✅ | `cross_compat_output_consistency` | 844ms | openai="2" anthropic="2" |

## ✅ sampling

通过：**4** &nbsp; 失败：**0** &nbsp; 跳过：**0**

| 状态 | 用例 | 耗时 | 详情 |
|:---:|---|---:|---|
| ✅ | `determinism_same_seed` | 3429ms | identical output " Paris, which is also the most populous city in" |
| ✅ | `temperature_zero_greedy` | 836ms | output="2, 2" |
| ✅ | `max_tokens_respected` | 848ms | completion_tokens=5 finish_reason=length |
| ✅ | `stop_sequence_honored` | 3639ms | finish_reason=stop text=".\n\nOne, two, three, four, five, six, seven, eight, nine, ten. " |

## ✅ features

通过：**3** &nbsp; 失败：**0** &nbsp; 跳过：**2**

| 状态 | 用例 | 耗时 | 详情 |
|:---:|---|---:|---|
| ✅ | `structured_output_json_schema` | 3144ms | valid JSON: {"name": "Lionel Messi", "score": 98} |
| ⏭ | `tool_calling_openai` | 9ms | model does not support tool calling |
| ✅ | `prefix_caching_kv_reuse` | 3405ms | prefix caching active (cached_tokens field not exposed) |
| ⏭ | `reasoning_thinking` | 32088ms | model does not support thinking mode |
| ✅ | `multimodal_image` | 520ms | vision response: "Green" |

## ✅ performance

通过：**5** &nbsp; 失败：**0** &nbsp; 跳过：**0**

| 状态 | 用例 | 耗时 | 详情 |
|:---:|---|---:|---|
| ✅ | `latency_ttft` | 41200ms | n=5 avg=208ms min=175ms max=325ms |
| ✅ | `throughput_tokens` | 21380ms | duration=15s concurrency=4 completed=4 failed=0 total_tokens=512 throughput=34.1 tok/s |
| ✅ | `latency_single_request` | 40435ms | n=5 avg=8.087s min=8.026s max=8.183s |
| ✅ | `prefix_caching_speedup` | 2496ms | cold=828ms warm=838ms speedup=-1.3% |
| ✅ | `batch_scaling` | 16324ms | batch=1 dur=5.246s tok/s=6.1  batch=2 dur=5.575s tok/s=11.5  batch=4 dur=5.504s tok/s=23.3 |

## ✅ concurrency

通过：**4** &nbsp; 失败：**0** &nbsp; 跳过：**0**

| 状态 | 用例 | 耗时 | 详情 |
|:---:|---|---:|---|
| ✅ | `concurrent_requests` | 3610ms | n=16 all succeeded in 3.61s |
| ✅ | `continuous_batching_mixed` | 24421ms | 7 mixed jobs all completed in 24.421s |
| ✅ | `preemption_short_wins` | 1141ms | 4/4 short requests done in 1.09s |
| ✅ | `cancellation_mid_stream` | 3000ms | cancelled cleanly after 16 events |

