package node

import (
	"fmt"
	"strings"

	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
	"github.com/gollmdev/asr-llm-tts/ai/dag/dagtypes"
)

type PromptAssemblyNode struct{}

func (n *PromptAssemblyNode) ID() string { return "prompt_assembly" }
func (n *PromptAssemblyNode) Mode() dag.NodeMode {
	return dag.ModeLazy
}
func (n *PromptAssemblyNode) ClosePolicy() dag.NodeClosePolicy {
	return dag.AggregateClosePolicy{
		Required: dag.Any(dag.HasEvent("llm_complete")),
	}
}
func (n *PromptAssemblyNode) Run(rt dag.NodeRuntime) error {
	ctx := rt.Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-rt.Input():
			if !ok {
				return nil
			}
			tc, ok := ev.Data.(*dagtypes.TurnContext)
			if !ok || tc == nil || tc.UserInput == nil {
				continue
			}
			if c, ok := tc.Metadata["tool_loop_count"].(int); ok && c >= 4 {
				// 禁止继续 tool call，改成直接回答
			}
			var messages []*dagtypes.Message

			systemPrompt := buildSystemPrompt(tc)
			messages = append(messages, &dagtypes.Message{
				Role:    "system",
				Content: systemPrompt,
			})

			if tc.Summary != "" {
				messages = append(messages, &dagtypes.Message{
					Role:    "system",
					Content: "对话摘要：\n" + tc.Summary,
				})
			}

			if len(tc.LongMemories) > 0 {
				messages = append(messages, &dagtypes.Message{
					Role:    "system",
					Content: "长期记忆：\n" + formatLongMemories(tc.LongMemories),
				})
			}

			if len(tc.Docs) > 0 {
				messages = append(messages, &dagtypes.Message{
					Role:    "system",
					Content: "参考资料：\n" + formatDocs(tc.Docs),
				})
			}

			if len(tc.ToolResults) > 0 {
				messages = append(messages, &dagtypes.Message{
					Role:    "system",
					Content: "工具结果：\n" + formatToolResults(tc.ToolResults),
				})
			}

			if len(tc.History) > 0 {
				messages = append(messages, tc.History...)
			} else {
				messages = append(messages, tc.UserInput)
			}

			payload := &dagtypes.PromptPayload{
				Context:  tc,
				Messages: messages,
			}

			rt.Emit(&dag.Event{
				Type: "prompt_ready",
				Data: payload,
				Rtx:  ev.Rtx,
			})
		}
	}

	return nil
}

func buildSystemPrompt(tc *dagtypes.TurnContext) string {
	var sb strings.Builder
	sb.WriteString("你是一个有帮助的智能助手。\n")
	sb.WriteString("回答要求：\n")
	sb.WriteString("1. 优先依据提供的参考资料回答。\n")
	sb.WriteString("2. 如果参考资料不足，明确说明不确定，不要编造。\n")
	sb.WriteString("3. 回答简洁、准确、结构清晰。\n")

	if tc.Route != nil && tc.Route.UseTools {
		sb.WriteString("4. 允许在必要时发起工具调用。\n")
	}
	sb.WriteString("请在回答结束时提出用户可能感兴趣的后续问题!")
	return sb.String()
}

func formatDocs(docs []dagtypes.RetrievedDoc) string {
	var sb strings.Builder
	for i, d := range docs {
		sb.WriteString(fmt.Sprintf("[%d] %s\n%s\n\n", i+1, d.Title, d.Content))
	}
	return sb.String()
}

func formatLongMemories(items []dagtypes.MemoryItem) string {
	var sb strings.Builder
	for i, it := range items {
		sb.WriteString(fmt.Sprintf("[%d] %s\n", i+1, it.Content))
	}
	return sb.String()
}

func formatToolResults(results []dagtypes.ToolResult) string {
	var sb strings.Builder
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("[%d] tool=%s success=%v\nargs=%s\nresult=%s\n\n",
			i+1, r.ToolName, r.Success, r.Args, r.Result))
	}
	return sb.String()
}
