package node

import (
	"io"
	"log"

	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
	"github.com/gollmdev/asr-llm-tts/ai/dag/dagtypes"
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
		case ev, ok := <-rt.Input():

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
			// var messages []*llm.Message
			// if messages, ok = ev.Data.([]*llm.Message); !ok {
			// 	log.Println("invalid message data")
			// 	continue
			// }
			payload, ok := ev.Data.(*dagtypes.PromptPayload)
			if !ok || payload == nil {
				log.Println("[chat] invalid prompt payload")
				continue
			}
			llmModel.Stream(dagtypes.ConvertMessages(payload.Messages))

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
						// time.Sleep(2 * time.Second)
						rt.Emit(&dag.Event{
							Type: "llm_chunk",
							From: n.ID(),
							Data: chunk,
						})
					} else if msg.Event == "OnToolCallFinish" && msg.ToolCalls != nil {
						// rt.Emit(&dag.Event{
						// 	Type: "llm_tool_call",
						// 	From: n.ID(),
						// 	Data: msg.ToolCalls,
						// })
						tools := msg.ToolCalls
						// 将 *map[string]*ToolCallState 转换成  []dagtypes.ToolCall
						var toolCalls []dagtypes.ToolCall
						for id, t := range *tools {
							toolCalls = append(toolCalls, dagtypes.ToolCall{
								Name:      t.Name,
								Arguments: t.Arguments.String(),
								ID:        id,
							})
						}

						// 这里可以根据需要过滤工具调用，比如只发送特定工具的调用
						rt.Emit(&dag.Event{
							Type: "llm_tool_call",
							Data: &dagtypes.ToolCallPayload{
								Context:   payload.Context,
								Messages:  payload.Messages,
								ToolCalls: toolCalls,
							},
							Rtx: ev.Rtx,
						})
					}
				}
			}

			log.Println("llm consumer close!")

			return nil
		}
	}
}
