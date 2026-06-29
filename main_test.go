package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAnthropicMessagesNonStreaming(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := setupRouter()

	body := `{"model":"claude-test","max_tokens":128,"system":"你好世界","messages":[{"role":"user","content":"测试"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages?token_length=2", strings.NewReader(body))
	req.Header.Set("Content-Type", jsonContentType)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var got AnthropicMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if got.Type != "message" || got.Role != "assistant" || got.Model != "claude-test" {
		t.Fatalf("unexpected message metadata: %+v", got)
	}
	if len(got.Content) != 1 || got.Content[0].Type != "text" || got.Content[0].Text != "你好世界" {
		t.Fatalf("unexpected content: %+v", got.Content)
	}
	if got.StopReason == nil || *got.StopReason != "end_turn" {
		t.Fatalf("stop_reason = %v, want end_turn", got.StopReason)
	}
}

func TestAnthropicMessagesStreamingEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := setupRouter()

	body := `{"model":"claude-test","max_tokens":128,"system":"你好世界","messages":[{"role":"user","content":"测试"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages?token_length=2", strings.NewReader(body))
	req.Header.Set("Content-Type", jsonContentType)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); contentType != sseContentType {
		t.Fatalf("content type = %q, want %q", contentType, sseContentType)
	}

	stream := rec.Body.String()
	for _, want := range []string{
		"event: message_start",
		"event: content_block_start",
		`"type":"text_delta"`,
		`"text":"你好"`,
		`"text":"世界"`,
		"event: content_block_stop",
		"event: message_delta",
		`"stop_reason":"end_turn"`,
		"event: message_stop",
	} {
		if !strings.Contains(stream, want) {
			t.Fatalf("stream does not contain %q:\n%s", want, stream)
		}
	}
	if strings.Contains(stream, "[DONE]") {
		t.Fatalf("anthropic stream should not contain [DONE]:\n%s", stream)
	}
}
