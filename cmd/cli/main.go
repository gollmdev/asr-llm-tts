package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/gollmdev/asr-llm-tts/ai/memory"
	"github.com/gollmdev/asr-llm-tts/ai/tools"

	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
	"github.com/gollmdev/asr-llm-tts/ai/dag/dagtypes"
	"github.com/gollmdev/asr-llm-tts/ai/dag/node"
)

var (
	GlobalDAG *dag.DAG
	once      sync.Once
)

func InitDAG() {
	once.Do(func() {
		toolDefs := tools.CustomDefinitions()
		toolExecutor := node.NewToolExecutorNode()
		missing := tools.RegisterCustom(toolExecutor, toolDefs)
		if len(missing) > 0 {
			log.Printf("tool handlers missing: %v", missing)
		}
		// contextMergeNode := node.NewContextMergeNode()
		GlobalDAG = dag.NewDAG(map[string]dag.Node{
			"router":               &node.RouterNode{},
			"context_merge":        &node.ContextMergeNode{},
			"chat":                 &node.LLMNode{Tools: toolDefs},
			"rag":                  &node.RAGNode{TopK: 5},
			"tool_executor":        toolExecutor,
			"conversation_context": &node.ConversationContextNode{RecentLimit: 5},
			"prompt_assembly":      &node.PromptAssemblyNode{},

			"tts": &node.TTSNode{},
			"asr": &node.ASRNode{},
			// "db":     &cnode.DBNode{}, // text and (asr_text or llm_chunk)
			"output": &node.OutputNode{},
		}, []dag.Edge{
			// 输入
			{FromNode: "__external__", OnEvent: "text", ToNode: "conversation_context"},
			{FromNode: "__external__", OnEvent: "audio", ToNode: "asr"},
			{FromNode: "__external__", OnEvent: "audio_end", ToNode: "asr"},

			// ASR
			{FromNode: "asr", OnEvent: "asr_text", ToNode: "conversation_context"},
			{FromNode: "asr", OnEvent: "asr_text", ToNode: "output"},
			{FromNode: "asr", OnEvent: "no_asr_text", ToNode: "output"},

			// 上下文 -> 路由
			{FromNode: "conversation_context", OnEvent: "context_ready", ToNode: "router"},

			// 路由结果进入 merge
			{FromNode: "router", OnEvent: "route_ready", ToNode: "context_merge"},

			// 只有需要 RAG 时才走 RAG
			{FromNode: "router", OnEvent: "need_rag", ToNode: "rag"},

			// RAG 检索结果进入 merge
			{FromNode: "rag", OnEvent: "rag_ready", ToNode: "context_merge"},

			// merge 完成后组 prompt
			{FromNode: "context_merge", OnEvent: "context_merged", ToNode: "prompt_assembly"},

			// prompt -> chat
			{FromNode: "prompt_assembly", OnEvent: "prompt_ready", ToNode: "chat"},

			// tool loop
			{FromNode: "chat", OnEvent: "llm_tool_call", ToNode: "tool_executor"},
			// {FromNode: "tool_executor", OnEvent: "tool_result", ToNode: "chat"},
			{FromNode: "tool_executor", OnEvent: "tool_result_context", ToNode: "context_merge"},
			{FromNode: "chat", OnEvent: "llm_complete", ToNode: "context_merge"},
			{FromNode: "chat", OnEvent: "llm_complete", ToNode: "prompt_assembly"},
			{FromNode: "chat", OnEvent: "llm_complete", ToNode: "tool_executor"},

			//输出
			// {FromNode: "__external__", OnEvent: "text", ToNode: "db"},
			// {FromNode: "asr", OnEvent: "asr_text", ToNode: "db"},
			// {FromNode: "chat", OnEvent: "llm_chunk", ToNode: "db"},
			// {FromNode: "chat", OnEvent: "node_done", ToNode: "db"},
			{FromNode: "chat", OnEvent: "llm_chunk", ToNode: "output"},
			{FromNode: "chat", OnEvent: "llm_chunk", ToNode: "tts",
				Cond: func(ev *dag.Event) bool {
					return ev.Rtx.EnableTTS // && ev.Data.(string) != "你好"
				}},
			{FromNode: "tts", OnEvent: "tts_audio", ToNode: "output"},

			// 观测
			{FromNode: "chat", OnEvent: "llm_tool_call", ToNode: "output"},
			{FromNode: "tool_executor", OnEvent: "tool_result", ToNode: "output"},
			// {FromNode: "db", OnEvent: "create_session", ToNode: "output"},
		})
	})
}

func GetDAG() *dag.DAG {
	if GlobalDAG == nil {
		InitDAG()
	}
	return GlobalDAG
}

func main() {
	ctx := context.Background()
	dagModel := GetDAG()
	service := map[string]any{
		// "userService":         h.userService,
		// "chatService":         h.chatService,
		// "conversationService": h.conversationService,
		// "db":                  h.db,
	}
	userID := int64(123)
	sessionID := int64(456)
	memory := memory.NewDBStore(ctx, userID, sessionID)
	session := dag.NewSession(&dag.SessionConfig{
		Ctx:       ctx,
		Dag:       dagModel,
		SessionId: sessionID,
		Services:  service,
		UserId:    userID,
		Memory:    memory,
	})

	session.Start()
	msg := []*dagtypes.Message{
		{
			Role:    "user",
			Content: "你好?",
		},
	}
	session.Dispatch("text", msg)

	var wg sync.WaitGroup
	wg.Go(func() {
		for ev := range session.Output() {
			// for audio print length
			if text, ok := ev.Data.(string); ok {
				fmt.Println("FINAL OUTPUT:", text)

			} else {
				if audio, ok := ev.Data.([]byte); ok {
					fmt.Println("FINAL OUTPUT LENGTH:", len(audio))
				}

			}
		}
	})
	wg.Wait()
	session.Close()
}
