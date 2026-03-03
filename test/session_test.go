package test

import (
	"context"
	"log"
	"testing"

	"github.com/gollmdev/asr-llm-tts/ai/event"
	"github.com/gollmdev/asr-llm-tts/ai/model"
	"github.com/gollmdev/asr-llm-tts/ai/session"
)

type SessionCallback struct {
}

func (c *SessionCallback) OnEvent(ctx *session.EventContext, eventType event.EventType, text string) {

}

func (c *SessionCallback) OnCitationsEvent(citations map[string]model.Citations) {

}

func (c *SessionCallback) OnThoughtChainEvent(thoughtChain model.ThoughtChain) {

}

func (c *SessionCallback) OnFinish() {

}
func (c *SessionCallback) GetMessage(text string) []map[string]any {
	return []map[string]any{
		{"role": "system", "content": "你是我的人工智能助手，协助我解答问题。"},
		{"role": "user", "content": text},
	}
}
func TestSession(t *testing.T) {
	ctx := context.Background()
	session := session.NewSession(ctx, &SessionCallback{}, &session.SessionConfig{TTSEnabled: false})
	go func() {
		for message := range session.Output() {
			log.Printf("Received message from session: %s", message)
		}
	}()

	session.LLMConsumer()
	session.MonitorSubSize()
	session.PublishEvent(event.Event{
		Type: event.EventTextChunk,
		Data: "你好，你是可以干什么?",
	})

	session.Close()
}
