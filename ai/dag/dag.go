package dag

import (
	"context"
	"fmt"
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

func (e *Engine) Start() error {
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
		log.Printf("Engine stopped!")
		// return err
		return nil
	})
	return e.g.Wait()
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
		// log.Printf(">>>> start %s", id)
		// defer log.Printf(">>>> end %s", id)

		// defer e.wg.Done()
		// state.mu.Lock()
		// state.started = true
		// // state.activeUpstreams++
		// state.mu.Unlock()

		e.bus <- Event{
			Type: "node_start",
			From: id,
			Data: nil,
		}
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
		// log.Printf("onNodeExit close %s", id)

		return err
	})

	// e.waitOnce.Do(func() {
	// 	e.g.Go(func() error {
	// 		e.wg.Wait()
	// 		e.cancel() // 任何节点出错或完成都取消整个引擎
	// 		// close(e.bus)
	// 		log.Printf("Engine stopped!")
	// 		// return err
	// 		return nil
	// 	})
	// })
	// go func() {

	// }()
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

	for _, edge := range e.dag.Edges {

		if edge.FromNode != ev.From || edge.OnEvent != ev.Type {
			continue
		}
		target := edge.ToNode

		node := e.dag.Nodes[target]

		// ⭐ Lazy 节点才按需启动
		if node.Mode() == ModeLazy {
			e.startNode(target)
		}

		// if ev.Type == "node_start" {

		// 	return

		// }
		state := e.nodeStates[target]
		log.Printf(">>>> start %s", target)

		state.mu.Lock()
		if !state.upstreamActive[ev.From] {
			state.activeUpstreams++
			state.upstreamActive[ev.From] = true

		}
		state.mu.Unlock()
		// activeUpstreams 应该表示 “有多少个上游节点正在向我发送数据”，而不是收到了多少个事件
		// 如果 toNode 的上游fromNode已经被激活，则不再增加 activeUpstreams
		// 后续在fromNode结束时，通过edge找到toNode，对 activeUpstreams 进行相应的减少

		e.nodeInput[target] <- ev

		log.Printf("from  %s to %s receive %s", edge.FromNode, target, ev.Data)
	}
	if ev.Type == "node_done" {
		e.wg.Done()
		log.Printf(">>>> end done %s", ev.From)
		e.handleUpstreamDone(ev.From)
		// e.onNodeExit(ev.From)
		// close(e.nodeInput[ev.From])
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
		// state.upstreams--

		// if state.upstreams == 0 && !state.done {
		// 	close(e.nodeInput[target])
		// 	log.Printf("node done: %s", target)

		// }
		// state.activeUpstreams--
		if state.upstreamActive[from] {
			delete(state.upstreamActive, from)
			state.activeUpstreams--
			log.Printf("Node %s activeUpstreams: %d", target, state.activeUpstreams)
		}

		//  && state.started
		if state.activeUpstreams == 0 && !state.done {
			close(e.nodeInput[target])
			log.Printf("node done: %s", target)
		}
		state.mu.Unlock()
	}

	// for _, node := range e.dag.Nodes {
	// 	state := e.nodeStates[node.ID()]
	// 	// state.mu.Lock()
	// 	if state.activeUpstreams != 0 {
	// 		return
	// 	}
	// }
	// // close(e.bus)
	// log.Println(e.dag.Nodes)
	// print all node activeUpstreams
	// for _, node := range e.dag.Nodes {
	// 	state := e.nodeStates[node.ID()]
	// 	log.Printf("Node %s activeUpstreams: %d", node.ID(), state.activeUpstreams)
	// }
	// log.Println("Engine stopped!")
}

func Test() {
	outputNode := NewOutputNode("final")

	dag := &DAG{
		Nodes: map[string]Node{
			"answer": &AnswerNode{},
			"tts":    &TTSNode{},
			"output": outputNode,
		},
		Edges: []Edge{
			{FromNode: "keyword", OnEvent: "keyword_done", ToNode: "db"},
			// {FromNode: "db", OnEvent: "db_result", ToNode: "answer"},
			{FromNode: "answer", OnEvent: "llm_chunk", ToNode: "tts"},
			{FromNode: "tts", OnEvent: "tts_audio", ToNode: "output"},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	engine := NewEngine(ctx, cancel, dag)
	go func() {
		// time.Sleep(200 * time.Millisecond)
		engine.nodeInput["answer"] <- Event{
			Type: "user_input",
			From: "external",
			Data: "Tell me about Golang",
		}
	}()
	go func() {
		for ev := range outputNode.Output {
			fmt.Println("FINAL OUTPUT:", ev.Data)
		}
	}()
	engine.Start()

}
