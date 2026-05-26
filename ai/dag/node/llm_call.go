package node

import (
	"log"

	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
	"github.com/gollmdev/asr-llm-tts/ai/dag/dagtypes"
	"github.com/gollmdev/asr-llm-tts/ai/provider/llm"
	"golang.org/x/sync/errgroup"
)

type LLMCallNode struct {
	Tools        []map[string]any
	ResponseJson bool
}

func (n *LLMCallNode) ID() string { return "llm_call" }
func (n *LLMCallNode) Mode() dag.NodeMode {
	return dag.ModeLazy
}

func (n *LLMCallNode) Run(
	rt dag.NodeRuntime,
) error {
	g, ctx := errgroup.WithContext(rt.Context())
	tools := []map[string]any{}
	if n.Tools != nil {
		tools = n.Tools
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-rt.Input():

			if !ok {
				log.Println("LLM stream completed")
				return nil
			}

			llmModel := llm.NewQwenChatModel(&llm.ChatModelConfig{
				Model:        "qwen-plus",
				Tools:        tools,
				Ctx:          ctx,
				G:            g,
				ResponseJson: n.ResponseJson,
			})

			payload, ok := ev.Data.(*dagtypes.PromptPayload)
			if !ok || payload == nil {
				log.Println("[chat] invalid prompt payload")
				continue
			}
			msg, err := llmModel.Generate(dagtypes.ConvertMessages(payload.Messages))
			if err != nil {
				return err
			}
			// if n.eventType == "" {
			// 	n.eventType = "llm_complete"
			// }

			// if ev.Ctx != nil && ev.Ctx["event_type"] != nil {
			// 	n.eventType = ev.Ctx["event_type"].(string)
			// } else if n.eventType == "" {
			// 	n.eventType = "llm_complete"
			// }

			rt.Emit(&dag.Event{
				Type: ev.Type,
				Data: msg,
				Ctx:  ev.Ctx,
			})

		}
	}
	// rt.Emit(&dag.Event{
	// 	Type: "llm_complete",
	// 	From: n.ID(),
	// })
	// return nil
}
