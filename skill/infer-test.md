运行推理引擎验收测试并分析报告。

## 使用场景

对部署好的 LLM 推理服务做一轮完整验收测试，验证 API 兼容性、功能正确性和性能基线。

## 操作步骤

1. 进入 dist 目录，确认二进制和配置文件存在：
   ```
   ls $PROJECT_ROOT/dist/
   ```
   其中 `$PROJECT_ROOT` 是 `infer-tester` 工程根目录（本文件所在目录的上级）。

2. 根据实际情况修改 `dist/config.json`：
   - `server.base_url`：推理服务地址（如 `http://192.168.0.190:8100`）
   - `model.name`：模型名称（可留空，工具会自动查询 `/v1/models` 解析）

3. 运行测试（macOS arm64 示例）：
   ```bash
   cd $PROJECT_ROOT/dist
   ./infer-tester-darwin-arm64 -config config.json
   ```
   根据平台选择对应二进制：
   - Linux x86_64：`infer-tester-linux-amd64`
   - Linux ARM：`infer-tester-linux-arm64`
   - macOS Intel：`infer-tester-darwin-amd64`
   - macOS Apple Silicon：`infer-tester-darwin-arm64`

4. 读取测试报告：
   ```
   $PROJECT_ROOT/dist/report.md
   ```

5. 分析报告，关注以下内容：
   - ❌ 失败用例：逐条说明失败原因和可能的修复方向
   - ⏭ 跳过用例：确认跳过原因是否符合预期（能力不支持 vs 服务异常）
   - performance 套件：解读 TTFT、tokens/s、batch_scaling 数值，给出性能评价
   - 整体结论：服务是否可以上线，有哪些注意事项

## 常用选项

只跑指定套件：
```bash
./infer-tester-darwin-arm64 -suite smoke,api
```

只跑指定用例：
```bash
./infer-tester-darwin-arm64 -case latency_ttft,throughput_tokens
```

列出所有用例不执行：
```bash
./infer-tester-darwin-arm64 -list
```

自定义报告路径：
```bash
./infer-tester-darwin-arm64 -report /tmp/my-report.md
```
