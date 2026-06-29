package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-uuid"
)

// RequestMessage 定义请求中的消息结构体
type RequestMessage struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

// RequestBody 定义请求体的结构体
type RequestBody struct {
	Model    string           `json:"model"`
	Messages []RequestMessage `json:"messages"`
	Stream   bool             `json:"stream"`
}

type AnthropicRequestBody struct {
	Model     string           `json:"model"`
	MaxTokens int              `json:"max_tokens"`
	System    string           `json:"system"`
	Messages  []RequestMessage `json:"messages"`
	Stream    bool             `json:"stream"`
}

type AnthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type AnthropicMessage struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Model        string                  `json:"model"`
	Content      []AnthropicContentBlock `json:"content"`
	StopReason   *string                 `json:"stop_reason"`
	StopSequence *string                 `json:"stop_sequence"`
	Usage        AnthropicUsage          `json:"usage"`
}

// ChoiceDelta 定义响应中的 delta 结构体
type ChoiceDelta struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Choice 定义响应中的 choice 结构体
type Choice struct {
	Index        int         `json:"index"`
	Delta        ChoiceDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

// ResponseChunk 定义响应块的结构体
type ResponseChunk struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	SystemFingerprint string   `json:"system_fingerprint"`
	Choices           []Choice `json:"choices"`
}

type CustomPrompt struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

type CustomRequestBody struct {
	Model    string         `json:"model"`
	PromptId string         `json:"prompt_id"`
	Prompt   []CustomPrompt `json:"prompt"`
	IsStream bool           `json:"is_stream"`
}

type CustomResponseModel struct {
	Name              string `json:"name"`
	SystemFingerprint string `json:"system_fingerprint"`
}

type CustomResponseChunk struct {
	PromptId          string              `json:"prompt_id"`
	Reply             string              `json:"reply"`
	Model             CustomResponseModel `json:"model"`
	ResponseUUID      string              `json:"response_uuid"`
	ResponseTimestamp time.Time           `json:"response_timestamp"`
	IsStop            string              `json:"is_stop"`
}

// 模拟流式响应的内容片段
var responseChunks []string
var customResponseChunks []string

const streamChunkDelay = 300 * time.Millisecond
const jsonContentType = "application/json; charset=utf-8"
const sseContentType = "text/event-stream; charset=utf-8"

// echoRealIP 读取请求头 X-Real-IP,若非空则回写到响应头 X-Echoed-Real-IP。
// 必须在响应写入(WriteHeader/c.JSON)之前调用。
func echoRealIP(c *gin.Context) {
	if ip := c.GetHeader("X-Real-IP"); ip != "" {
		c.Writer.Header().Set("X-Echoed-Real-IP", ip)
	}
}

func setSSEHeaders(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", sseContentType)
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
}

func writeJSON(c *gin.Context, code int, obj interface{}) {
	c.Writer.Header().Set("Content-Type", jsonContentType)
	c.JSON(code, obj)
}

func splitByTokenLength(content string, tokenLength int) []string {
	if tokenLength <= 0 {
		tokenLength = 3
	}

	var chunks []string
	runes := []rune(content)
	for i := 0; i < len(runes); i += tokenLength {
		end := i + tokenLength
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}

func writeSSEEvent(c *gin.Context, event string, obj interface{}) bool {
	respBytes, err := json.Marshal(obj)
	if err != nil {
		log.Printf("Failed to marshal %s event: %v", event, err)
		return false
	}
	c.Writer.WriteString("event: " + event + "\n")
	c.Writer.WriteString("data: " + string(respBytes) + "\n\n")
	c.Writer.(http.Flusher).Flush()
	return true
}

func setupRouter() *gin.Engine {
	r := gin.Default()

	// 静态文件服务
	r.Static("/static", "./static")

	// 根路径返回导航页
	r.GET("/", func(c *gin.Context) {
		c.File("./static/main.html")
	})
	r.HEAD("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// 浏览器默认请求的 favicon,直接返回 static 下的文件
	r.GET("/favicon.ico", func(c *gin.Context) {
		c.Header("Content-Type", "image/x-icon")
		c.File("./static/favicon.ico")
	})

	// Chrome DevTools 自动探测的元数据,返回空 200 避免 404
	r.GET("/.well-known/appspecific/com.chrome.devtools.json", func(c *gin.Context) {
		writeJSON(c, http.StatusOK, gin.H{})
	})

	// 新增路由，模拟 OpenAI 流式响应
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		var req RequestBody
		if err := c.ShouldBindJSON(&req); err != nil {
			writeJSON(c, http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 获取 token_length 查询参数
		tokenLengthStr := c.Query("token_length")
		if tokenLengthStr == "" {
			tokenLengthStr = "3"
		}
		var tokenLength int
		if tokenLengthStr != "" {
			var err error
			tokenLength, err = strconv.Atoi(tokenLengthStr)
			if err != nil {
				writeJSON(c, http.StatusBadRequest, gin.H{"error": "token_length must be an integer"})
				return
			}
		}

		echoRealIP(c)

		// 分开存储 user 和 system 的 content
		var userContents []string
		var systemContents []string
		for _, msg := range req.Messages {
			switch msg.Role {
			case "user":
				userContents = append(userContents, msg.Content)
			case "system":
				systemContents = append(systemContents, msg.Content)
			}
		}

		// 将所有 systemContents 拼接成一个大字符串
		combinedContent := ""
		for _, content := range systemContents {
			combinedContent += content
		}

		// 按照 token_length 切分 combinedContent 并更新 responseChunks
		var newResponseChunks []string
		runes := []rune(combinedContent) // 将字符串转换为 rune 切片，按字符处理
		for i := 0; i < len(runes); i += tokenLength {
			end := i + tokenLength
			if end > len(runes) {
				end = len(runes)
			}
			newResponseChunks = append(newResponseChunks, string(runes[i:end])) // 将 rune 切片转换回字符串
		}
		responseChunks = newResponseChunks

		if !req.Stream {
			// 如果不要求流式响应，返回一个完整的响应
			id := "chatcmpl-115"
			object := "chat.completion"
			created := time.Now().Unix()
			model := req.Model
			systemFingerprint := "fp_ollama"
			content := ""
			for _, chunk := range systemContents {
				content += chunk
			}

			finishReason := "stop"
			promptTokens := 7
			completionTokens := 22
			totalTokens := 29

			fullResponse := gin.H{
				"id":                 id,
				"object":             object,
				"created":            created,
				"model":              model,
				"system_fingerprint": systemFingerprint,
				"choices": []gin.H{
					{
						"index": 0,
						"message": gin.H{
							"role":    "assistant",
							"content": content,
						},
						"finish_reason": finishReason,
					},
				},
				"usage": gin.H{
					"prompt_tokens":     promptTokens,
					"completion_tokens": completionTokens,
					"total_tokens":      totalTokens,
				},
			}
			writeJSON(c, http.StatusOK, fullResponse)
			return
		}

		// 设置响应头,显式声明 UTF-8,避免 Python2 客户端按 ISO-8859-1 解码中文。
		setSSEHeaders(c)
		c.Writer.WriteHeader(http.StatusOK)

		// 开始流式响应
		id := "chatcmpl-68"
		model := req.Model
		created := time.Now().Unix()
		for i, chunk := range responseChunks {
			var finishReason *string // 声明 finishReason 为指针类型
			if i == len(responseChunks)-1 {
				finishReason = nil // 最后一个数据块的 finishReason 设为 nil
			} else {
				finishReason = nil // 中间块的 finishReason 也为 nil
			}
			respChunk := ResponseChunk{
				ID:                id,
				Object:            "chat.completion.chunk",
				Created:           created,
				Model:             model,
				SystemFingerprint: "fp_ollama",
				Choices: []Choice{
					{
						Index: 0,
						Delta: ChoiceDelta{
							Role:    "assistant",
							Content: chunk,
						},
						FinishReason: finishReason, // 使用指针类型
					},
				},
			}
			respBytes, err := json.Marshal(respChunk)
			if err != nil {
				log.Printf("Failed to marshal response chunk: %v", err)
				return
			}
			// 发送分块响应
			line := "data: " + string(respBytes) + "\n\n"
			c.Writer.WriteString(line)
			c.Writer.(http.Flusher).Flush()
			// 模拟大模型逐块思考和输出的停顿。
			time.Sleep(streamChunkDelay)
		}

		// 添加一个额外的块，content 为空，reason 为 "stop"
		stopReason := "stop"
		finalChunk := ResponseChunk{
			ID:                id,
			Object:            "chat.completion.chunk",
			Created:           created,
			Model:             model,
			SystemFingerprint: "fp_ollama",
			Choices: []Choice{
				{
					Index: 0,
					Delta: ChoiceDelta{
						Role:    "assistant",
						Content: "", // content 为空
					},
					FinishReason: &stopReason, // reason 为 "stop"
				},
			},
		}
		finalBytes, err := json.Marshal(finalChunk)
		if err != nil {
			log.Printf("Failed to marshal final response chunk: %v", err)
			return
		}
		// 发送结束块
		finalLine := "data: " + string(finalBytes) + "\n\n"
		c.Writer.WriteString(finalLine)
		c.Writer.WriteString("data: [DONE]\n\n")
		c.Writer.(http.Flusher).Flush()
	})

	r.POST("/v1/messages", func(c *gin.Context) {
		var req AnthropicRequestBody
		if err := c.ShouldBindJSON(&req); err != nil {
			writeJSON(c, http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": err.Error()}})
			return
		}

		tokenLengthStr := c.Query("token_length")
		if tokenLengthStr == "" {
			tokenLengthStr = "3"
		}
		tokenLength, err := strconv.Atoi(tokenLengthStr)
		if err != nil {
			writeJSON(c, http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "token_length must be an integer"}})
			return
		}

		echoRealIP(c)

		combinedContent := req.System
		chunks := splitByTokenLength(combinedContent, tokenLength)
		stopReason := "end_turn"
		messageID := "msg_mock_01"
		model := req.Model
		if model == "" {
			model = "claude-3-5-sonnet-20241022"
		}
		outputTokens := len([]rune(combinedContent))
		message := AnthropicMessage{
			ID:    messageID,
			Type:  "message",
			Role:  "assistant",
			Model: model,
			Content: []AnthropicContentBlock{
				{
					Type: "text",
					Text: combinedContent,
				},
			},
			StopReason: &stopReason,
			Usage: AnthropicUsage{
				InputTokens:  len(req.Messages) + len([]rune(req.System)),
				OutputTokens: outputTokens,
			},
		}

		if !req.Stream {
			writeJSON(c, http.StatusOK, message)
			return
		}

		// Anthropic Messages API 使用 event + data 的 SSE 事件序列,不追加 [DONE] 标记。
		setSSEHeaders(c)
		c.Writer.WriteHeader(http.StatusOK)

		startMessage := message
		startMessage.Content = []AnthropicContentBlock{}
		startMessage.StopReason = nil
		startMessage.Usage.OutputTokens = 0
		if !writeSSEEvent(c, "message_start", gin.H{
			"type":    "message_start",
			"message": startMessage,
		}) {
			return
		}

		if !writeSSEEvent(c, "content_block_start", gin.H{
			"type":  "content_block_start",
			"index": 0,
			"content_block": gin.H{
				"type": "text",
				"text": "",
			},
		}) {
			return
		}

		for _, chunk := range chunks {
			if !writeSSEEvent(c, "content_block_delta", gin.H{
				"type":  "content_block_delta",
				"index": 0,
				"delta": gin.H{
					"type": "text_delta",
					"text": chunk,
				},
			}) {
				return
			}
			time.Sleep(streamChunkDelay)
		}

		if !writeSSEEvent(c, "content_block_stop", gin.H{
			"type":  "content_block_stop",
			"index": 0,
		}) {
			return
		}

		if !writeSSEEvent(c, "message_delta", gin.H{
			"type": "message_delta",
			"delta": gin.H{
				"stop_reason":   stopReason,
				"stop_sequence": nil,
			},
			"usage": gin.H{
				"output_tokens": outputTokens,
			},
		}) {
			return
		}

		writeSSEEvent(c, "message_stop", gin.H{"type": "message_stop"})
	})

	r.POST("/v1/custom/chat", func(c *gin.Context) {
		var req CustomRequestBody
		if err := c.ShouldBindJSON(&req); err != nil {
			writeJSON(c, http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		// 获取 token_length 查询参数
		tokenLengthStr := c.Query("token_length")
		if tokenLengthStr == "" {
			tokenLengthStr = "3"
		}
		tokenLength, err := strconv.Atoi(tokenLengthStr)
		if err != nil {
			writeJSON(c, http.StatusBadRequest, gin.H{"error": "Invalid token_length parameter"})
			return
		}

		echoRealIP(c)

		// 分开存储 user 和 system 的 content
		var userContents []string
		var systemContents []string
		for _, message := range req.Prompt {
			if message.Role == "user" {
				userContents = append(userContents, message.Content)
			} else if message.Role == "system" {
				systemContents = append(systemContents, message.Content)
			}
		}

		combinedContent := ""
		for _, content := range systemContents {
			combinedContent += content
		}
		var customResponseChunks []string
		runes := []rune(combinedContent) // 将字符串转换为 rune 切片，按字符处理
		for i := 0; i < len(runes); i += tokenLength {
			end := i + tokenLength
			if end > len(runes) {
				end = len(runes)
			}
			customResponseChunks = append(customResponseChunks, string(runes[i:end]))
		}
		if !req.IsStream {

			writeJSON(c, http.StatusOK, gin.H{
				"prompt_id": req.PromptId,
				"uuid": func() string {
					uuidStr, _ := uuid.GenerateUUID()
					return uuidStr
				}(),
				"timestamp": time.Now().Unix(),
				"reply":     combinedContent,
				"model": gin.H{
					"name":               req.Model,
					"system_fingerprint": "fp_custom",
				},
			})
			return
		}

		// 设置响应头,显式声明 UTF-8,避免 Python2 客户端按 ISO-8859-1 解码中文。
		setSSEHeaders(c)
		c.Writer.WriteHeader(http.StatusOK)

		//开始流式响应
		for i, chunk := range customResponseChunks {
			var isStop string
			if i == len(customResponseChunks)-1 {
				isStop = "false"
			} else {
				isStop = "false"
			}
			respChunk := CustomResponseChunk{
				PromptId: req.PromptId,
				ResponseUUID: func() string {
					uuidStr, _ := uuid.GenerateUUID()
					return uuidStr
				}(),
				ResponseTimestamp: time.Now(),
				Reply:             chunk,
				Model: CustomResponseModel{
					Name:              req.Model,
					SystemFingerprint: "fp_custom",
				},
				IsStop: isStop,
			}
			respBytes, err := json.Marshal(respChunk)
			if err != nil {
				log.Printf("Failed to marshal response chunk: %v", err)
				return
			}
			line := "data: " + string(respBytes) + "\n\n"
			c.Writer.WriteString(line)
			c.Writer.(http.Flusher).Flush()
			// 模拟大模型逐块思考和输出的停顿。
			time.Sleep(streamChunkDelay)
		}
		// 添加一个额外的块，content 为空，is_stop 为 true
		finalChunk := CustomResponseChunk{
			PromptId: req.PromptId,
			ResponseUUID: func() string {
				uuidStr, _ := uuid.GenerateUUID()
				return uuidStr
			}(),
			ResponseTimestamp: time.Now(),
			Reply:             "",
			Model: CustomResponseModel{
				Name:              req.Model,
				SystemFingerprint: "fp_custom",
			},
			IsStop: "true",
		}
		finalBytes, err := json.Marshal(finalChunk)
		if err != nil {
			log.Printf("Failed to marshal final response chunk: %v", err)
			return
		}
		// 发送结束块
		finalLine := "data: " + string(finalBytes) + "\n\n"
		c.Writer.WriteString(finalLine)
		c.Writer.WriteString("data: [FINISH]\n\n")
		c.Writer.(http.Flusher).Flush()
	})

	return r
}

func main() {
	r := setupRouter()

	// 启动服务器，默认在 0.0.0.0:8080 启动服务
	if err := r.Run("[::]:8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
