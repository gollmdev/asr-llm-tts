package asr

import (
	"context"
	"errors"

	// "gin-quickstart/pkg/eventbus"
	"log"
	"os"
	"time"

	"github.com/golangllm/asr-llm-tts/eventbus"
	"golang.org/x/sync/errgroup"
)

type AsrStream struct {
	model       string
	format      string
	sampleRate  int
	apiKey      string
	unsubscribe func()
	ch          <-chan eventbus.Event
	// done        chan struct{}
	callback RecognitionCallback
}

type Callback struct {
	onDataFunc     func(data []byte)
	onCompleteFunc func(data string)
}

func (c *Callback) OnOpen() {}
func (c *Callback) OnComplete(result string) {
	if c.onCompleteFunc != nil {
		c.onCompleteFunc(result)
	}
}
func (c *Callback) OnError(result *RecognitionResult) {

}
func (c *Callback) OnClose() {}
func (c *Callback) OnEvent(result *RecognitionResult) {

}

func NewAsrStream(unsubscribe func(),
	ch <-chan eventbus.Event,
	model,
	format string,
	sampleRate int,
	// done chan struct{},
	onDataFunc func(data []byte),
	onCompleteFunc func(data string),
) *AsrStream {
	return &AsrStream{
		apiKey:      os.Getenv("DASHSCOPE_API_KEY"),
		model:       model,
		unsubscribe: unsubscribe,
		ch:          ch,
		// done:        done,
		format:     format,
		sampleRate: sampleRate,

		callback: &Callback{onDataFunc: onDataFunc, onCompleteFunc: onCompleteFunc},
	}
}

func (s *AsrStream) Call(g *errgroup.Group, ctx context.Context) error {
	// ticker := time.NewTicker(10 * time.Second)
	timer := time.NewTimer(silenceTimeout)

	defer timer.Stop()
	recognition, err := NewRecognition(
		s.apiKey,
		s.model,
		s.format,
		s.sampleRate,
		s.callback,
		g,
	)
	if err != nil {
		return err
	}
	// start recognition stream
	if err := recognition.StartStream(ctx); err != nil {
		return err
	}
	log.Println("AsrStream started, waiting for audio data...")

	for {
		select {
		case <-ctx.Done():
			recognition.Close()
			log.Println("Context done, exiting asr warp readLoop.")
			return nil
		case <-timer.C:
			// log.Println("AsrStream timeout")
			return errors.New("asr stream timeout")
		case event, ok := <-s.ch:
			if !ok {
				log.Println("AsrStream error!")
				return nil
			}
			timer.Reset(silenceTimeout)
			switch event.Type {
			case eventbus.EventAudioChunk:
				audioData, ok := event.Data.([]byte)
				if !ok {
					continue
				}
				log.Printf("AsrStream received binary message of length %d", len(audioData))
				err := recognition.StreamingCall(audioData)
				// _ = audioData
				if err != nil {
					return err
				}
			case eventbus.EventAudioDone:
				err := recognition.StreamingComplete(30 * time.Second)

				if err != nil {
					return err
				}
				// close(s.done)
				return nil

				// case eventbus.EventLLMDone:
				// 	err := recognition.StreamingComplete(30 * time.Second)
				// 	if err != nil {
				// 		return err
				// 	}
				// 	return nil
			}

		}

	}

}
