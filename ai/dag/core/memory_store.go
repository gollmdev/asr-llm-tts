package dag

import "sync"

type Message string
type MemoryStore interface {
	Get(sessionID string) []Message
	Append(sessionID string, msg Message)
}

type InMemoryStore struct {
	mu    sync.RWMutex
	store map[string][]Message
}

func NewMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		store: make(map[string][]Message),
	}
}

func (m *InMemoryStore) Get(sessionID string) []Message {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]Message{}, m.store[sessionID]...)
}

func (m *InMemoryStore) Append(sessionID string, msg Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[sessionID] = append(m.store[sessionID], msg)
}
