package node

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
	"github.com/gollmdev/asr-llm-tts/ai/dag/dagtypes"
	"github.com/gollmdev/asr-llm-tts/ai/provider/llm"
)

type RouterNode struct{}

func (n *RouterNode) ID() string         { return "router" }
func (n *RouterNode) Mode() dag.NodeMode { return dag.ModeLazy }

func (n *RouterNode) Run(rt dag.NodeRuntime) error {

	ctx := rt.Context()
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
				log.Println("[router] invalid turn context")
				continue
			}

			decision := n.routeWithLLM(ctx, rt, tc)
			tc.Route = decision

			rt.Emit(&dag.Event{
				Type: "route_ready",
				Data: tc,
				Rtx:  ev.Rtx,
			})

			if decision.UseRAG {
				rt.Emit(&dag.Event{
					Type: "need_rag",
					Data: tc,
					Rtx:  ev.Rtx,
				})
			}
		}
	}

}
func (n *RouterNode) routeWithLLM(ctx context.Context, rt dag.NodeRuntime, tc *dagtypes.TurnContext) *dagtypes.RouteDecision {
	sysPrompt := `
你是一个路由器。判断当前请求是否需要：
1. use_rag: 需要从知识库检索事实、专业资料、文档依据
2. use_tools: 需要调用工具（天气、计算、搜索、数据库、外部API等）
3. direct_reply: 可直接基于已有上下文回答

只返回 JSON：
{
  "intent": "xxx",
  "use_rag": true,
  "use_tools": false,
  "direct_reply": false,
  "reason": "xxx"
}
`

	messages := []*dagtypes.Message{
		{Role: "system", Content: strings.TrimSpace(sysPrompt)},
		{Role: "user", Content: tc.UserInput.Content},
	}

	model := llm.NewQwenChatModel(&llm.ChatModelConfig{
		Model: "qwen-plus",
		Ctx:   ctx,
		G:     rt.Group(),
	})

	model.Stream(dagtypes.ConvertMessages(messages))

	var buf strings.Builder
	for {
		msg, err := model.Recv()
		if err != nil {
			break
		}
		if msg.Content != nil {
			buf.WriteString(*msg.Content)
		}
	}

	raw := buf.String()
	var rr dagtypes.RouteDecision
	// "{\n  \"intent\": \"health_advice\",\n  \"use_rag\": true,\n  \"use_tools\": false,\n  \"direct_reply\": false,\n  \"reason\": \"短链脂肪酸（SCFA。\"\n}"
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &rr); err != nil {
		log.Printf("[router] parse route json failed: %v raw=%s", err, raw)
		return &dagtypes.RouteDecision{
			Intent:      "chat",
			UseRAG:      false,
			UseTools:    false,
			DirectReply: true,
			Reason:      "fallback",
			NeedTTS:     rt.RuntimeContext().EnableTTS,
		}
	}

	rr.NeedTTS = rt.RuntimeContext().EnableTTS
	return &rr
}
