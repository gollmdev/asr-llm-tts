package node

import (
	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
)

type ASRNode struct{}

func (n *ASRNode) ID() string { return "asr" }
func (n *ASRNode) Mode() dag.NodeMode {
	return dag.ModeLazy
}

func (n *ASRNode) Run(
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
			text, ok := ev.Data.(string)
			if !ok {
				// return nil
				continue
			}
			rt.Emit(&dag.Event{
				Type: "asr_test",
				From: n.ID(),
				Data: "音频转文本:[ " + text + " ]",
			})
			return nil
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
