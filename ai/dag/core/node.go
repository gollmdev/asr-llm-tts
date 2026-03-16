package dag

import (
	"context"
	"fmt"
	"log"
)

type AnswerNode struct{}

func (n *AnswerNode) ID() string { return "llm" }
func (n *AnswerNode) Mode() NodeMode {
	return ModeAlwaysOn
}
func (n *AnswerNode) Run(
	// ctx context.Context,
	rt NodeRuntime,
	// in <-chan Event,
	// out chan<- Event,
) error {
	ctx := rt.Context()

	for {
		select {
		case ev, ok := <-rt.Input():
			if !ok {
				return nil
			}
			dbData := ev.Data.(string)
			// mock answer stream with
			stream := []string{
				dbData + " + llm answer part 1",
				dbData + " + llm answer part 2",
				dbData + " + llm answer part 3",
			}
			for _, chunk := range stream {
				rt.Emit(&Event{
					Type: "llm_chunk",
					From: n.ID(),
					Data: chunk,
				})

			}
			return nil
			// for chunk := range stream {
			// 	out <- Event{
			// 		Type: "llm_chunk",
			// 		From: n.ID(),
			// 		Data: chunk,
			// 	}
			// }

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (n *AnswerNode) InputTypes() []string {
	return []string{"db_result"}
}
func (n *AnswerNode) OutputTypes() []string {
	return []string{"llm_chunk"}
}

type TTSNode struct{}

func (n *TTSNode) ID() string { return "tts" }
func (n *TTSNode) Mode() NodeMode {
	return ModeLazy
}

func (n *TTSNode) Run(
	// ctx context.Context,
	rt NodeRuntime,
	// in <-chan Event,
	// out chan<- Event,
) error {

	ctx := rt.Context()
	for {
		select {
		case ev, ok := <-rt.Input():
			if !ok {
				return nil
			}
			text := ev.Data.(string)

			// audio := callTTS(text)
			audio := []string{
				text + "+ tts part 1",
				text + "+ tts part 2",
				text + "+ tts part 3",
			}
			for _, chunk := range audio {
				rt.Emit(&Event{
					Type: "tts_audio",
					From: n.ID(),
					Data: chunk,
				})

			}
			// close(in)
			return nil
			// out <- Event{
			// 	Type: "tts_audio",
			// 	From: n.ID(),
			// 	Data: audio,
			// }

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (n *TTSNode) InputTypes() []string {
	return []string{"llm_chunk"}
}
func (n *TTSNode) OutputTypes() []string {
	return []string{"tts_audio"}
}

type OutputNodeTest struct {
	// id     string
	Output chan *Event
}

func NewOutputNodeTest(id string) *OutputNodeTest {
	return &OutputNodeTest{
		// id:     id,
		Output: make(chan *Event, 32),
	}
}
func (n *OutputNodeTest) Mode() NodeMode {
	return ModeLazy
}
func (n *OutputNodeTest) ID() string { return "output" }

func (n *OutputNodeTest) Run(
	// ctx context.Context,
	rt NodeRuntime,
) error {
	ctx := rt.Context()
	for {
		select {
		case ev, ok := <-rt.Input():
			if !ok {
				log.Println("output close!")
				return nil
			}
			// text := ev.Data.(string)
			n.Output <- ev
			log.Printf("OutputNode received text: %s", ev.Data)

			// // audio := callTTS(text)
			// audio := []string{
			// 	text + "这是根据你的问题和数据库内容生成的回答，第一部分。",
			// 	text + "这是根据你的问题和数据库内容生成的回答，第二部分。",
			// 	text + "这是根据你的问题和数据库内容生成的回答，第三部分。",
			// }
			// out <- Event{
			// 	Type: "tts_audio",
			// 	From: n.ID(),
			// 	Data: audio,
			// }

		case <-ctx.Done():
			close(n.Output)
			return ctx.Err()
		}
	}
}

func (n *OutputNodeTest) InputTypes() []string {
	return []string{"tts_audio"}
}
func (n *OutputNodeTest) OutputTypes() []string {
	return []string{"test"}
}

func Test() {
	outputNode := NewOutputNodeTest("final")

	dag := &DAG{
		Nodes: map[string]Node{
			"answer": &AnswerNode{},
			"tts":    &TTSNode{},
			"output": outputNode,
		},
		Edges: []Edge{
			{FromNode: "keyword", OnEvent: "keyword_done", ToNode: "db"},
			// {FromNode: "db", OnEvent: "db_result", ToNode: "answer"},
			{FromNode: "answer", OnEvent: "llm_chunk", ToNode: "tts"},
			{FromNode: "tts", OnEvent: "tts_audio", ToNode: "output"},
		},
	}
	ctx := context.Background()
	engine := NewEngine(ctx, dag, &RuntimeContext{
		SessionID: 12346,
	})
	go func() {
		// time.Sleep(200 * time.Millisecond)
		engine.nodeInput["answer"] <- &Event{
			Type: "user_input",
			From: "external",
			Data: "Tell me about Golang",
		}
	}()
	go func() {
		for ev := range outputNode.Output {
			fmt.Println("FINAL OUTPUT:", ev.Data)
		}
	}()
	engine.Start()
	engine.Close()
}
