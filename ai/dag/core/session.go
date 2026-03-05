package dag

import "context"

type Session struct {
	// ID     string
	engine *Engine
	output chan *Event

	// cancel context.CancelFunc
}

func NewSession(ctx context.Context, dag *DAG, id string) *Session {
	outputChan := make(chan *Event, 64)

	// 创建 OutputNode 并注入 outputChan
	outputNode := NewOutputNode("output", outputChan)

	// 把 outputNode 加入 DAG
	dag.Nodes["output"] = outputNode

	// 例如把 answer 的 llm_chunk 送到 output
	// dag.Edges = append(dag.Edges,
	// 	Edge{
	// 		FromNode: "answer",
	// 		OnEvent:  "llm_chunk",
	// 		ToNode:   "final",
	// 	},
	// )
	memory := NewMemoryStore()

	engine := NewEngine(ctx, dag, &RuntimeContext{
		sessionID: id,
		memory:    memory,
	})

	return &Session{
		engine: engine,
		output: outputChan,
	}
}

func (s *Session) Dispatch(data any) {
	s.engine.wg.Add(1)
	s.engine.dispatch(&Event{
		From: "input",
		Type: "input",
		Data: data,
	})

	s.engine.dispatch(&Event{
		Type: "node_done",
		From: "input",
		Data: nil,
	})
}

func (s *Session) Start() {
	s.engine.Start()
}

func (s *Session) Close() {
	s.engine.Close()
}

func (s *Session) Output() <-chan *Event {
	return s.output
}
