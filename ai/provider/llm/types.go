package llm

import (
	"context"
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
	StreamChat(ctx context.Context, model string, messages []Message) (<-chan TokenEvent, error)
}

// Message is a minimal chat message shape.
type Message struct {
	Role    string // system | user | assistant
	Content string
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type ToolResult struct {
	ToolCallID string
	Name       string
	Content    string
}

type StreamChatMessage struct {
	Event     string
	Content   *string
	err       error
	usage     *map[string]any
	ToolCalls *map[string]*ToolCallState
}
