package node

import (
	"io"
	"log"

	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
	"github.com/gollmdev/asr-llm-tts/ai/provider/llm"
	"golang.org/x/sync/errgroup"
)

type LLMNode struct {
	Tools []map[string]any
}

func (n *LLMNode) ID() string { return "llm" }
func (n *LLMNode) Mode() dag.NodeMode {
	return dag.ModeLazy
}

func (n *LLMNode) Run(
	rt dag.NodeRuntime,
) error {
	g, ctx := errgroup.WithContext(rt.Context())
	tools := []map[string]any{}
	if n.Tools != nil {
		tools = n.Tools
	}
	// sessionID := rt.RuntimeContext().SessionID
	for {
		select {
		case <-ctx.Done():
			return nil
		case message, ok := <-rt.Input():
			if !ok {
				log.Println("TTSStream completed")
				return nil
			}

			// tools := []map[string]any{
			// 	{
			// 		"type": "function",
			// 		"function": map[string]any{
			// 			"name":        "get_weather",
			// 			"description": "当你想查询指定城市的天气时非常有用。",
			// 			"parameters": map[string]any{
			// 				"type": "object",
			// 				"properties": map[string]any{
			// 					"location": map[string]string{
			// 						"type":        "string",
			// 						"description": "城市或县区，比如北京市、杭州市、余杭区等。",
			// 					},
			// 				},
			// 				"required": []string{"location"},
			// 			},
			// 		},
			// 	},
			// }
			llmModel := llm.NewQwenChatModel(&llm.ChatModelConfig{
				Model: "qwen-plus",
				Tools: tools,
				Ctx:   ctx,
				G:     g,
			})
			// userText := message.Data.(string)

			// 1️⃣ 写入 user message
			// rt.RuntimeContext().Memory.Append(sessionID, &llm.Message{
			// 	Role:    "user",
			// 	Content: userText,
			// })
			// messages := rt.RuntimeContext().Memory.Get(sessionID)
			var messages []*llm.Message
			if messages, ok = message.Data.([]*llm.Message); !ok {
				log.Println("invalid message data")
				continue
			}

			llmModel.Stream(convertMessages(messages))

			for {
				msg, err := llmModel.Recv()
				if err != nil {
					if err != io.EOF {
						log.Println("LLMStream error:", err)
					}
					break
				}
				if msg != nil {
					if msg.Event == "OnText" && msg.Content != nil {
						// onChunkReceived(*msg.Content)
						chunk := *msg.Content
						// s.sendJson(SessionText, "message", chunk)
						// s.FullResponse.WriteString(chunk)
						// s.bus.Publish(event.Event{Type: event.EventLLMChunk, Data: chunk})
						// 模拟llm延迟发送
						// time.Sleep(1 * time.Second)
						rt.Emit(&dag.Event{
							Type: "llm_chunk",
							From: n.ID(),
							Data: chunk,
						})
					} else if msg.Event == "OnToolCallFinish" && msg.ToolCalls != nil {
						rt.Emit(&dag.Event{
							Type: "llm_tool_call",
							From: n.ID(),
							Data: msg.ToolCalls,
						})
					}
				}
			}

			log.Println("llm consumer close!")

			return nil
		}
	}
}
func convertMessages(msgs []*llm.Message) []map[string]any {
	var res []map[string]any
	for _, m := range msgs {
		item := map[string]any{
			"role": m.Role,
		}
		if m.Content != "" {
			item["content"] = m.Content
		}
		if m.ToolCalls != nil {
			item["tool_calls"] = m.ToolCalls
		}
		if m.ToolCallID != "" {
			item["tool_call_id"] = m.ToolCallID
		}
		if m.Name != "" {
			item["name"] = m.Name
		}
		res = append(res, item)
	}
	return res
}
