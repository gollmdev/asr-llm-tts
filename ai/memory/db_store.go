package memory

import (
	"context"
	"sync"

	"github.com/gollmdev/asr-llm-tts/ai/dag/dagtypes"
)

type DBStore struct {
	mu sync.RWMutex
	// userService         *service.UserService
	// chatService         *service.ChatMessageService
	// conversationService *service.ConversationService
	ctx    context.Context
	userID int64
	store  map[int64][]*dagtypes.Message
}

func NewDBStore(ctx context.Context,
	userID int64,
	sessionID int64,
	// userService *service.UserService,
	// chatService *service.ChatMessageService,
	// conversationService *service.ConversationService
) *DBStore {
	return &DBStore{
		// userService:         userService,
		// chatService:         chatService,
		// conversationService: conversationService,
		ctx:    ctx,
		userID: userID,
		store:  make(map[int64][]*dagtypes.Message),
	}
}

func (m *DBStore) Init(sessionID int64) error {
	if sessionID == 0 {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	// message, err := m.chatService.ListChatMessagesBySessionID(m.ctx, sessionID, m.userID, "", 10)
	// if err != nil {
	// 	return err
	// }
	// convert to dagtypes.Message
	var msgs []*dagtypes.Message
	// for _, msg := range message {
	// 	msgs = append(msgs, &dagtypes.Message{
	// 		Role:    msg.Role,
	// 		Content: msg.Content,
	// 	})
	// }
	m.store[sessionID] = msgs
	return nil
}

func (m *DBStore) Get(sessionID int64) []*dagtypes.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]*dagtypes.Message{}, m.store[sessionID]...)
}

func (m *DBStore) Append(sessionID int64, msg *dagtypes.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[sessionID] = append(m.store[sessionID], msg)
	return nil

}

func (m *DBStore) GetRecent(sessionID int64, n int) ([]*dagtypes.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	msgs := m.store[sessionID]
	if len(msgs) <= n {
		return append([]*dagtypes.Message{}, msgs...), nil
	}
	return append([]*dagtypes.Message{}, msgs[len(msgs)-n:]...), nil
}

func (m *DBStore) GetSummary(sessionID int64) (string, error) {
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

func (m *DBStore) Save(sessionID int64) error { // 将内存中的消息保存到数据库中
	m.mu.RLock()
	defer m.mu.RUnlock()
	// msgs := m.store[sessionID]
	// for _, msg := range msgs {
	// 	// 这里需要将dagtypes.Message转换为model.ChatMessage
	// 	chatMsg := &model.ChatMessage{
	// 		UserID:    m.userID,
	// 		SessionID: sessionID,
	// 		Role:      msg.Role,
	// 		Content:   msg.Content,
	// 	}
	// 	err := m.chatService.CreateChatMessage(m.ctx, chatMsg)
	// 	if err != nil {
	// 		return err
	// 	}
	// }
	return nil
}
