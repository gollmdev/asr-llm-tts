package dag

import (
	"encoding/json"

	"github.com/gollmdev/asr-llm-tts/ai/dag/service"
)

type RuntimeContext struct {
	UserID    int64
	SessionID int64
	Memory    MemoryStore
	EnableTTS bool
	Output    Emitter
	Services  map[string]any
	Retriever service.Retriever
	Config    any
	// Ctx       context.Context
	// Cancel    context.CancelFunc
}
type Event struct {
	Type string
	From string
	Data any
	Ctx  map[string]any
	Rtx  *RuntimeContext
}

func (e *Event) GetDBString() (string, error) {
	// 1. 先判断 Data 是否本来就是字符串
	if str, ok := e.Data.(string); ok {
		return str, nil // 是字符串，直接返回
	}

	// 2. 如果不是字符串（比如是结构体、map等），则序列化为 JSON
	jsonBytes, err := json.Marshal(e.Data)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

type Emitter interface {
	Emit(*Event)
}
