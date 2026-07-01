# infer-tester

针对 OpenAI 兼容接口推理引擎（vLLM、SGLang 等）的功能、接口兼容性与性能测试工具。使用 Go 标准库编写，无外部依赖，适合离线部署环境。

## 编译

**前提条件**：Go 1.21+

```bash
# 在工程根目录执行，同时编译 6 个平台，产物输出到 dist/
bash build.sh
```

`build.sh` 除二进制文件外，还会将 `config.json` 和 `USAGE.md` 一并复制到 `dist/`，打包 `dist/` 即可交付给用户。

手动交叉编译：

```bash
GOOS=linux GOARCH=amd64 go build -o dist/infer-tester-linux-amd64 ./cmd/infer-tester
GOOS=linux GOARCH=arm64 go build -o dist/infer-tester-linux-arm64 ./cmd/infer-tester
```

## 项目结构

```
cmd/infer-tester/   CLI 入口
internal/
  client/           HTTP 客户端（OpenAI / Anthropic / SSE）
  config/           配置加载
  runner/           测试执行器与类型定义
  report/           JSON + Markdown 报告生成
  suite/            各测试套件实现
```

用户文档见 `USAGE.md`，随二进制一起发布到 `dist/`。
