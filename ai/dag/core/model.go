package dag

import "github.com/gollmdev/asr-llm-tts/ai/dag/service"

type RuntimeContext struct {
	UserID    int64
	SessionID int64
	Memory    MemoryStore
	EnableTTS bool
	Output    Emitter
	Services  map[string]any
	Retriever service.Retriever
	// Ctx       context.Context
	// Cancel    context.CancelFunc
}
type Event struct {
	Type string
	From string
	Data any
	Rtx  *RuntimeContext
}

type Emitter interface {
	Emit(*Event)
}
