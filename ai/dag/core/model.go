package dag

type RuntimeContext struct {
	UserID    int64
	SessionID int64
	Memory    MemoryStore
	EnableTTS bool
	Output    Emitter
	Services  map[string]any
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
