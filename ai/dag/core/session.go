package dag

import (
	"context"
	"log"
)

type Session struct {
	ID     int64
	engine *Engine
	output chan *Event
	rtx    *RuntimeContext

	// cancel context.CancelFunc
}

type ChannelEmitter struct {
	ch chan *Event
}

func (e *ChannelEmitter) Emit(ev *Event) {
	e.ch <- ev
}
func NewSession(ctx context.Context, dag *DAG, id int64) *Session {
	outputChan := make(chan *Event, 64)

	// 创建 OutputNode 并注入 outputChan
	// outputNode := NewOutputNode("output", outputChan)

	// 把 outputNode 加入 DAG
	// dag.Nodes["output"] = outputNode

	// 例如把 answer 的 llm_chunk 送到 output
	// dag.Edges = append(dag.Edges,
	// 	Edge{
	// 		FromNode: "answer",
	// 		OnEvent:  "llm_chunk",
	// 		ToNode:   "final",
	// 	},
	// )
	memory := NewMemoryStore()
	rtx := &RuntimeContext{
		sessionID: id,
		memory:    memory,
		Output:    &ChannelEmitter{outputChan},
		EnableTTS: false,
	}
	engine := NewEngine(ctx, dag, rtx)
	engine.OnDAGDone = func() {
		log.Printf(">>>node: %d DAG done!", id)
		close(outputChan)
	}

	engine.Use(LoggingMiddleware())

	return &Session{
		engine: engine,
		output: outputChan,
		ID:     id,
		rtx:    rtx,
	}
}

func (s *Session) SetTTS(enable bool) {
	// if s.engine != nil && s.engine.rtx != nil {
	// 	s.engine.rtx.EnableTTS = enable
	// }
	s.rtx.EnableTTS = enable
}

func (s *Session) Dispatch(eventType string, data any) {
	// s.engine.wg.Add(1)
	s.engine.dispatch(&Event{
		From: "__external__",
		Type: eventType,
		Data: data,
	})

	// s.engine.dispatch(&Event{
	// 	Type: "node_done",
	// 	From: "input",
	// 	Data: nil,
	// })
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
