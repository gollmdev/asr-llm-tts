package dag

type RuntimeContext struct {
	sessionID int64
	memory    MemoryStore
	EnableTTS bool
	Output    Emitter
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
