package node

import (
	"encoding/json"
	"fmt"
	"sync"

	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
	"github.com/gollmdev/asr-llm-tts/ai/dag/dagtypes"
)

type ToolFunc func(arguments string) (string, error)

type ToolExecutorNode struct {
	mu       sync.RWMutex
	toolFunc map[string]ToolFunc
}

func NewToolExecutorNode() *ToolExecutorNode {
	return &ToolExecutorNode{
		toolFunc: make(map[string]ToolFunc),
	}
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
	return dag.ModeLazy
}
func (n *ToolExecutorNode) ClosePolicy() dag.NodeClosePolicy {
	return dag.AggregateClosePolicy{
		Required: dag.Any(dag.HasEvent("llm_complete")),
	}
}
func (n *ToolExecutorNode) Run(
	// ctx context.Context,
	rt dag.NodeRuntime,
	// in <-chan dag.Event,
	// out chan<- dag.Event,
) error {
	ctx := rt.Context()
	sessionId := rt.RuntimeContext().SessionID
	userId := rt.RuntimeContext().UserID
	for {
		select {
		case ev, ok := <-rt.Input():
			if !ok {
				return nil
				// continue
			}
			payload, ok := ev.Data.(*dagtypes.ToolCallPayload)
			if !ok || payload == nil || payload.Context == nil {
				continue
			}

			results := make([]dagtypes.ToolResult, 0, len(payload.ToolCalls))
			for _, toolCall := range payload.ToolCalls {
				toolCall.SessionID = sessionId
				toolCall.UserId = userId

				results = append(results, n.executeTool(toolCall))
			}
			updatedCtx := dagtypes.CloneTurnContext(payload.Context)
			updatedCtx.ToolResults = append(updatedCtx.ToolResults, results...)

			if updatedCtx.Metadata == nil {
				updatedCtx.Metadata = map[string]any{}
			}
			updatedCtx.Metadata["tool_call_count"] = len(payload.ToolCalls)

			// 防止重复执行同一轮对话中的工具调用，增加调用次数统计
			toolLoopCount, _ := updatedCtx.Metadata["tool_loop_count"].(int)
			updatedCtx.Metadata["tool_loop_count"] = toolLoopCount + 1
			rt.Emit(&dag.Event{
				Type: "tool_result_context",
				Data: updatedCtx,
			})

			// rt.Emit(&dag.Event{
			// 	Type: "tool_result",
			// 	Data: results,
			// })

		case <-ctx.Done():
			return ctx.Err()
		}
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
func (n *ToolExecutorNode) executeTool(toolCall dagtypes.ToolCall) dagtypes.ToolResult {
	enrichedArgs := injectSessionAndUserArgs(toolCall.Arguments, toolCall.SessionID, toolCall.UserId)
	toolCall.Arguments = enrichedArgs

	result := dagtypes.ToolResult{
		ToolCallID: toolCall.ID,
		ToolName:   toolCall.Name,
		Args:       enrichedArgs,
	}

	if fn := n.registeredTool(toolCall.Name); fn != nil {
		content, err := fn(enrichedArgs)
		if err != nil {
			result.Result = fmt.Sprintf("tool %s execute failed: %v", toolCall.Name, err)
			return result
		}
		result.Result = content
		return result
	}

	result.Result = fmt.Sprintf("tool %s is not implemented", toolCall.Name)

	return result
}

func injectSessionAndUserArgs(arguments string, sessionId, userId int64) string {
	var obj map[string]any
	if err := json.Unmarshal([]byte(arguments), &obj); err == nil && obj != nil {
		obj["sessionId"] = sessionId
		obj["userId"] = userId
		obj["_systemUserId"] = userId
		if b, err := json.Marshal(obj); err == nil {
			return string(b)
		}
	}

	wrapped := map[string]any{
		"sessionId":     sessionId,
		"userId":        userId,
		"_systemUserId": userId,
	}
	if arguments != "" {
		wrapped["_rawArguments"] = arguments
	}
	if b, err := json.Marshal(wrapped); err == nil {
		return string(b)
	}

	return arguments
}
