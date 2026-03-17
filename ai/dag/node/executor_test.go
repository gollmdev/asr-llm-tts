package node

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gollmdev/asr-llm-tts/ai/provider/llm"
)

func TestExecuteToolGetWeather(t *testing.T) {
	executor := NewToolExecutorNode()
	executor.RegisterTool("get_weather", GetWeatherTool)

	result := executor.executeTool(llm.ToolCall{
		ID:        "call_1",
		Name:      "get_weather",
		Arguments: `{"location":"西安"}`,
	})

	if result.ToolCallID != "call_1" {
		t.Fatalf("unexpected tool call id: %q", result.ToolCallID)
	}
	if result.Name != "get_weather" {
		t.Fatalf("unexpected tool name: %q", result.Name)
	}
	if !strings.Contains(result.Content, "西安") {
		t.Fatalf("unexpected tool result content: %q", result.Content)
	}
}

func TestExecuteToolUnknown(t *testing.T) {
	executor := NewToolExecutorNode()
	result := executor.executeTool(llm.ToolCall{Name: "unknown_tool"})
	if !strings.Contains(result.Content, "not implemented") {
		t.Fatalf("unexpected fallback content: %q", result.Content)
	}
}

func TestRegisterToolsFromDefinitions(t *testing.T) {
	executor := NewToolExecutorNode()
	definitions := []map[string]any{
		{
			"type": "function",
			"function": map[string]any{
				"name": "echo",
			},
		},
	}

	missing := executor.RegisterToolsFromDefinitions(definitions, map[string]ToolFunc{
		"echo": func(arguments string) (string, error) {
			return fmt.Sprintf("echo:%s", arguments), nil
		},
	})

	if len(missing) != 0 {
		t.Fatalf("expected no missing tools, got %v", missing)
	}

	result := executor.executeTool(llm.ToolCall{
		ID:        "call_2",
		Name:      "echo",
		Arguments: "hello",
	})
	if result.Content != "echo:hello" {
		t.Fatalf("unexpected execute content: %q", result.Content)
	}
}

func TestRegisterToolsFromDefinitionsMissing(t *testing.T) {
	executor := NewToolExecutorNode()
	definitions := []map[string]any{
		{
			"type": "function",
			"function": map[string]any{
				"name": "unknown_tool",
			},
		},
	}

	missing := executor.RegisterToolsFromDefinitions(definitions, BuiltinToolRegistry())
	if len(missing) != 1 || missing[0] != "unknown_tool" {
		t.Fatalf("unexpected missing tools: %v", missing)
	}
}
