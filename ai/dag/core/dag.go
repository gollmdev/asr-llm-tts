package dag

import (
	"context"
	"log"
	"sync"

	"golang.org/x/sync/errgroup"
)

type NodeMode int

const (
	ModeLazy     NodeMode = iota // 按需启动
	ModeAlwaysOn                 // 预启动
)

type Node interface {
	ID() string
	// InputTypes() []string
	// OutputTypes() []string
	Mode() NodeMode

	// Run(ctx context.Context, in <-chan Event, out chan<- Event)
	Run(rt NodeRuntime) error
}
type NodeState struct {
	mu      sync.Mutex
	started bool
	// upstreams int
	// activeUpstreams int
	done      bool
	upstreams map[string]struct{}

	upstreamActive map[string]bool
	receivedEvents map[string]struct{}
	pendingEvents  []*Event
}

type ConditionFunc func(*Event) bool

type Edge struct {
	FromNode string
	OnEvent  string
	ToNode   string
	Cond     ConditionFunc // ⭐ 条件判断

}

type DAG struct {
	Nodes      map[string]Node
	Edges      []Edge
	downstream map[string][]string
	upstreams  map[string]map[string]struct{}
	router     *EventRouter
}

func NewDAG(nodes map[string]Node, edges []Edge) *DAG {
	upstreams, downstream := buildGraph(edges)
	router := NewEventRouter(edges)
	return &DAG{
		Nodes:      nodes,
		Edges:      edges,
		upstreams:  upstreams,
		downstream: downstream,
		router:     router,
	}
}
func buildGraph(edges []Edge) (
	map[string]map[string]struct{},
	map[string][]string,
) {

	upstreams := map[string]map[string]struct{}{}
	downstream := map[string][]string{}

	for _, e := range edges {

		if upstreams[e.ToNode] == nil {
			upstreams[e.ToNode] = map[string]struct{}{}
		}

		upstreams[e.ToNode][e.FromNode] = struct{}{}

		downstream[e.FromNode] =
			append(downstream[e.FromNode], e.ToNode)
	}

	return upstreams, downstream
}

type Engine struct {
	dag        *DAG
	nodeInput  map[string]chan *Event
	nodeStates map[string]*NodeState
	startRules map[string]NodeStartPolicy
	closeRules map[string]NodeClosePolicy

	bus    chan *Event
	ctx    context.Context
	cancel context.CancelFunc
	g      *errgroup.Group
	// nodeG    *errgroup.Group
	// nodeCtx  context.Context
	waitOnce sync.Once
	wg       sync.WaitGroup

	// sessionID   string
	rtx             *RuntimeContext
	middlewares     []EmitMiddleware
	isFirstDispatch bool
	OnDAGDone       func()
	downstream      map[string][]string
	router          *EventRouter
	// Output chan Event // TUDO

}

// func buildDownstream(edges []Edge) map[string][]string {

// 	m := map[string][]string{}

// 	for _, e := range edges {
// 		m[e.FromNode] = append(m[e.FromNode], e.ToNode)
// 	}

//		return m
//	}

func NewEngine(ctx context.Context, dag *DAG, rtx *RuntimeContext) *Engine {
	ctx, cancel := context.WithCancel(ctx)

	g, ctx := errgroup.WithContext(ctx)

	// nodeG, nodeCtx := errgroup.WithContext(ctx)
	e := &Engine{
		dag:             dag,
		nodeInput:       make(map[string]chan *Event),
		nodeStates:      make(map[string]*NodeState),
		startRules:      make(map[string]NodeStartPolicy),
		closeRules:      make(map[string]NodeClosePolicy),
		bus:             make(chan *Event, 64),
		ctx:             ctx,
		cancel:          cancel,
		g:               g,
		rtx:             rtx,
		middlewares:     make([]EmitMiddleware, 0),
		isFirstDispatch: true,
		router:          dag.router,
		downstream:      dag.downstream,
		// nodeG:      nodeG,
		// nodeCtx:    nodeCtx,
	}

	// upstreamCount := make(map[string]int)
	// for _, edge := range dag.Edges {
	// 	upstreamCount[edge.ToNode]++
	// }
	// upstreams, downstream := buildGraph(dag.Edges)
	upstreams := dag.upstreams
	// downstream := dag.downstream
	for id := range dag.Nodes {
		e.nodeInput[id] = make(chan *Event, 32)
		e.nodeStates[id] = &NodeState{
			upstreamActive: make(map[string]bool),
			receivedEvents: make(map[string]struct{}),
			pendingEvents:  make([]*Event, 0),
			upstreams:      upstreams[id],
			// upstreams: upstreamCount[id],
		}
		if provider, ok := dag.Nodes[id].(NodeStartPolicyProvider); ok {
			e.startRules[id] = provider.StartPolicy()
		}
		if provider, ok := dag.Nodes[id].(NodeClosePolicyProvider); ok {
			e.closeRules[id] = provider.ClosePolicy()
		}
	}
	// e.router = NewEventRouter(dag.Edges)
	// e.downstream = downstream
	// e.downstream = buildDownstream(dag.Edges)
	return e
}

func (e *Engine) Start() {
	// g, ctx := errgroup.WithContext(e.ctx)
	e.wg.Add(1)
	log.Printf(">>>node: %d start +1", e.rtx.SessionID)

	// ⭐ 预启动 AlwaysOn 节点
	for id, node := range e.dag.Nodes {
		if node.Mode() == ModeAlwaysOn {
			e.startNode(id)
		}
	}

	// 启动 dispatcher
	e.g.Go(func() error {
		defer func() {
			log.Printf(">>>node: %d dispatcher loop stopped!", e.rtx.SessionID)
		}()
		return e.dispatchLoop()
	})
	e.g.Go(func() error {
		defer log.Printf(">>>node: %d Engine stopped!", e.rtx.SessionID)

		e.wg.Wait()
		// e.cancel() // 任何节点出错或完成都取消整个引擎
		close(e.bus)
		if e.OnDAGDone != nil {
			e.OnDAGDone()
		}

		// delete nodeInput nodeState
		for id := range e.nodeInput {
			// close(e.nodeInput[id])
			delete(e.nodeInput, id)
			// delete(e.nodeStates, id)
		}
		for id := range e.nodeStates {
			delete(e.nodeStates, id)
		}

		// return err
		return nil
	})
}
func (e *Engine) Close() {
	if err := e.g.Wait(); err != nil {
		log.Println("close error:", err)
	}
	e.cancel()
}

func (e *Engine) dispatchLoop() error {

	for ev := range e.bus {
		e.dispatch(ev)
	}
	return nil
}
func (e *Engine) emit(ev *Event) {
	e.bus <- ev
}
func (e *Engine) startNode(id string) {
	// toNode of targets: tts -> output

	state, ok := e.nodeStates[id]
	if !ok {
		log.Printf("%s is not a valid node", id)
		return
	}

	// state.mu.Lock()
	if state.started {
		// state.mu.Unlock()
		return
	}
	state.started = true

	targets := e.downstream[id]
	for _, target := range targets {
		state := e.nodeStates[target]
		if !state.upstreamActive[id] {
			// state.mu.Lock()
			// state.activeUpstreams++
			state.upstreamActive[id] = true
			// state.mu.Unlock()

		}

	}
	// state.mu.Unlock()

	// rt := NewNodeRuntime(id, e.middlewares, e.rtx, input, e.emit)

	// 	nodeID:    id,
	// 	sessionID: e.sessionID,
	// 	input:     input,
	// 	emit:      e.emit, // 绑定 engine 的 emit
	// 	state:     make(map[string]any),
	// }

	// state.activeUpstreams++
	// state.started = true
	// // e.wg.Add(1)
	// state.activeUpstreams++
	if !e.isFirstDispatch {
		e.wg.Add(1)
		log.Printf(">>>node: %d  %s +1", e.rtx.SessionID, id)
	} else {
		e.isFirstDispatch = false
		log.Printf(">>>node: %d  %s use start +1", e.rtx.SessionID, id)
	}

	e.g.Go(func() error {
		log.Printf(">>>> start node  %s", id)
		defer log.Printf(">>>> end node %s", id)

		// defer e.wg.Done()
		// state.mu.Lock()
		// state.started = true
		// // state.activeUpstreams++
		// state.mu.Unlock()

		// e.bus <- Event{
		// 	Type: "node_start",
		// 	From: id,
		// 	Data: nil,
		// }
		rt := e.newNodeRuntime(id)
		err := e.dag.Nodes[id].Run(
			rt,
			// e.nodeInput[id],
			// e.bus,
		)
		// close(e.nodeInput[id])

		// e.onNodeExit(id)

		state.mu.Lock()
		state.done = true
		state.mu.Unlock()

		// e.bus <- &Event{
		// 	Type: "node_done",
		// 	From: id,
		// 	Data: nil,
		// }
		e.emit(&Event{
			Type: "node_done",
			From: id,
		})
		return err
	})

}

func (e *Engine) newNodeRuntime(id string) *nodeRuntime {
	input := e.nodeInput[id]
	// nodeCtx, nodeCancel := context.WithCancel(e.ctx)
	rt := &nodeRuntime{
		nodeID: id,
		// middlewares: e.middlewares,
		rtx:   e.rtx,
		input: input,
		state: make(map[string]any),
		ctx:   e.ctx, // engine context
		g:     e.g,   // engine errgroup
		// Cancel: e.cancel,
	}
	rt.emit = e.buildEmitChain(e.emit)
	return rt
	// rt := &nodeRuntime{
}

func (e *Engine) buildEmitChain(final EmitFunc) EmitFunc {
	wrapped := final

	// 倒序构建洋葱模型
	for i := len(e.middlewares) - 1; i >= 0; i-- {
		mw := e.middlewares[i]
		// wrapped = func(ev Event) {
		// 	mw(ev, next)
		// }
		wrapped = mw(wrapped)
	}

	return wrapped
}
func (e *Engine) Use(mw EmitMiddleware) {
	e.middlewares = append(e.middlewares, mw)
}

// func (e *Engine) onNodeExit(id string) {
// 	state := e.nodeStates[id]

// state.mu.Lock()
// state.done = true
// state.mu.Unlock()

// 	delete(e.nodeStates, id)
// 	delete(e.nodeInput, id)
// 	// e.handleUpstreamDone(id)

// 	log.Printf("onNodeExit close %s", id)

// }
func (e *Engine) isNode(id string) bool {
	_, ok := e.nodeStates[id]
	return ok
}

// dispatch 在 goroutine 中
// startNode 是动态调用
// 某个节点 e.wg.Done() 后
// 可能 dispatch 又 startNode
// 导致 wg.Wait() 提前返回或语义失效
func (e *Engine) dispatch(ev *Event) {
	log.Printf("dispatch %s %s", ev.From, ev.Type)

	targets := e.router.Route(ev)
	if ev.From == "asr" {
		log.Printf(">>>asr event: %s, targets: %v", ev.Type, targets)
	}
	for _, target := range targets {
		node := e.dag.Nodes[target]
		state := e.nodeStates[target]

		// 开启延迟加载的 toNode 节点的 goroutine
		// {FromNode: "answer", OnEvent: "llm_chunk", ToNode: "tts"}
		// engine.nodeInput["answer"] <- Event{
		// 	Type: "user_input",
		// 	From: "external",
		// 	Data: "Tell me about Golang",
		// }
		// 例如首次向 answer 发送消息 tts 是延迟加载

		// activeUpstreams 应该表示 “有多少个上游节点正在向我发送数据”，而不是收到了多少个事件
		// 如果 toNode 的上游fromNode已经被激活，则不再增加 activeUpstreams
		// 后续在fromNode结束时，通过edge找到toNode，对 activeUpstreams 进行相应的减少
		// isNode := e.isNode(ev.From)
		// state := e.nodeStates[target]
		// if _, ok := state.upstreams[ev.From]; ok {

		// }
		// if isNode {
		// 	if !state.upstreamActive[ev.From] {
		// 		state.mu.Lock()
		// 		// state.activeUpstreams++
		// 		state.upstreamActive[ev.From] = true
		// 		state.mu.Unlock()
		// 		if node.Mode() == ModeLazy {
		// 			e.startNode(target)
		// 		}
		// 	}

		// } else {
		// 	if node.Mode() == ModeLazy {
		// 		e.startNode(target)
		// 	}

		// }
		state.mu.Lock()
		state.receivedEvents[ev.Type] = struct{}{}

		if node.Mode() == ModeLazy && !state.started {
			state.pendingEvents = append(state.pendingEvents, ev)
			shouldStart := e.shouldStartNode(target, state)
			state.mu.Unlock()

			if shouldStart {
				e.startNode(target)
				e.flushPendingEvents(target)
			}
			continue
		}

		state.mu.Unlock()

		// if node.Mode() == ModeLazy {
		// 	e.startNode(target)
		// }

		e.nodeInput[target] <- ev
		// select {
		// case :
		// default:
		// 	log.Println("node input channel is full, dropping event")
		// }

		// if audio, ok := ev.Data.([]byte); ok {
		// 	log.Printf(">>>>>>>> from  %s to %s receive %d bytes", ev.From, target, len(audio))
		// } else {
		// 	log.Printf(">>>>>>>> from  %s to %s receive %s", ev.From, target, ev.Data)
		// }
	}
	if ev.Type == "node_done" {
		e.wg.Done()
		log.Printf(">>>node:  %d  %s -1", e.rtx.SessionID, ev.From)

		log.Printf(">>>> end signal %s", ev.From)
		e.handleNodeDone(ev.From)
	}

}

func (e *Engine) shouldStartNode(nodeID string, state *NodeState) bool {
	rule, hasRule := e.startRules[nodeID]
	if !hasRule {
		return true
	}

	received := make(map[string]struct{}, len(state.receivedEvents))
	for eventType := range state.receivedEvents {
		received[eventType] = struct{}{}
	}

	return rule.CanStart(NodeStartContext{
		NodeID:         nodeID,
		ReceivedEvents: received,
	})
}

func (e *Engine) flushPendingEvents(nodeID string) {
	state, ok := e.nodeStates[nodeID]
	if !ok {
		return
	}

	// state.mu.Lock()
	pending := state.pendingEvents
	state.pendingEvents = nil
	// state.mu.Unlock()

	for _, pendingEvent := range pending {
		e.nodeInput[nodeID] <- pendingEvent
	}
}
func (e *Engine) handleNodeDone(nodeID string) {

	targets := e.downstream[nodeID]

	for _, target := range targets {

		state, ok := e.nodeStates[target]
		if !ok {
			log.Printf("error: invalid node %s", target)
			continue
		}

		state.mu.Lock()

		delete(state.upstreamActive, nodeID)

		// if len(state.upstreamActive) == 0 && !state.done {
		if e.shouldCloseNode(target, state) {

			close(e.nodeInput[target])
			state.done = true

		}

		state.mu.Unlock()
	}
}

func (e *Engine) shouldCloseNode(nodeID string, state *NodeState) bool {
	if state.done {
		return false
	}
	rule, hasRule := e.closeRules[nodeID]
	if !hasRule {
		return len(state.upstreamActive) == 0
	}

	received := make(map[string]struct{}, len(state.receivedEvents))
	for eventType := range state.receivedEvents {
		received[eventType] = struct{}{}
	}

	return rule.CanClose(NodeCloseContext{
		NodeID:          nodeID,
		ReceivedEvents:  received,
		ActiveUpstreams: len(state.upstreamActive),
	})
}

// func (e *Engine) dispatch2(ev *Event) {

// 	if ev.Type == "node_done" {
// 		e.wg.Done()
// 		log.Printf(">>>> end signal %s", ev.From)
// 		e.handleUpstreamDone(ev.From)
// 		return
// 	}
// 	// targets := e.router.Route(ev)

// 	// if ev.Type == "node_done" {
// 	// 	e.wg.Done()
// 	// 	log.Printf(">>>> end signal %s", ev.From)
// 	// 	// e.handleUpstreamDone(ev.From)
// 	// 	for _, target := range targets {
// 	// 		state := e.nodeStates[target]

// 	// 		state.mu.Lock()

// 	// 		if state.upstreamActive[ev.From] {
// 	// 			delete(state.upstreamActive, ev.From)
// 	// 			state.activeUpstreams--
// 	// 			// log.Printf("Node %s activeUpstreams: %d", target, state.activeUpstreams)
// 	// 		}

// 	// 		//  && state.started
// 	// 		if state.activeUpstreams == 0 && !state.done {
// 	// 			close(e.nodeInput[target])
// 	// 			// delete(e.nodeStates, target)
// 	// 			// delete(e.nodeInput, target)

// 	// 			log.Printf("node done: %s", target)
// 	// 		}
// 	// 		state.mu.Unlock()
// 	// 	}
// 	// 	return
// 	// }
// 	// for _, target := range targets {
// 	// 	node := e.dag.Nodes[target]

// 	// 	// 开启延迟加载的 toNode 节点的 goroutine
// 	// 	// {FromNode: "answer", OnEvent: "llm_chunk", ToNode: "tts"}
// 	// 	// engine.nodeInput["answer"] <- Event{
// 	// 	// 	Type: "user_input",
// 	// 	// 	From: "external",
// 	// 	// 	Data: "Tell me about Golang",
// 	// 	// }
// 	// 	// 例如首次向 answer 发送消息 tts 是延迟加载
// 	// 	if node.Mode() == ModeLazy {
// 	// 		e.startNode(target)
// 	// 	}

// 	// 	// activeUpstreams 应该表示 “有多少个上游节点正在向我发送数据”，而不是收到了多少个事件
// 	// 	// 如果 toNode 的上游fromNode已经被激活，则不再增加 activeUpstreams
// 	// 	// 后续在fromNode结束时，通过edge找到toNode，对 activeUpstreams 进行相应的减少

// 	// 	state := e.nodeStates[target]
// 	// 	state.mu.Lock()
// 	// 	if !state.upstreamActive[ev.From] {
// 	// 		state.activeUpstreams++
// 	// 		state.upstreamActive[ev.From] = true
// 	// 	}
// 	// 	state.mu.Unlock()

// 	// 	e.nodeInput[target] <- ev

// 	// 	// log.Printf(">>>>>>>> from  %s to %s receive %s", ev.From, target, ev.Data)
// 	// }
// 	for _, edge := range e.dag.Edges {

// 		if edge.FromNode != ev.From || edge.OnEvent != ev.Type {
// 			continue
// 		}
// if edge.Cond != nil && !edge.Cond(ev) {
// 	continue
// }
// 		target := edge.ToNode

// 		node := e.dag.Nodes[target]

// 		// 开启延迟加载的 toNode 节点的 goroutine
// 		// {FromNode: "answer", OnEvent: "llm_chunk", ToNode: "tts"}
// 		// engine.nodeInput["answer"] <- Event{
// 		// 	Type: "user_input",
// 		// 	From: "external",
// 		// 	Data: "Tell me about Golang",
// 		// }
// 		// 例如首次向 answer 发送消息 tts 是延迟加载
// 		if node.Mode() == ModeLazy {
// 			e.startNode(target)
// 		}

// 		// activeUpstreams 应该表示 “有多少个上游节点正在向我发送数据”，而不是收到了多少个事件
// 		// 如果 toNode 的上游fromNode已经被激活，则不再增加 activeUpstreams
// 		// 后续在fromNode结束时，通过edge找到toNode，对 activeUpstreams 进行相应的减少

// 		state := e.nodeStates[target]
// 		state.mu.Lock()
// 		if !state.upstreamActive[ev.From] {
// 			// state.activeUpstreams++
// 			state.upstreamActive[ev.From] = true
// 		}
// 		state.mu.Unlock()

// 		e.nodeInput[target] <- ev

// 		// log.Printf(">>>>>>>> from  %s to %s receive %s", edge.FromNode, target, ev.Data)
// 	}

// }

// func (e *Engine) handleUpstreamDone(from string) {

// 	for _, edge := range e.dag.Edges {

// 		if edge.FromNode != from {
// 			continue
// 		}

// 		target := edge.ToNode
// 		state := e.nodeStates[target]

// 		state.mu.Lock()

// 		if state.upstreamActive[from] {
// 			delete(state.upstreamActive, from)
// 			// state.activeUpstreams--
// 			// log.Printf("Node %s activeUpstreams: %d", target, state.activeUpstreams)
// 		}

// 		//  && state.started
// 		if len(state.upstreamActive) == 0 && !state.done {
// 			close(e.nodeInput[target])
// 			// delete(e.nodeStates, target)
// 			// delete(e.nodeInput, target)

// 			log.Printf("node done: %s", target)
// 		}
// 		state.mu.Unlock()
// 	}

// }
