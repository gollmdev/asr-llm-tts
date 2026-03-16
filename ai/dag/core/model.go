package dag

type RuntimeContext struct {
	sessionID int64
	memory    MemoryStore
	EnableTTS bool
	// Ctx       context.Context
	// Cancel    context.CancelFunc
}
type Event struct {
	Type string
	From string
	Data any
	Rtx  *RuntimeContext
}
