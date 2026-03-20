package node

import (
	"context"
	"errors"
	"io"
	"log"

	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
	"github.com/gollmdev/asr-llm-tts/ai/dag/dagtypes"
	"github.com/gollmdev/asr-llm-tts/ai/provider/llm"
	"golang.org/x/sync/errgroup"
)

type LLMLoopNode struct {
	Tools []map[string]any
}

func (n *LLMLoopNode) ID() string { return "llm" }
func (n *LLMLoopNode) Mode() dag.NodeMode {
	return dag.ModeLazy
}

func (n *LLMLoopNode) Run(
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
		case message, ok := <-rt.Input():
			if !ok {
				log.Println("TTSStream completed")
				return nil
			}
			history, err := n.buildInitialMessages(message)
			if err != nil {
				return err
			}

			if err := n.runToolLoop(rt, ctx, g, tools, history); err != nil {
				if !errors.Is(err, context.Canceled) {
					log.Println("LLMStream error:", err)
				}
				return err
			}

			log.Println("llm consumer close!")

			return nil
		}
	}
}

func (n *LLMLoopNode) buildInitialMessages(message *dag.Event) ([]map[string]any, error) {
	if message == nil {
		return nil, errors.New("llm input is nil")
	}

	content, ok := message.Data.(string)
	if !ok {
		return nil, errors.New("llm input must be string")
	}

	switch message.Type {
	case "text", "asr_text":
		return []map[string]any{{"role": "user", "content": content}}, nil
	default:
		return nil, errors.New("unsupported llm input event: " + message.Type)
	}
}

func (n *LLMLoopNode) runToolLoop(
	rt dag.NodeRuntime,
	ctx context.Context,
	g *errgroup.Group,
	tools []map[string]any,
	history []map[string]any,
) error {
	for {
		stream := llm.NewQwenChatModel(&llm.ChatModelConfig{
			Model: "qwen-plus",
			Tools: tools,
			Ctx:   ctx,
			G:     g,
		})

		stream.Stream(history)

		var pendingToolCalls []dagtypes.ToolCall
		for {
			msg, err := stream.Recv()
			if err != nil {
				if err != io.EOF {
					return err
				}
				break
			}
			if msg == nil {
				continue
			}

			switch msg.Event {
			case "OnText":
				if msg.Content == nil {
					continue
				}
				rt.Emit(&dag.Event{
					Type: "llm_chunk",
					Data: *msg.Content,
				})
			case "OnToolCallFinish":
				pendingToolCalls = llm.NormalizeToolCalls(msg.ToolCalls)
				if len(pendingToolCalls) == 0 {
					continue
				}
				history = append(history, llm.BuildAssistantToolCallMessage(pendingToolCalls))
				rt.Emit(&dag.Event{
					Type: "llm_tool_call",
					Data: pendingToolCalls,
				})
			}
		}

		if len(pendingToolCalls) == 0 {
			return nil
		}

		toolResults, err := n.waitToolResults(ctx, rt.Input())
		if err != nil {
			return err
		}
		if len(toolResults) == 0 {
			return errors.New("tool executor returned empty results")
		}

		history = append(history, llm.BuildToolResultMessages(toolResults)...)
	}
}

func (n *LLMLoopNode) waitToolResults(ctx context.Context, input <-chan *dag.Event) ([]dagtypes.ToolResult, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case ev, ok := <-input:
			if !ok {
				return nil, io.EOF
			}
			if ev == nil || ev.Type != "tool_result" {
				continue
			}
			results, ok := ev.Data.([]dagtypes.ToolResult)
			if !ok {
				return nil, errors.New("tool_result payload is invalid")
			}
			return results, nil
		}
	}
}
