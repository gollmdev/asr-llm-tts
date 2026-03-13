package node

import (
	"context"
	"log"
	"time"

	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
	tts "github.com/gollmdev/asr-llm-tts/ai/provider/ttsv2"
	"golang.org/x/sync/errgroup"
)

type TTSNode struct {
}

func (n *TTSNode) ID() string { return "tts" }
func (n *TTSNode) Mode() dag.NodeMode {
	return dag.ModeLazy
}

func (n *TTSNode) Run(
	// ctx context.Context,
	rt dag.NodeRuntime,
	// in <-chan dag.Event,
	// out chan<- dag.Event,
) error {

	ctx, cancel := context.WithCancel(rt.Context())
	defer func() {
		log.Println("tts cancel!")
		cancel()

	}()
	g, ctx := errgroup.WithContext(ctx)
	// messsage :=
	syn, err := tts.NewSpeechSynthesizer(
		"cosyvoice-v3-flash",
		"longwan_v3",
		tts.PCM_22050HZ_MONO_16BIT,
		g)

	if err != nil {
		return err
	}
	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case ev, ok := <-rt.Input():
				if !ok {
					if err := syn.StreamingComplete(ctx, 30*time.Second); err != nil {
						log.Println(err)
					}
					cancel()
					// close(l.done)
					log.Println(">> tss close session, tts is complete! ")
					return nil
				}
				text := ev.Data.(string)
				log.Println("Speech Synthesizer received chunk:", text)
				if err := syn.StreamingCall(ctx, text); err != nil {
					log.Println(err)
					cancel()
					return nil
				}
			}
		}
	})

	for {
		select {
		case <-ctx.Done():
			return nil // 如果返回error 会导致 startNode返回 error并退出，进而导致 dispatcher 退出
		case msg, ok := <-syn.Output():
			if !ok {
				log.Println("tts done")

				return nil
			}
			if msg.Event == "OnData" {
				rt.Emit(&dag.Event{Data: msg.Data, Type: "tts_audio"})
			}
		}
	}
}
