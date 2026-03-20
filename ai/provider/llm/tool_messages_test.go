package llm

import (
	"strings"
	"testing"

	"github.com/gollmdev/asr-llm-tts/ai/dag/dagtypes"
)

func TestNormalizeToolCallsAndBuildMessages(t *testing.T) {
	toolCalls := map[string]*ToolCallState{
		"call_2": {
			Name: "get_weather",
		},
		"call_1": {
			Name: "get_weather",
		},
	}
	toolCalls["call_1"].Arguments.WriteString(`{"location":"杭州"}`)
	toolCalls["call_2"].Arguments.WriteString(`{"location":"西安"}`)

	normalized := NormalizeToolCalls(&toolCalls)
	if len(normalized) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(normalized))
	}
	if normalized[0].ID != "call_1" {
		t.Fatalf("expected sorted tool calls, got first id %q", normalized[0].ID)
	}

	assistantMessage := BuildAssistantToolCallMessage(normalized)
	entries, ok := assistantMessage["tool_calls"].([]map[string]any)
	if !ok {
		t.Fatalf("tool_calls payload has unexpected type: %T", assistantMessage["tool_calls"])
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 tool call entries, got %d", len(entries))
	}

	toolMessages := BuildToolResultMessages([]dagtypes.ToolResult{{
		ToolCallID: "call_1",
		ToolName:   "get_weather",
		Result:     "杭州当前天气晴，气温15度。",
	}})
	if len(toolMessages) != 1 {
		t.Fatalf("expected 1 tool result message, got %d", len(toolMessages))
	}
	if !strings.Contains(toolMessages[0]["content"].(string), "杭州") {
		t.Fatalf("unexpected tool result content: %v", toolMessages[0]["content"])
	}
}
