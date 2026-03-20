package llm

import (
	"sort"

	"github.com/gollmdev/asr-llm-tts/ai/dag/dagtypes"
)

func NormalizeToolCalls(toolCalls *map[string]*ToolCallState) []dagtypes.ToolCall {
	if toolCalls == nil || len(*toolCalls) == 0 {
		return nil
	}

	ids := make([]string, 0, len(*toolCalls))
	for id := range *toolCalls {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	result := make([]dagtypes.ToolCall, 0, len(ids))
	for _, id := range ids {
		call := (*toolCalls)[id]
		if call == nil {
			continue
		}
		result = append(result, dagtypes.ToolCall{
			ID:        id,
			Name:      call.Name,
			Arguments: call.Arguments.String(),
		})
	}

	return result
}

func BuildAssistantToolCallMessage(toolCalls []dagtypes.ToolCall) map[string]any {
	entries := make([]map[string]any, 0, len(toolCalls))
	for index, toolCall := range toolCalls {
		entries = append(entries, map[string]any{
			"id":    toolCall.ID,
			"type":  "function",
			"index": index,
			"function": map[string]string{
				"name":      toolCall.Name,
				"arguments": toolCall.Arguments,
			},
		})
	}

	return map[string]any{
		"role":       "assistant",
		"content":    "",
		"tool_calls": entries,
	}
}

func BuildToolResultMessages(results []dagtypes.ToolResult) []map[string]any {
	messages := make([]map[string]any, 0, len(results))
	for _, result := range results {
		messages = append(messages, map[string]any{
			"role":         "tool",
			"name":         result.ToolName,
			"content":      result.Result,
			"tool_call_id": result.ToolCallID,
		})
	}

	return messages
}
