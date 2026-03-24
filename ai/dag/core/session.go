package dag

import (
	"context"
	"log"

	"github.com/gollmdev/asr-llm-tts/ai/dag/service"
)

type Session struct {
	ID     int64
	engine *Engine
	output chan *Event
	rtx    *RuntimeContext

	// cancel context.CancelFunc
}
type SessionConfig struct {
	Ctx       context.Context
	Dag       *DAG
	SessionId int64
	UserId    int64
	Services  map[string]any
	Memory    MemoryStore
}

type ChannelEmitter struct {
	ch chan *Event
}

func (e *ChannelEmitter) Emit(ev *Event) {
	e.ch <- ev
}

func NewSession(config *SessionConfig) *Session {
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
	// memory := NewMemoryStore()
	rtx := &RuntimeContext{
		SessionID: config.SessionId,
		Memory:    config.Memory,
		Output:    &ChannelEmitter{outputChan},
		EnableTTS: false,
		UserID:    config.UserId,
		Services:  config.Services,
		Retriever: &service.MockRetriever{},
	}
	engine := NewEngine(config.Ctx, config.Dag, rtx)
	engine.OnDAGDone = func() {
		log.Printf(">>>node: %d DAG done!", config.SessionId)
		close(outputChan)
	}

	engine.Use(LoggingMiddleware())

	return &Session{
		engine: engine,
		output: outputChan,
		ID:     config.SessionId,
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
