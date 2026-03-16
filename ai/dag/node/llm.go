package node

import (
	"io"
	"log"

	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
	"github.com/gollmdev/asr-llm-tts/ai/provider/llm"
	"golang.org/x/sync/errgroup"
)

type LLMNode struct{}

func (n *LLMNode) ID() string { return "llm" }
func (n *LLMNode) Mode() dag.NodeMode {
	return dag.ModeLazy
}

func (n *LLMNode) Run(
	// ctx context.Context,
	rt dag.NodeRuntime,
	// in <-chan dag.Event,
	// out chan<- dag.Event,
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
			// if s.callback != nil {
			// 	// s.callback.OnEvent(EventUserMessage, message.Data.(string), func(subMessage []map[string]any) {
			// 	// 	s.bus.Publish(Event{Type: EventUserMessage, Data: subMessage})
			// 	// })
			// 	s.callback.OnEvent(s.eventCtx, event.EventUserMessage, message.Data.(string))
			// }
			// input := s.callback.GetMessage(message.Data.(string))
			// s.LLMStream(input, true, func(chunk string) {
			// 	// log.Println("LLMStream received chunk:", chunk)
			// 	s.sendJson(SessionText, "message", chunk)
			// 	s.FullResponse.WriteString(chunk)
			// 	s.bus.Publish(Event{Type: EventLLMChunk, Data: chunk})
			// })
			tools := []map[string]any{
				{
					"type": "function",
					"function": map[string]any{
						"name":        "get_weather",
						"description": "当你想查询指定城市的天气时非常有用。",
						"parameters": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"location": map[string]string{
									"type":        "string",
									"description": "城市或县区，比如北京市、杭州市、余杭区等。",
								},
							},
							"required": []string{"location"},
						},
					},
				},
			}
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

			// if s.callback != nil {
			// 	defer log.Println("llm-life: llm recv done")

			// }

			// s.bus.Publish(event.Event{Type: event.EventLLMDone, Data: nil})

			// 获取完整响应（此时才转为 string）
			// finalResponse := s.FullResponse.String()
			// if s.callback != nil && finalResponse != "" {
			// 	// s.callback.OnEventResult(s.ctx, EventLLMResponseComplete, finalResponse, func(msgType SessionMessageType, data map[string]any) {
			// 	// 	s.sendJsonMap(msgType, data)
			// 	// })
			// 	// s.callback.OnEvent(EventLLMResponseComplete, finalResponse, func(subMessage []map[string]any) {
			// 	// 	s.bus.Publish(Event{Type: EventUserMessage, Data: subMessage})
			// 	// })
			// 	s.callback.OnEvent(s.eventCtx, event.EventLLMResponseComplete, finalResponse)
			// 	// mock data for citations and thought chain events
			// 	s.callback.OnCitationsEvent(map[string]model.Citations{
			// 		"12345679": {
			// 			Title:   "cite1",
			// 			Number:  1,
			// 			ChunkID: "12345679",
			// 		}, "89454131": {
			// 			Title:   "cite2",
			// 			Number:  2,
			// 			ChunkID: "89454131",
			// 		},
			// 	})
			// 	s.callback.OnThoughtChainEvent(model.ThoughtChain{
			// 		Status: "success",
			// 		Title:  "thought chain title",
			// 		Items: []model.ThoughtItem{
			// 			{
			// 				Title:   "thought1",
			// 				Content: "thought content 1",
			// 			}, {
			// 				Title:   "thought2",
			// 				Content: "thought content 2",
			// 			},
			// 		},
			// 	})
			// }
			// log.Println("Final LLM Response:", finalResponse)

			// if !s.ttsEnabled {
			// 	// close(s.Done)
			// 	log.Println(">> llm close session, tts is not open! ")
			// }
			// close(s.Done)

			// close() // close llm task consumer
			log.Println("llm consumer close!")

			return nil
		}
	}
}
