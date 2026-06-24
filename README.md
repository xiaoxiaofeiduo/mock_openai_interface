# my-gin-project

一个基于 Go + Gin 的本地 Mock 服务，用于模拟 OpenAI 风格和自定义协议的聊天响应。项目主要面向前端联调、流式响应调试和接口行为验证，不会调用真实大模型。

## 功能特性

- 支持 OpenAI 兼容接口：`POST /v1/chat/completions`
- 支持自定义聊天接口：`POST /v1/custom/chat`
- 同时支持流式 SSE 响应和普通 JSON 响应
- 可通过 `token_length` 控制每个流式分片长度
- 提供静态测试页面，方便手动构造请求和查看响应
- 支持回显请求头 `X-Real-IP` 到响应头 `X-Echoed-Real-IP`

## 项目结构

```text
.
├── main.go                         # Gin 服务入口和所有接口处理逻辑
├── static/
│   ├── main.html                   # 测试页面导航
│   ├── simulate_chat_ai.html       # OpenAI 兼容接口测试页
│   ├── simulate_customize_chat.html # 自定义协议接口测试页
│   ├── app.css                     # 前端页面公共样式
│   └── favicon.ico
├── docs/
│   └── mock-chat-server-blog.md    # 项目分享博客
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

打开根路径后，可以选择两个测试页面：

- OpenAI 聊天模拟测试：`/static/simulate_chat_ai.html`
- 自定义流式响应测试：`/static/simulate_customize_chat.html`

## 构建与测试

```bash
go build -o my-gin-project .
go test ./...
go vet ./...
gofmt -w main.go
```

当前项目暂无自动化测试文件，`go test ./...` 会显示 `[no test files]`。

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
- 设置模型名、Token 长度和可选 `X-Real-IP`
- 在右侧响应面板中查看请求结果

## 实现要点

核心逻辑集中在 `main.go`：

1. 解析请求 JSON。
2. 提取所有 `system` 消息。
3. 将内容按 `token_length` 做 rune 级切片。
4. 根据流式开关返回 JSON 或 SSE。
5. 对流式响应逐块写入 `data: <json>\n\n` 并立即 flush。

## 注意事项

- 这是本地 Mock 服务，不适合直接作为生产服务。
- 服务默认绑定 `[::]:8080`，在部分环境下可能对局域网可见。
- 当前没有真实鉴权、限流、并发隔离或持久化能力。
- 不要在项目中提交真实 API Key 或敏感配置。
