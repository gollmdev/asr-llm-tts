package node

import (
	"context"
	"log"
	"strings"

	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
	"github.com/gollmdev/asr-llm-tts/ai/dag/dagtypes"
	"github.com/gollmdev/asr-llm-tts/ai/provider/llm"
)

type RAGNode struct {
	TopK int
}

func (n *RAGNode) ID() string { return "rag" }

func (n *RAGNode) Mode() dag.NodeMode {
	return dag.ModeLazy
}

func (n *RAGNode) Run(rt dag.NodeRuntime) error {
	ctx := rt.Context()
	topK := n.TopK
	if topK <= 0 {
		topK = 4
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-rt.Input():
			if !ok {
				return nil
			}

			tc, ok := ev.Data.(*dagtypes.TurnContext)
			if !ok || tc == nil || tc.UserInput == nil {
				log.Println("[rag] invalid turn context")
				continue
			}

			query := n.buildQuery(ctx, rt, tc)
			docs, err := rt.RuntimeContext().Retriever.Search(query, topK)
			if err != nil {
				log.Printf("[rag] retriever search error: %v", err)
				docs = nil
			}

			tc.Docs = docs
			if tc.Metadata == nil {
				tc.Metadata = map[string]any{}
			}
			tc.Metadata["rag_query"] = query

			rt.Emit(&dag.Event{
				Type: "rag_ready",
				Data: tc,
				Rtx:  ev.Rtx,
			})
		}
	}
}

func (n *RAGNode) buildQuery(ctx context.Context, rt dag.NodeRuntime, tc *dagtypes.TurnContext) string {
	sysPrompt := `
你是检索查询生成器。请根据用户问题生成适合知识库检索的简洁查询。
要求：
1. 保留核心实体、术语、时间、英文名
2. 输出单行纯文本
3. 不要解释
`

	msgs := []*dagtypes.Message{
		{Role: "system", Content: strings.TrimSpace(sysPrompt)},
		{Role: "user", Content: tc.UserInput.Content},
	}

	model := llm.NewQwenChatModel(&llm.ChatModelConfig{
		Model: "qwen-plus",
		Ctx:   ctx,
		G:     rt.Group(),
	})
	model.Stream(dagtypes.ConvertMessages(msgs))

	var sb strings.Builder
	for {
		msg, err := model.Recv()
		if err != nil {
			break
		}
		if msg.Content != nil {
			sb.WriteString(*msg.Content)
		}
	}

	query := strings.TrimSpace(sb.String())
	if query == "" {
		query = tc.UserInput.Content
	}
	return query
}
