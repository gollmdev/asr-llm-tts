package node

import (
	"context"
	"errors"
	"log"
	"time"

	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
	"github.com/gollmdev/asr-llm-tts/ai/dag/dagtypes"
	asr "github.com/gollmdev/asr-llm-tts/ai/provider/asrv2"
	"golang.org/x/sync/errgroup"
)

type ASRNode struct{}

func (n *ASRNode) ID() string { return "asr" }
func (n *ASRNode) Mode() dag.NodeMode {
	return dag.ModeLazy
}

func (n *ASRNode) Run(
	rt dag.NodeRuntime,
) error {
	ctx, cancel := context.WithCancel(rt.Context())
	defer func() {
		log.Println("asr cancel!")
		cancel()

	}()
	g, ctx := errgroup.WithContext(ctx)
	recognition, err := asr.NewRecognition(
		"paraformer-realtime-v2",
		"pcm",
		16000,
		ctx,
		g,
	)
	if err != nil {
		return err
	}

	if err := recognition.StartStream(); err != nil {
		return err
	}
	silenceTimeout := 23 * time.Second
	g.Go(func() error {
		timer := time.NewTimer(silenceTimeout)
		defer func() {
			timer.Stop()

		}()

		for {
			select {
			case <-ctx.Done():

				log.Println("AsrStream context done")
				return nil
			case <-timer.C:
				// log.Println("AsrStream timeout")
				return errors.New("asr stream timeout")
			case ev, ok := <-rt.Input():
				if !ok {

					return nil
				}
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(silenceTimeout)

				switch ev.Type {
				case "audio":

					audioData, ok := ev.Data.([]byte)
					if !ok {
						continue
					}
					// log.Printf("AsrStream received binary message of length %d", len(audioData))
					err := recognition.StreamingCall(audioData)
					if err != nil {
						return err
					}
				case "audio_end":
					err := recognition.StreamingComplete(30 * time.Second)
					if err != nil {
						log.Println("AsrStream error:", err)
					}
					return nil

				}

			}
		}
	})
	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				// for range recognition.Output() {
				// }
				recognition.Close()
				log.Println("AsrStream output context done")
				return nil // 如果返回error 会导致 startNode返回 error并退出，进而导致 dispatcher 退出
			case msg, ok := <-recognition.Output():
				if !ok {
					log.Println("asr done")

					return nil
				}
				if msg.Event == "OnComplete" {
					if msg.Data == "" {
						log.Println("asr result is empty")
						rt.Emit(&dag.Event{From: n.ID(), Type: "no_asr_text"})
						return nil
					}
					msg := []*dagtypes.Message{
						{
							Role:    "user",
							Content: msg.Data,
						},
					}
					rt.Emit(&dag.Event{From: n.ID(), Data: msg, Type: "asr_text"})
				}
			}
		}
	})
	return g.Wait()

}
