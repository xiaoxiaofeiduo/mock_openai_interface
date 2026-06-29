# my-gin-project

一个基于 Go + Gin 的本地 Mock 服务，用于模拟 OpenAI、Anthropic 和自定义协议的聊天响应。项目主要面向前端联调、流式响应调试和接口行为验证，不会调用真实大模型。

## 功能特性

- 支持 OpenAI 兼容接口：`POST /v1/chat/completions`
- 支持 Anthropic Messages 接口：`POST /v1/messages`
- 支持自定义聊天接口：`POST /v1/custom/chat`
- 同时支持流式 SSE 响应和普通 JSON 响应
- 可通过 `token_length` 控制每个流式分片长度
- 提供静态测试页面，方便手动构造请求和查看响应
- 支持回显请求头 `X-Real-IP` 到响应头 `X-Echoed-Real-IP`

## 项目结构

```text
.
├── main.go                         # Gin 服务入口和所有接口处理逻辑
├── main_test.go                    # 接口响应和 SSE 事件测试
├── static/
│   ├── main.html                   # 测试页面导航
│   ├── simulate_chat_ai.html       # OpenAI 兼容接口测试页
│   ├── simulate_anthropic_chat.html # Anthropic Messages 接口测试页
│   ├── simulate_customize_chat.html # 自定义协议接口测试页
│   ├── app.css                     # 前端页面公共样式
│   └── favicon.ico
├── go.mod
└── go.sum
```

## 快速开始

安装依赖：

```bash
go mod download
```

启动服务：

```bash
go run main.go
```

服务默认监听：

```text
http://127.0.0.1:8080/
```

打开根路径后，可以选择测试页面：

- OpenAI 聊天模拟测试：`/static/simulate_chat_ai.html`
- Anthropic Messages 模拟测试：`/static/simulate_anthropic_chat.html`
- 自定义流式响应测试：`/static/simulate_customize_chat.html`

## 构建与测试

```bash
go build -o my-gin-project .
go test ./...
go vet ./...
gofmt -w main.go main_test.go
```

当前自动化测试覆盖 Anthropic Messages 的非流式响应和 SSE 事件序列。

## GitHub Release

推送 `v*` 标签会自动触发 GitHub Actions 构建并发布 Release：

```bash
git tag v1.0.0
git push origin v1.0.0
```

也可以在 GitHub Actions 页面手动运行 `Release` 工作流并填写 tag。

Release 产物包含以下平台：

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`
- `windows/amd64`
- `windows/arm64`

每个压缩包都包含可执行文件、`static/` 静态资源、`README.md` 和 `LICENSE`。`checksums.txt` 提供所有产物的 SHA-256 校验值。

## 接口说明

### OpenAI 兼容接口

请求：

```bash
curl --noproxy '*' -X POST 'http://127.0.0.1:8080/v1/chat/completions?token_length=3' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "deepseek-r1:1.5b",
    "messages": [
      {"role": "system", "content": "这是一个模拟回复。"},
      {"role": "user", "content": "你好"}
    ],
    "stream": false
  }'
```

说明：

- `stream: false` 返回一次性 JSON。
- `stream: true` 返回 `text/event-stream`。
- 服务端会拼接所有 `role = "system"` 的内容作为模型回复。
- `token_length` 默认值为 `3`，按字符分片。
- 流式响应最后会发送 `data: [DONE]`。

### Anthropic Messages 接口

请求：

```bash
curl --noproxy '*' -X POST 'http://127.0.0.1:8080/v1/messages?token_length=3' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 1024,
    "system": "这是一个 Anthropic 协议回复。",
    "messages": [
      {"role": "user", "content": "你好"}
    ],
    "stream": false
  }'
```

说明：

- `stream: false` 返回 Anthropic `message` JSON，回复内容位于 `content[0].text`。
- `stream: true` 返回 Anthropic 风格 SSE 事件。
- 服务端会使用顶层 `system` 字段作为模型回复内容。
- `token_length` 默认值为 `3`，按字符分片。
- 流式响应包含 `message_start`、`content_block_start`、`content_block_delta`、`content_block_stop`、`message_delta` 和 `message_stop` 事件。
- Anthropic 流式响应不会发送 `data: [DONE]`。

### 自定义聊天接口

请求：

```bash
curl --noproxy '*' -X POST 'http://127.0.0.1:8080/v1/custom/chat?token_length=3' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "deepseek-r1:1.5b",
    "prompt_id": "demo-001",
    "prompt": [
      {"role": "system", "content": "这是一个自定义协议回复。"},
      {"role": "user", "content": "你好"}
    ],
    "is_stream": false
  }'
```

说明：

- `is_stream: false` 返回一次性 JSON。
- `is_stream: true` 返回 SSE 分片。
- 流式响应最后会发送 `data: [FINISH]`。
- 每个响应分片包含 `prompt_id`、`reply`、`model`、`response_uuid`、`response_timestamp` 和 `is_stop`。

## 前端测试页

页面位于 `static/` 目录，没有构建流程，直接修改 HTML/CSS 即可生效。测试页支持：

- 添加和删除多条用户消息
- 添加和删除多条模型答复片段
- 切换流式 / 非流式请求
- 设置模型名、Token 长度、Anthropic `max_tokens` 和可选 `X-Real-IP`
- 在右侧响应面板中查看请求结果

## 实现要点

核心逻辑集中在 `main.go`：

1. 解析请求 JSON。
2. 提取 OpenAI / 自定义协议里的 `system` 消息，或 Anthropic 请求里的顶层 `system` 字段。
3. 将内容按 `token_length` 做 rune 级切片。
4. 根据流式开关返回 JSON 或 SSE。
5. OpenAI / 自定义协议逐块写入 `data: <json>\n\n` 并立即 flush；Anthropic 协议写入 `event: <name>\ndata: <json>\n\n` 并立即 flush。

## 注意事项

- 这是本地 Mock 服务，不适合直接作为生产服务。
- 服务默认绑定 `[::]:8080`，在部分环境下可能对局域网可见。
- 当前没有真实鉴权、限流、并发隔离或持久化能力。
- 不要在项目中提交真实 API Key 或敏感配置。
