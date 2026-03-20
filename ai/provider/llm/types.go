package llm

import (
	"context"

	"github.com/gollmdev/asr-llm-tts/ai/dag/dagtypes"
)

// TokenEvent represents an incremental token from the model.
// If Err is non-nil, the stream should end with error.
type TokenEvent struct {
	Delta string
	Err   error
	Done  bool
}

// Provider defines a streaming chat-completion interface.
// Implementations must stream token deltas until Done or context is canceled.
type Provider interface {
	// StreamChat streams completion for the given messages and yields TokenEvent on the returned channel.
	// The channel is closed when streaming ends. Implementations should stop when ctx is done.
	StreamChat(ctx context.Context, model string, messages []dagtypes.Message) (<-chan TokenEvent, error)
}

// Message is a minimal chat message shape.

// type ToolResult struct {
// 	ToolCallID string
// 	Name       string
// 	Content    string
// }

type StreamChatMessage struct {
	Event     string
	Content   *string
	err       error
	usage     *map[string]any
	ToolCalls *map[string]*ToolCallState
}
