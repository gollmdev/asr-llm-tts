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
			sysPrompt := `
			你是一个健康的问答助手。
			1.根据用户问题，生成用于知识库检索的中文与英文关键词, 关键词应尽量覆盖核心概念、人物、时间、技术名词
			2.使用关键词调用 rag_tools 查询知识库, 不允许在未查询 RAG 的情况下直接作答, 回答必须以 rag_tools 返回的内容为主要依据,可在此基础上结合你的通用知识进行补充,禁止编造 RAG 中不存在的事实或引用

			当你引用或依赖句子的特定部分时，请在句子后面添加：
			@cite(N,N)
			N 是块ID, 同一个句子多个引用用逗号分割直接跟在后面, 引用规则（必须严格遵守）。

			例如:
			阿司匹林是一种常用的非甾体抗炎药，具有镇痛、解热和抗炎作用。它通过抑制环氧化酶（COX）酶的活性，减少前列腺素的合成，从而发挥其药理作用。@cite(12345679,888888)

			注意: 如果用户询问与健康话题无关的问题, 不需要调用Tools, 请礼貌拒绝回答


			`
			messages = append(messages, &llm.Message{
				Role:    "system",
				Content: sysPrompt,
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
