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

	Run(ctx context.Context, in <-chan Event, out chan<- Event) error
}
type NodeState struct {
	mu      sync.Mutex
	started bool
	// upstreams int
	activeUpstreams int
	done            bool
	upstreamActive  map[string]bool
}
type Event struct {
	Type string
	From string
	Data any
}

type Edge struct {
	FromNode string
	OnEvent  string
	ToNode   string
}

type DAG struct {
	Nodes map[string]Node
	Edges []Edge
}

type Engine struct {
	dag        *DAG
	nodeInput  map[string]chan Event
	nodeStates map[string]*NodeState

	bus    chan Event
	ctx    context.Context
	cancel context.CancelFunc
	g      *errgroup.Group
	// nodeG    *errgroup.Group
	// nodeCtx  context.Context
	waitOnce sync.Once
	wg       sync.WaitGroup

	// Output chan Event // TUDO

}

func NewEngine(ctx context.Context, cancel context.CancelFunc, dag *DAG) *Engine {
	g, ctx := errgroup.WithContext(ctx)
	// nodeG, nodeCtx := errgroup.WithContext(ctx)
	e := &Engine{
		dag:        dag,
		nodeInput:  make(map[string]chan Event),
		nodeStates: make(map[string]*NodeState),
		bus:        make(chan Event, 64),
		ctx:        ctx,
		cancel:     cancel,
		g:          g,

		// nodeG:      nodeG,
		// nodeCtx:    nodeCtx,
	}

	// upstreamCount := make(map[string]int)
	// for _, edge := range dag.Edges {
	// 	upstreamCount[edge.ToNode]++
	// }

	for id := range dag.Nodes {
		e.nodeInput[id] = make(chan Event, 32)
		e.nodeStates[id] = &NodeState{
			upstreamActive: make(map[string]bool),
			// upstreams: upstreamCount[id],
		}
	}

	return e
}

func (e *Engine) Start() {
	// g, ctx := errgroup.WithContext(e.ctx)

	// ⭐ 预启动 AlwaysOn 节点
	for id, node := range e.dag.Nodes {
		if node.Mode() == ModeAlwaysOn {
			e.startNode(id)
		}
	}

	// 启动 dispatcher
	e.g.Go(func() error {
		return e.dispatchLoop()
	})
	e.g.Go(func() error {
		e.wg.Wait()
		// e.cancel() // 任何节点出错或完成都取消整个引擎
		close(e.bus)

		// delete nodeInput nodeState
		for id := range e.nodeInput {
			// close(e.nodeInput[id])
			delete(e.nodeInput, id)
			// delete(e.nodeStates, id)
		}
		for id := range e.nodeStates {
			delete(e.nodeStates, id)
		}

		log.Printf("Engine stopped!")
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

	for {
		select {
		case ev, ok := <-e.bus:
			if !ok {
				return nil
			}
			e.dispatch(ev)

		case <-e.ctx.Done():
			return nil
		}
	}
}

func (e *Engine) startNode(id string) {

	state := e.nodeStates[id]

	state.mu.Lock()
	if state.started {
		state.mu.Unlock()
		return
	}
	state.started = true
	state.mu.Unlock()

	// state.activeUpstreams++
	// state.started = true
	// // e.wg.Add(1)
	// state.activeUpstreams++
	e.wg.Add(1)
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
		err := e.dag.Nodes[id].Run(
			e.ctx,
			e.nodeInput[id],
			e.bus,
		)
		// close(e.nodeInput[id])

		// e.onNodeExit(id)
		e.bus <- Event{
			Type: "node_done",
			From: id,
			Data: nil,
		}
		state.mu.Lock()
		state.done = true
		state.mu.Unlock()

		return err
	})

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

// dispatch 在 goroutine 中
// startNode 是动态调用
// 某个节点 e.wg.Done() 后
// 可能 dispatch 又 startNode
// 导致 wg.Wait() 提前返回或语义失效
func (e *Engine) dispatch(ev Event) {
	if ev.Type == "node_done" {
		e.wg.Done()
		log.Printf(">>>> end signal %s", ev.From)
		e.handleUpstreamDone(ev.From)
		return
	}
	for _, edge := range e.dag.Edges {

		if edge.FromNode != ev.From || edge.OnEvent != ev.Type {
			continue
		}
		target := edge.ToNode

		node := e.dag.Nodes[target]

		// 开启延迟加载的 toNode 节点的 goroutine
		// {FromNode: "answer", OnEvent: "llm_chunk", ToNode: "tts"}
		// engine.nodeInput["answer"] <- Event{
		// 	Type: "user_input",
		// 	From: "external",
		// 	Data: "Tell me about Golang",
		// }
		// 例如首次向 answer 发送消息 tts 是延迟加载
		if node.Mode() == ModeLazy {
			e.startNode(target)
		}

		// activeUpstreams 应该表示 “有多少个上游节点正在向我发送数据”，而不是收到了多少个事件
		// 如果 toNode 的上游fromNode已经被激活，则不再增加 activeUpstreams
		// 后续在fromNode结束时，通过edge找到toNode，对 activeUpstreams 进行相应的减少

		state := e.nodeStates[target]
		state.mu.Lock()
		if !state.upstreamActive[ev.From] {
			state.activeUpstreams++
			state.upstreamActive[ev.From] = true
		}
		state.mu.Unlock()

		e.nodeInput[target] <- ev

		log.Printf(">>>>>>>> from  %s to %s receive %s", edge.FromNode, target, ev.Data)
	}

}

func (e *Engine) handleUpstreamDone(from string) {

	for _, edge := range e.dag.Edges {

		if edge.FromNode != from {
			continue
		}

		target := edge.ToNode
		state := e.nodeStates[target]

		state.mu.Lock()

		if state.upstreamActive[from] {
			delete(state.upstreamActive, from)
			state.activeUpstreams--
			// log.Printf("Node %s activeUpstreams: %d", target, state.activeUpstreams)
		}

		//  && state.started
		if state.activeUpstreams == 0 && !state.done {
			close(e.nodeInput[target])
			// delete(e.nodeStates, target)
			// delete(e.nodeInput, target)

			log.Printf("node done: %s", target)
		}
		state.mu.Unlock()
	}

}
