package dag

import (
	"context"
	"sync"
)

type NodeRuntime interface {
	Input() <-chan *Event
	Emit(*Event)
	GetState(key string) any
	SetState(key string, value any)
	Context() context.Context
}

type EmitFunc func(*Event)

// type EmitMiddleware func(Event, func(Event))

type EmitMiddleware func(EmitFunc) EmitFunc

func (r *nodeRuntime) Context() context.Context {
	return r.ctx // 直接返回 nodeRuntime 的 ctx
}

type RuntimeContext struct {
	sessionID int64
	memory    MemoryStore
	// Ctx       context.Context
	// Cancel    context.CancelFunc
}
type nodeRuntime struct {
	nodeID string
	// middlewares []EmitMiddleware
	ctx   context.Context
	rtx   *RuntimeContext
	input <-chan *Event
	emit  func(*Event)
	mu    sync.RWMutex
	state map[string]any
}

// func NewNodeRuntime(nodeID string, middlewares []EmitMiddleware, rtx *RuntimeContext, input <-chan *Event, emit func(*Event)) *nodeRuntime {
// 	rt := &nodeRuntime{
// 		nodeID: nodeID,
// 		// sessionID:   sessionID,
// 		// middlewares: middlewares,
// 		rtx:   rtx,
// 		input: input,
// 		state: make(map[string]any),
// 	}

//		rt.emit = rt.buildEmitChain(emit)
//		return rt
//	}
func (r *nodeRuntime) Input() <-chan *Event {
	return r.input
}

func (r *nodeRuntime) Emit(ev *Event) {
	ev.From = r.nodeID
	ev.SessionID = r.rtx.sessionID
	r.emit(ev)
}

// 中间件洋葱模型构建方式

// func (r *nodeRuntime) buildEmitChain(ev Event, final EmitFunc) EmitFunc {
// 	wrapped := final

// 	// 倒序构建洋葱模型
// 	for i := len(r.middlewares) - 1; i >= 0; i-- {
// 		next := wrapped
// 		mw := r.middlewares[i]
// 		wrapped = mw(ev, next)
// 	}

// 	return wrapped
// }

func (r *nodeRuntime) GetState(key string) any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state[key]
}

func (r *nodeRuntime) SetState(key string, value any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state[key] = value
}
