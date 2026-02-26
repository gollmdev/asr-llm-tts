package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type PrintHandler struct {
	OnTextFunc func(delta string)
}

func (h *PrintHandler) OnText(delta string) {
	// fmt.Print(delta)
	if h.OnTextFunc != nil {
		h.OnTextFunc(delta)
	}
}

func (h *PrintHandler) OnToolCallDelta(id, name, args string) {
	fmt.Printf("\n[id %s] [tool %s] args chunk: %s\n", id, name, args)
}

func (h *PrintHandler) OnToolCallFinish(toolCalls map[string]*ToolCallState) {
	fmt.Println("\n\n=== TOOL CALLS ===")
	for id, call := range toolCalls {
		fmt.Printf("id=%s name=%s args=%s\n",
			id,
			call.Name,
			call.Arguments.String(),
		)
	}
}

func (h *PrintHandler) OnDone() {
	// fmt.Println("\n\n[stream done]")
}

func (h *PrintHandler) OnError(err error) {
	fmt.Println("llm OnError stream error:", err)
}
func (h *PrintHandler) OnUsage(usage map[string]any) {
	fmt.Printf("[usage] prompt_tokens=%v completion_tokens=%v total_tokens=%v\n",
		usage["prompt_tokens"],
		usage["completion_tokens"],
		usage["total_tokens"],
	)
}

type ToolCallState struct {
	Name      string
	Arguments strings.Builder
}

// define clinet struct and methods to call llm provider, e.g. dashscope
type LLMStream struct {
	url            string
	apiKey         string
	model          string
	tools          []map[string]any
	callback       StreamEventCallback
	currentMessage []map[string]any
	toolCalls      map[string]*ToolCallState
}

func NewLLMStream(model string,
	tools []map[string]any,
	OnTextFunc func(delta string)) *LLMStream {
	return &LLMStream{
		url:    "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
		apiKey: os.Getenv("DASHSCOPE_API_KEY"),
		model:  model,
		tools:  tools,
		callback: &PrintHandler{
			OnTextFunc: OnTextFunc,
		},
		currentMessage: make([]map[string]any, 0),
		// toolCalls:      make(map[string]*ToolCallState),
	}
}

func (l *LLMStream) reqPayload(message []map[string]any, stream bool) map[string]any {
	body := map[string]any{
		"model":    l.model,
		"messages": message,
		"stream":   stream,
		"tools":    l.tools,
		// "stream_options": map[string]any{
		// 	"include_usage": true,
		// },
	}
	if stream {
		body["stream_options"] = map[string]any{
			"include_usage": true,
		}
	}
	// b, _ := json.Marshal(body)
	return body
}

func (l *LLMStream) buildRequest(ctx context.Context, message []map[string]any, stream bool) (*http.Request, error) {
	paylaod := l.reqPayload(message, stream)
	b, err := json.Marshal(paylaod)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+l.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return req, nil

}

func (l *LLMStream) Call(ctx context.Context, message []map[string]any, stream bool) error {

	client := &http.Client{Timeout: 0}
	l.currentMessage = append(l.currentMessage, message...)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:

		}
		l.toolCalls = make(map[string]*ToolCallState)
		req, err := l.buildRequest(ctx, l.currentMessage, stream)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			// Read body for debugging (limit size to avoid huge dumps)
			data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			// panic()
			log.Printf("request failed: %s - %s", resp.Status, string(data))
			return fmt.Errorf("request failed: %s - %s", resp.Status, string(data))
		}

		reader := NewChatStreamReader(resp.Body, l.callback, l.toolCalls, stream)
		reader.Run(ctx)
		if len(l.toolCalls) != 0 {
			i := 0
			for id, call := range l.toolCalls {
				assistantMessage := map[string]any{
					"role":    "assistant",
					"content": "",
					"tool_calls": []map[string]any{
						{
							"id":    id,
							"type":  "function",
							"index": i,
							"function": map[string]string{
								"arguments": call.Arguments.String(),
								"name":      call.Name,
							},
						},
					},
				}
				l.currentMessage = append(l.currentMessage, assistantMessage)
				i++

				// tool call
				log.Println("Tool call:", id, call.Name, call.Arguments.String())
				// {"role": "tool", "content": function_output, "tool_call_id": completion.choices[0].message.tool_calls[0].id}
				l.currentMessage = append(l.currentMessage, map[string]any{
					"role":         "tool",
					"name":         call.Name,
					"content":      "多云",
					"tool_call_id": id,
				})
			}

			// no tool calls, finish after first response
			// return nil
		} else {
			return nil
		}
		// return nil
	}

}
