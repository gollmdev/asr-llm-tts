package node

import (
	"log"

	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
	"github.com/gollmdev/asr-llm-tts/ai/dag/dagtypes"
)

type mergeState struct {
	Context   *dagtypes.TurnContext
	RouteDone bool
	RAGDone   bool
	ToolDone  bool
}

type ContextMergeNode struct {
	// mu    sync.Mutex
	// state map[string]*mergeState
}

// func NewContextMergeNode() *ContextMergeNode {
// 	return &ContextMergeNode{
// 		state: make(map[string]*mergeState),
// 	}
// }

func (n *ContextMergeNode) ID() string { return "context_merge" }
func (n *ContextMergeNode) Mode() dag.NodeMode {
	return dag.ModeLazy
}

func (n *ContextMergeNode) Run(rt dag.NodeRuntime) error {
	state := make(map[string]*mergeState)
	for ev := range rt.Input() {
		tc, ok := ev.Data.(*dagtypes.TurnContext)
		if !ok || tc == nil {
			log.Println("[context_merge] invalid turn context")
			continue
		}
		if tc.TurnID == "" {
			log.Println("[context_merge] empty turn id")
			continue
		}

		// n.mu.Lock()
		ms, exists := state[tc.TurnID]
		if !exists {
			ms = &mergeState{}
			state[tc.TurnID] = ms
		}

		if ms.Context == nil {
			ms.Context = tc
		} else {
			ms.Context = mergeTurnContext(ms.Context, tc)
		}

		switch ev.Type {
		case "route_ready":
			ms.RouteDone = true
			if ms.Context.Route != nil && !ms.Context.Route.UseRAG {
				readyCtx := ms.Context
				delete(state, tc.TurnID)
				// n.mu.Unlock()

				rt.Emit(&dag.Event{
					Type: "context_merged",
					Data: readyCtx,
					Rtx:  ev.Rtx,
				})
				continue
			}

		case "rag_ready":
			ms.RAGDone = true
		case "tool_result_context":
			readyCtx := ms.Context
			delete(state, tc.TurnID)

			rt.Emit(&dag.Event{
				Type: "context_merged",
				Data: readyCtx,
				Rtx:  ev.Rtx,
			})
			continue
		}

		shouldEmit := false
		if ms.Context.Route != nil {
			if ms.Context.Route.UseRAG {
				shouldEmit = ms.RouteDone && ms.RAGDone
			} else {
				shouldEmit = ms.RouteDone
			}
		}

		if shouldEmit {
			readyCtx := ms.Context
			delete(state, tc.TurnID)
			// n.mu.Unlock()

			rt.Emit(&dag.Event{
				Type: "context_merged",
				Data: readyCtx,
				Rtx:  ev.Rtx,
			})
			continue
		}

		// n.mu.Unlock()
	}
	return nil
}

func mergeTurnContext(dst, src *dagtypes.TurnContext) *dagtypes.TurnContext {
	if dst == nil {
		return src
	}
	if src == nil {
		return dst
	}

	if dst.UserInput == nil && src.UserInput != nil {
		dst.UserInput = src.UserInput
	}
	if len(dst.History) == 0 && len(src.History) > 0 {
		dst.History = src.History
	}
	if dst.Summary == "" && src.Summary != "" {
		dst.Summary = src.Summary
	}
	if len(src.LongMemories) > 0 {
		dst.LongMemories = src.LongMemories
	}
	if len(src.Docs) > 0 {
		dst.Docs = src.Docs
	}
	if len(src.ToolResults) > 0 {
		dst.ToolResults = mergeToolResults(dst.ToolResults, src.ToolResults)
	}
	if dst.Route == nil && src.Route != nil {
		dst.Route = src.Route
	}
	if dst.Metadata == nil {
		dst.Metadata = map[string]any{}
	}
	for k, v := range src.Metadata {
		dst.Metadata[k] = v
	}
	return dst
}

func mergeToolResults(dst, src []dagtypes.ToolResult) []dagtypes.ToolResult {
	seen := make(map[string]struct{}, len(dst))
	out := append([]dagtypes.ToolResult(nil), dst...)
	for _, item := range dst {
		seen[item.ToolCallID] = struct{}{}
	}
	for _, item := range src {
		if _, ok := seen[item.ToolCallID]; ok {
			continue
		}
		out = append(out, item)
		seen[item.ToolCallID] = struct{}{}
	}
	return out
}
