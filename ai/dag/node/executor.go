package node

import (
	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
)

type ToolExecutorNode struct{}

func (n *ToolExecutorNode) ID() string { return "tool_executor" }
func (n *ToolExecutorNode) Mode() dag.NodeMode {
	return dag.ModeAlwaysOn
}

func (n *ToolExecutorNode) Run(
	// ctx context.Context,
	rt dag.NodeRuntime,
	// in <-chan dag.Event,
	// out chan<- dag.Event,
) error {
	ctx := rt.Context()
	for {
		select {
		case ev, ok := <-rt.Input():
			if !ok {
				return nil
			}
			_ = ev.Data.(string)

			// result := executeTool(toolCall)

			rt.Emit(&dag.Event{
				Type: "tool_result",
				From: n.ID(),
				Data: "西安的天气是15度",
			})

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
