package dag

import (
	"sync"

	"github.com/gollmdev/asr-llm-tts/ai/provider/llm"
)

// type Message string
type MemoryStore interface {
	Get(sessionID int64) []*llm.Message
	Append(sessionID int64, msg *llm.Message)
}

type InMemoryStore struct {
	mu    sync.RWMutex
	store map[int64][]*llm.Message
}

func NewMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		store: make(map[int64][]*llm.Message),
	}
}

func (m *InMemoryStore) Get(sessionID int64) []*llm.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]*llm.Message{}, m.store[sessionID]...)
}

func (m *InMemoryStore) Append(sessionID int64, msg *llm.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[sessionID] = append(m.store[sessionID], msg)
}
