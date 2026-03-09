package tts

import (
	"context"
	"time"

	// "gin-quickstart/pkg/eventbus"
	"log"
	"os"

	"github.com/gollmdev/asr-llm-tts/ai/event"
	"golang.org/x/sync/errgroup"
)

// type Callback struct {
// 	onDataFunc func(data []byte)
// }

// func (c *Callback) OnOpen()            { log.Println("open") }
// func (c *Callback) OnComplete()        { log.Println("tts complete") }
// func (c *Callback) OnError(msg string) { log.Println("error:", msg) }
// func (c *Callback) OnClose()           { log.Println("close") }
// func (c *Callback) OnEvent(msg string) {
// 	// log.Println("event:", msg)
// }
// func (c *Callback) OnData(data []byte) {
// 	c.onDataFunc(data)
// 	// log.Printf("audio chunk: %d bytes\n", len(data))
// }

type StreamAudioMessage struct {
	Event string
	Data  []byte
	Err   error
}
type TtsStream struct {
	apiKey string
	model  string
	voice  string
	fmt    AudioFormat
	// unsubscribe func()
	ch <-chan event.Event
	// done        chan struct{}
	callback ResultCallback
	message  chan *StreamAudioMessage
}

func NewTtsStream(
	ch <-chan event.Event,
	model, voice string, fmt AudioFormat,
	// done chan struct{},
	onDataFunc func(data []byte)) *TtsStream {
	return &TtsStream{
		apiKey: os.Getenv("DASHSCOPE_API_KEY"),
		model:  model,
		voice:  voice,
		fmt:    fmt,
		// unsubscribe: unsubscribe,
		ch:      ch,
		message: make(chan *StreamAudioMessage, 1),
		// done:        done,
		// callback: &Callback{onDataFunc: onDataFunc},

	}
}

func (l *TtsStream) Call(g *errgroup.Group, ctx context.Context) error {
	// apiKey := os.Getenv("DASHSCOPE_API_KEY")
	// model := "cosyvoice-v3-flash"
	// voice := "longanyang"

	// fmt := tts.AudioFormatDefault

	// headers := http.Header{} // add custom headers if needed
	// additional := map[string]any{"enable_ssml": true}
	// cb := &Callback{}
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()

	}()
	syn, err := NewSpeechSynthesizer(
		l.model,
		l.voice,
		l.fmt,
		// l.message,
		// l.callback,
		g)

	if err != nil {
		log.Println(err)
		return err
	}
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()

	for {
		select {
		case <-ctx.Done():
			// syn.conn.Close()
			// cancel()
			log.Println("Context done, exiting tts warp readLoop.")
			return nil
		case message, ok := <-l.ch:
			if !ok {
				if err := syn.StreamingComplete(ctx, 30*time.Second); err != nil {
					log.Println(err)
				}
				// close(l.done)
				log.Println(">> tss close session, tts is complete! ")
				// cancel()
				log.Println("TTSStream event channel closed")
				return nil
			}
			text := message.Data.(string)
			log.Println("Speech Synthesizer received chunk:", text)
			if err := syn.StreamingCall(ctx, text); err != nil {
				log.Println(err)
				// cancel()
				return nil
			}
			// log.Println("Start TTSStream for event:", message.Data.(string))
			// switch message.Type {
			// case event.EventLLMChunk:

			// 	// TTSStream(text, s.ctx, func(chunk []byte) {
			// 	// 	s.sendSafe(SessionAudio, chunk)
			// 	// })
			// case event.EventLLMDone:

			// 	return nil
			// }

		}
	}
}
