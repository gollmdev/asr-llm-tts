package test

import (
	"context"
	"fmt"
	"sync"

	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
	"github.com/gollmdev/asr-llm-tts/ai/dag/node"
)

func Test2() {
	// outputNode := dag.NewOutputNode("final")
	// fan-in
	dagModel := &dag.DAG{
		Nodes: map[string]dag.Node{
			"llm": &node.LLMNode{},
			"tts": &node.TTSNode{},
			"asr": &node.ASRNode{},
		},
		Edges: []dag.Edge{
			{FromNode: "__external__", OnEvent: "text", ToNode: "llm"},
			{FromNode: "__external__", OnEvent: "audio", ToNode: "asr"},
			{FromNode: "asr", OnEvent: "asr_test", ToNode: "llm"},

			{FromNode: "llm", OnEvent: "llm_chunk", ToNode: "tts",
				Cond: func(ev *dag.Event) bool {
					return ev.Data.(string) != "你好"
				}},
			{FromNode: "llm", OnEvent: "llm_chunk", ToNode: "output"},
			{FromNode: "tts", OnEvent: "tts_audio", ToNode: "output"},
		},
	}
	ctx := context.Background()
	session := dag.NewSession(ctx, dagModel, 1325153)
	// engine := dag.NewEngine(ctx, cancel, dagModel)
	// go func() {
	// 	// time.Sleep(200 * time.Millisecond)
	// 	engine.nodeInput["answer"] <- dag.Event{
	// 		Type: "user_input",
	// 		From: "external",
	// 		Data: "Tell me about Golang",
	// 	}
	// }()
	session.Dispatch("audio", "你好")
	session.Start()

	var wg sync.WaitGroup
	wg.Go(func() {
		for ev := range session.Output() {
			fmt.Println("FINAL OUTPUT:", ev.Data)
		}
	})
	wg.Wait()
	session.Close()
}
