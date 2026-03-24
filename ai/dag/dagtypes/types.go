package dagtypes

type MemoryItem struct {
	ID      string
	Content string
	Score   float64
	Source  string
}
type Message struct {
	Role       string     `json:"role"` // system | user | assistant | tool
	Content    string     `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"` // tool name
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}
type RetrievedDoc struct {
	ID       string
	Content  string
	Score    float64
	Source   string
	Title    string
	Metadata map[string]any
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
	SessionID int64
	UserId    int64
}

type ToolResult struct {
	ToolCallID string
	ToolName   string
	Args       string
	Result     string
	Success    bool
}

type RouteDecision struct {
	Intent      string
	UseRAG      bool
	UseTools    bool
	DirectReply bool
	NeedTTS     bool
	Reason      string
}

type UserTurnInput struct {
	Text     string
	Modality string // text / asr
}

type TurnContext struct {
	SessionID    int64
	TurnID       string
	UserInput    *Message
	History      []*Message
	Summary      string
	LongMemories []MemoryItem
	Docs         []RetrievedDoc
	ToolResults  []ToolResult
	Route        *RouteDecision
	Metadata     map[string]any
}

type PromptPayload struct {
	Context  *TurnContext
	Messages []*Message
}

type RetrievalRequest struct {
	SessionID int64
	TurnID    string
	Query     string
	TopK      int
	Context   *TurnContext
}

type ToolCallPayload struct {
	Context   *TurnContext
	Messages  []*Message
	ToolCalls []ToolCall
}

func ConvertMessages(msgs []*Message) []map[string]any {
	var res []map[string]any
	for _, m := range msgs {
		item := map[string]any{
			"role": m.Role,
		}
		if m.Content != "" {
			item["content"] = m.Content
		}
		if m.ToolCalls != nil {
			item["tool_calls"] = m.ToolCalls
		}
		if m.ToolCallID != "" {
			item["tool_call_id"] = m.ToolCallID
		}
		if m.Name != "" {
			item["name"] = m.Name
		}
		res = append(res, item)
	}
	return res
}

func CloneTurnContext(src *TurnContext) *TurnContext {
	if src == nil {
		return nil
	}

	dst := &TurnContext{
		SessionID:    src.SessionID,
		TurnID:       src.TurnID,
		UserInput:    src.UserInput,
		Summary:      src.Summary,
		Route:        src.Route,
		Metadata:     copyMap(src.Metadata),
		LongMemories: append([]MemoryItem(nil), src.LongMemories...),
		Docs:         append([]RetrievedDoc(nil), src.Docs...),
		ToolResults:  append([]ToolResult(nil), src.ToolResults...),
	}

	if len(src.History) > 0 {
		dst.History = append([]*Message(nil), src.History...)
	}

	return dst
}

func copyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
