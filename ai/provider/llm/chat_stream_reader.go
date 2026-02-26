package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
)

type StreamEventCallback interface {
	OnText(delta string)
	OnToolCallDelta(id string, name string, argumentsChunk string)
	OnToolCallFinish(toolCalls map[string]*ToolCallState)
	OnDone()
	OnError(err error)
	OnUsage(usage map[string]any)
}
type ChatStreamReader struct {
	reader    *bufio.Reader
	handler   StreamEventCallback
	toolCalls map[string]*ToolCallState
	stream    bool
	// OnToolCallFinish func(toolCalls map[string]*ToolCallState)

}

func NewChatStreamReader(r io.Reader, h StreamEventCallback, toolCalls map[string]*ToolCallState, stream bool) *ChatStreamReader {
	return &ChatStreamReader{
		reader:    bufio.NewReader(r),
		handler:   h,
		toolCalls: toolCalls,
		stream:    stream,
	}
}

//	{
//	    "choices": [],
//	    "object": "chat.completion.chunk",
//	    "usage": {
//	        "prompt_tokens": 22,
//	        "completion_tokens": 66,
//	        "total_tokens": 88,
//	        "prompt_tokens_details": {
//	            "cached_tokens": 0
//	        }
//	    },
//	    "created": 1770364655,
//	    "system_fingerprint": null,
//	    "model": "qwen-plus",
//	    "id": "chatcmpl-8388ad93-bab4-9f14-87ba-6289f3b144c9"
//	}
func (c *ChatStreamReader) Run(ctx context.Context) {
	defer c.handler.OnDone()

	var currentToolCallID string

	for {
		select {
		case <-ctx.Done():
			c.handler.OnError(ctx.Err())
			return
		default:
		}

		if c.stream {
			line, err := c.reader.ReadString('\n')

			if err != nil {
				if err != io.EOF {
					c.handler.OnError(err)
				}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			if !strings.HasPrefix(line, "data:") {
				continue
			}

			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" {
				return
			}

			var obj map[string]any
			if err := json.Unmarshal([]byte(payload), &obj); err != nil {
				continue
			}

			if usage, ok := obj["usage"].(map[string]any); ok {
				c.handler.OnUsage(usage)
				continue
			}

			choices, ok := obj["choices"].([]any)
			if !ok || len(choices) == 0 {
				continue
			}

			choice := choices[0].(map[string]any)

			// finish_reason
			if fr, ok := choice["finish_reason"].(string); ok && fr == "tool_calls" {
				c.handler.OnToolCallFinish(c.toolCalls)
				// if c.OnToolCallFinish != nil {
				// 	c.OnToolCallFinish(c.toolCalls)
				// }
				return
			}
			message, ok := choice["message"].(map[string]any)
			if ok {
				if content, ok := message["content"].(string); ok {
					c.handler.OnText(content)
				}
			}
			delta, ok := choice["delta"].(map[string]any)
			if ok {
				// 2️ tool_calls
				if tcList, ok := delta["tool_calls"].([]any); ok {
					for _, tc := range tcList {
						tcMap := tc.(map[string]any)

						id, _ := tcMap["id"].(string)
						if id != "" {
							currentToolCallID = id
						}

						state := c.toolCalls[currentToolCallID]
						if state == nil {
							state = &ToolCallState{}
							c.toolCalls[currentToolCallID] = state
						}

						if fn, ok := tcMap["function"].(map[string]any); ok {
							if name, ok := fn["name"].(string); ok && state.Name == "" {
								state.Name = name
							}
							if args, ok := fn["arguments"].(string); ok {
								state.Arguments.WriteString(args)
								c.handler.OnToolCallDelta(
									currentToolCallID,
									state.Name,
									args,
								)
							}
						}
					}
				} else if content, ok := delta["content"].(string); ok {
					// 1️ 普通文本
					c.handler.OnText(content)
					continue
				}
			}
		} else {
			line, _ := c.reader.ReadString('\n')
			// if err != nil {
			// 	if err != io.EOF {
			// 		c.handler.OnError(err)
			// 	}
			// 	return
			// }
			var obj map[string]any
			if err := json.Unmarshal([]byte(line), &obj); err != nil {
				continue
			}
			choices, ok := obj["choices"].([]any)
			if !ok || len(choices) == 0 {
				continue
			}
			choice := choices[0].(map[string]any)

			message, ok := choice["message"].(map[string]any)
			if ok {
				if content, ok := message["content"].(string); ok {
					c.handler.OnText(content)
				}
			}
			return
		}

	}
}
