package node

import (
	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
)

type TTSNode struct{}

func (n *TTSNode) ID() string { return "tts" }
func (n *TTSNode) Mode() dag.NodeMode {
	return dag.ModeAlwaysOn
}

func (n *TTSNode) Run(
	// ctx context.Context,
	rt dag.NodeRuntime,
	// in <-chan dag.Event,
	// out chan<- dag.Event,
) error {
	ctx := rt.Context()

	for {
		select {
		case ev, ok := <-rt.Input():
			if !ok {
				return nil
			}
			text := ev.Data.(string)
			rt.Emit(&dag.Event{
				Type: "tts_audio",
				From: n.ID(),
				Data: "audio: [" + text + "]",
			})
			// audio := callTTS(text)
			// audio := []string{
			// 	text + "+ tts part 1",
			// 	text + "+ tts part 2",
			// 	text + "+ tts part 3",
			// }
			// for _, chunk := range audio {
			// 	out <- dag.Event{
			// 		Type: "tts_audio",
			// 		From: n.ID(),
			// 		Data: chunk,
			// 	}

			// }
			// close(in)
			// return nil
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
