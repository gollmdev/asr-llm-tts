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
	for {
		select {
		case <-ctx.Done():
			return nil
		case message, ok := <-rt.Input():
			if !ok {
				log.Println("TTSStream completed")
				return nil
			}
			tools := []map[string]any{}
			if n.Tools != nil {
				tools = n.Tools
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
			llm := llm.NewQwenChatModel(&llm.ChatModelConfig{
				Model: "qwen-plus",
				Tools: tools,
				Ctx:   ctx,
				G:     g,
			})

			llm.Stream([]map[string]any{
				{"role": "user", "content": message.Data.(string)},
			})

			for {
				msg, err := llm.Recv()
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
						rt.Emit(&dag.Event{
							Type: "llm_chunk",
							From: n.ID(),
							Data: chunk,
						})
					}
				}
			}

			log.Println("llm consumer close!")

			return nil
		}
	}
}
