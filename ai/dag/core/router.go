package dag

type EventRouter struct {
	routes map[string]map[string][]*Edge
}

func NewEventRouter(
	edges []Edge,
) *EventRouter {

	routes := make(map[string]map[string][]*Edge)

	for _, e := range edges {

		if routes[e.FromNode] == nil {
			routes[e.FromNode] = make(map[string][]*Edge)
		}

		routes[e.FromNode][e.OnEvent] =
			append(routes[e.FromNode][e.OnEvent], &e)
	}

	return &EventRouter{
		routes: routes,
	}
}
func (r *EventRouter) Route(ev *Event) []string {

	m := r.routes[ev.From]
	// if edge.Cond != nil && !edge.Cond(ev) {
	// 		continue
	// }
	if m == nil {
		return nil
	}
	targets := m[ev.Type]
	var result []string
	for _, edge := range targets {
		if edge.Cond != nil && !edge.Cond(ev) {
			continue
		}
		result = append(result, edge.ToNode)
	}
	return result
}

// func NewEventRouter(
// 	edges []Edge,
// 	nodeInput map[string]chan *Event,
// ) *EventRouter {
// 	return &EventRouter{
// 		edges:     edges,
// 		nodeInput: nodeInput,
// 	}
// }

// func (r *EventRouter) Route(ev *Event) {

// 	for _, edge := range r.edges {

// 		if edge.FromNode != ev.From {
// 			continue
// 		}

// 		if edge.OnEvent != ev.Type {
// 			continue
// 		}

// 		ch := r.nodeInput[edge.ToNode]

// 		select {
// 		case ch <- ev:

// 		default:
// 			// 避免阻塞
// 		}
// 	}
// }

// func (r *EventRouter) Route(ev *Event) []*Edge {

// 	key := ev.From + ":" + ev.Type

// 	edges := r.routes[key]

// 	var result []*Edge

// 	for _, edge := range edges {

// 		if edge.Cond != nil && !edge.Cond(ev) {
// 			continue
// 		}

// 		result = append(result, edge)
// 	}

// 	return result
// }

// func (e *EventRouter) dispatch(ev *Event) {
// 	if ev.Type == "node_done" {
// 		e.wg.Done()
// 		log.Printf(">>>> end signal %s", ev.From)
// 		e.handleUpstreamDone(ev.From)
// 		return
// 	}
// 	for _, edge := range e.edges {

// 		if edge.FromNode != ev.From || edge.OnEvent != ev.Type {
// 			continue
// 		}
// 		if edge.Cond != nil && !edge.Cond(ev) {
// 			continue
// 		}
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
// 			state.activeUpstreams++
// 			state.upstreamActive[ev.From] = true
// 		}
// 		state.mu.Unlock()

// 		e.nodeInput[target] <- ev

// 		log.Printf(">>>>>>>> from  %s to %s receive %s", edge.FromNode, target, ev.Data)
// 	}

// }

// func (e *EventRouter) handleUpstreamDone(from string) {

// 	for _, edge := range e.dag.Edges {

// 		if edge.FromNode != from {
// 			continue
// 		}

// 		target := edge.ToNode
// 		state := e.nodeStates[target]

// 		state.mu.Lock()

// 		if state.upstreamActive[from] {
// 			delete(state.upstreamActive, from)
// 			state.activeUpstreams--
// 			// log.Printf("Node %s activeUpstreams: %d", target, state.activeUpstreams)
// 		}

// 		//  && state.started
// 		if state.activeUpstreams == 0 && !state.done {
// 			close(e.nodeInput[target])
// 			// delete(e.nodeStates, target)
// 			// delete(e.nodeInput, target)

// 			log.Printf("node done: %s", target)
// 		}
// 		state.mu.Unlock()
// 	}

// }
