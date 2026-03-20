package dag

import (
	"sync"

	"github.com/gollmdev/asr-llm-tts/ai/dag/dagtypes"
)

// type Message string
type MemoryStore interface {
	Get(sessionID int64) []*dagtypes.Message
	Append(sessionID int64, msg *dagtypes.Message) error
	GetRecent(sessionID int64, n int) ([]*dagtypes.Message, error)
	GetSummary(sessionID int64) (string, error)
}

type InMemoryStore struct {
	mu    sync.RWMutex
	store map[int64][]*dagtypes.Message
}

func NewMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		store: make(map[int64][]*dagtypes.Message),
	}
}

func (m *InMemoryStore) Get(sessionID int64) []*dagtypes.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]*dagtypes.Message{}, m.store[sessionID]...)
}

func (m *InMemoryStore) Append(sessionID int64, msg *dagtypes.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[sessionID] = append(m.store[sessionID], msg)
	return nil

}

func (m *InMemoryStore) GetRecent(sessionID int64, n int) ([]*dagtypes.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	msgs := m.store[sessionID]
	if len(msgs) <= n {
		return append([]*dagtypes.Message{}, msgs...), nil
	}
	return append([]*dagtypes.Message{}, msgs[len(msgs)-n:]...), nil
}

func (m *InMemoryStore) GetSummary(sessionID int64) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	msgs := m.store[sessionID]
	if len(msgs) == 0 {
		return "", nil
	}
	// 简单返回最后一条消息的前20个字符作为摘要，实际可以调用专门的文本摘要模型来生成
	lastMsg := msgs[len(msgs)-1]
	if lastMsg.Content == "" {
		return "", nil
	}
	content := lastMsg.Content
	if len(content) > 20 {
		content = content[:20]
	}
	return content, nil
}
