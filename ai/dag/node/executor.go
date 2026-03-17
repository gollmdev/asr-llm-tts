package node

import (
	"encoding/json"
	"fmt"
	"sync"

	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
	"github.com/gollmdev/asr-llm-tts/ai/provider/llm"
)

type ToolFunc func(arguments string) (string, error)

type ToolExecutorNode struct {
	mu       sync.RWMutex
	toolFunc map[string]ToolFunc
}

func BuiltinToolRegistry() map[string]ToolFunc {
	return map[string]ToolFunc{
		"get_weather": GetWeatherTool,
	}
}

func NewToolExecutorNode() *ToolExecutorNode {
	return &ToolExecutorNode{
		toolFunc: make(map[string]ToolFunc),
	}
}

func (n *ToolExecutorNode) RegisterTool(name string, fn ToolFunc) {
	if name == "" || fn == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.toolFunc[name] = fn
}

// RegisterToolsFromDefinitions auto-binds tool handlers by matching tool definition
// function names with handlers from the provided registry.
func (n *ToolExecutorNode) RegisterToolsFromDefinitions(definitions []map[string]any, registry map[string]ToolFunc) []string {
	missing := make([]string, 0)
	for _, definition := range definitions {
		name := toolNameFromDefinition(definition)
		if name == "" {
			continue
		}
		fn := registry[name]
		if fn == nil {
			missing = append(missing, name)
			continue
		}
		n.RegisterTool(name, fn)
	}
	return missing
}

func toolNameFromDefinition(definition map[string]any) string {
	fn, ok := definition["function"].(map[string]any)
	if !ok {
		return ""
	}
	name, _ := fn["name"].(string)
	return name
}

func (n *ToolExecutorNode) registeredTool(name string) ToolFunc {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.toolFunc[name]
}

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
			toolCalls, ok := ev.Data.([]llm.ToolCall)
			if !ok {
				continue
			}

			results := make([]llm.ToolResult, 0, len(toolCalls))
			for _, toolCall := range toolCalls {
				results = append(results, n.executeTool(toolCall))
			}

			rt.Emit(&dag.Event{
				Type: "tool_result",
				Data: results,
			})

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (n *ToolExecutorNode) executeTool(toolCall llm.ToolCall) llm.ToolResult {
	result := llm.ToolResult{
		ToolCallID: toolCall.ID,
		Name:       toolCall.Name,
	}

	if fn := n.registeredTool(toolCall.Name); fn != nil {
		content, err := fn(toolCall.Arguments)
		if err != nil {
			result.Content = fmt.Sprintf("tool %s execute failed: %v", toolCall.Name, err)
			return result
		}
		result.Content = content
		return result
	}

	result.Content = fmt.Sprintf("tool %s is not implemented", toolCall.Name)

	return result
}

func GetWeatherTool(arguments string) (string, error) {
	type weatherArgs struct {
		Location string `json:"location"`
	}

	var args weatherArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("无法解析天气查询参数: %w", err)
	}
	if args.Location == "" {
		return "", fmt.Errorf("缺少 location 参数")
	}

	return fmt.Sprintf("%s当前天气晴，气温15度。", args.Location), nil
}
