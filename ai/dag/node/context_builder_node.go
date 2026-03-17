package node

import (
	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
	"github.com/gollmdev/asr-llm-tts/ai/provider/llm"
)

type ContextBuilderNode struct {
}

func (n *ContextBuilderNode) ID() string { return "context_builder" }
func (n *ContextBuilderNode) Mode() dag.NodeMode {
	return dag.ModeLazy
}

func (n *ContextBuilderNode) Run(rt dag.NodeRuntime) error {
	sessionID := rt.RuntimeContext().SessionID
	ctx := rt.Context()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-rt.Input():
			if !ok {
				return nil
			}
			userText := ev.Data.([]*llm.Message)

			mem := rt.RuntimeContext().Memory

			// 1️⃣ 写入 user
			mem.Append(sessionID, userText[0])

			// 2️⃣ 短期记忆
			short := mem.Get(sessionID)

			// 3️⃣ 长期记忆
			// long := n.LongTerm.Search(userText)

			// 4️⃣ RAG
			// docs := n.RAG.Search(userText)

			// 5️⃣ 构建 messages（关键）
			var messages []*llm.Message

			// system prompt
			messages = append(messages, &llm.Message{
				Role:    "system",
				Content: "你是一个智能助手",
			})

			// 长期记忆
			// messages = append(messages, long...)

			// RAG
			// if len(docs) > 0 {
			// 	messages = append(messages, Message{
			// 		Role:    "system",
			// 		Content: "参考资料:\n" + strings.Join(docs, "\n"),
			// 	})
			// }

			// 短期记忆（最重要）
			messages = append(messages, short...)

			rt.Emit(&dag.Event{
				Type: "messages",
				Data: messages,
			})

			return nil
		}
	}
}
