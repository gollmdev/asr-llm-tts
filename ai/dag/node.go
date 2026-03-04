package dag

import (
	"context"
	"log"
)

type AnswerNode struct{}

func (n *AnswerNode) ID() string { return "answer" }
func (n *AnswerNode) Mode() NodeMode {
	return ModeAlwaysOn
}
func (n *AnswerNode) Run(
	ctx context.Context,
	in <-chan Event,
	out chan<- Event,
) error {

	for {
		select {
		case ev, ok := <-in:
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
				out <- Event{
					Type: "llm_chunk",
					From: n.ID(),
					Data: chunk,
				}

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
	ctx context.Context,
	in <-chan Event,
	out chan<- Event,
) error {

	for {
		select {
		case ev, ok := <-in:
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
				out <- Event{
					Type: "tts_audio",
					From: n.ID(),
					Data: chunk,
				}

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

type OutputNode struct {
	// id     string
	Output chan Event
}

func NewOutputNode(id string) *OutputNode {
	return &OutputNode{
		// id:     id,
		Output: make(chan Event, 32),
	}
}
func (n *OutputNode) Mode() NodeMode {
	return ModeLazy
}
func (n *OutputNode) ID() string { return "output" }

func (n *OutputNode) Run(
	ctx context.Context,
	in <-chan Event,
	out chan<- Event,
) error {

	for {
		select {
		case ev, ok := <-in:
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

func (n *OutputNode) InputTypes() []string {
	return []string{"tts_audio"}
}
func (n *OutputNode) OutputTypes() []string {
	return []string{"test"}
}
