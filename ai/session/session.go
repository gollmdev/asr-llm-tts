package session

import (
	"context"
	"encoding/json"
	"io"

	// "gin-quickstart/pkg/ai/provider/asr"
	// "gin-quickstart/pkg/ai/provider/llm"
	// "gin-quickstart/pkg/ai/provider/tts"
	// "gin-quickstart/pkg/eventbus"
	"log"
	"strings"
	"sync"

	"github.com/gollmdev/asr-llm-tts/ai/event"
	"github.com/gollmdev/asr-llm-tts/ai/model"
	"github.com/gollmdev/asr-llm-tts/ai/provider/asr"
	"github.com/gollmdev/asr-llm-tts/ai/provider/llm"
	"github.com/gollmdev/asr-llm-tts/ai/provider/tts"

	"golang.org/x/sync/errgroup"
)

// text->llm->text，text->llm->(text and tts), asr->llm->text, asr->llm->(text and tts)
type SessionMessageType string
type SessionName string

const (
	LLM SessionName = "llm"
	TTS SessionName = "tts"
	ASR SessionName = "asr"
)

const (
	SessionText  SessionMessageType = "text"
	SessionAudio SessionMessageType = "audio"
)

type SessionMessage struct {
	Type SessionMessageType
	Data []byte
}

type Consumer struct {
	unsubscribe func()
	close       func()
	g           *errgroup.Group
}
type SessionConfig struct {
	TTSEnabled bool
}
type Session struct {
	// gCtx    context.Context
	// gCancel context.CancelFunc
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	g      *errgroup.Group

	// audioIn  chan []byte
	// textIn   chan string
	// ttsInput chan string
	// textOut  chan string
	// audioOut chan []byte
	bus          *Bus
	output       chan SessionMessage
	FullResponse strings.Builder
	// Done         chan struct{}
	SessionConfig *SessionConfig
	// consumers  map[string]func() // key -> cancel func
	consumers map[SessionName]*Consumer
	callback  Callback
	eventCtx  *EventContext

	// // 新增：用于等待 runPipeline 退出
	// Done chan struct{}
}
type EventContext struct {
	Ctx          context.Context
	Send         func(msgType SessionMessageType, data map[string]any)
	PublishEvent func(event event.Event)
}
type Callback interface {

	// OnEvent(eventType EventType, text string, publishMesaage func(message []map[string]any))
	// OnEvent(eventType EventType, text map[string]any, publishMesaage func(message []map[string]any))
	OnCitationsEvent(citations map[string]model.Citations)
	OnThoughtChainEvent(thoughtChain model.ThoughtChain)
	OnEvent(ctx *EventContext, eventType event.EventType, text string)
	// OnEventResult(ctx *EventContext, eventType EventType, text string)
	OnFinish()
	GetMessage(text string) []map[string]any
}

func NewSession(ctx context.Context, callback Callback, config *SessionConfig) *Session {
	// ctx, cancel := context.WithCancel(context.Background())
	baseCtx, cancel := context.WithCancel(ctx)
	// gCtx, gCancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(baseCtx) //  关键：绑定 context

	if config == nil {
		config = &SessionConfig{}
	}

	s := &Session{
		ctx:    ctx,
		cancel: cancel,
		g:      g,
		// gCtx:    gCtx,
		// gCancel: gCancel,
		// textOut:  make(chan string, 32),
		// ttsInput: make(chan string, 32),
		// audioOut: make(chan []byte, 32),
		bus:    NewBus(1024),
		output: make(chan SessionMessage, 32),
		// Done:       make(chan struct{}),
		// ttsEnabled: false,
		SessionConfig: config,
		consumers:     make(map[SessionName]*Consumer),
		// textIn:  make(chan string, 32),
		// audioIn: make(chan []byte, 32),
		// done:    make(chan struct{}), // 标记 runPipeline 是否退出
		callback: callback,
	}
	s.eventCtx = &EventContext{
		Ctx:          ctx,
		Send:         s.sendJsonMap,
		PublishEvent: s.PublishEvent,
	}
	// 启动处理协程
	// go s.runPipeline()
	return s
}

func (s *Session) Output() <-chan SessionMessage {
	return s.output
}

// func (s *Session) addConsumer(name string, unsubscribe func()) {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()

// 	if s.consumers == nil {
// 		s.consumers = make(map[string]func())
// 	}

// 	s.consumers[name] = unsubscribe
// }
// func (s *Session) CancelConsumer(name string) {
// s.mu.Lock()
// unsubscribe, ok := s.consumers[name]
// if ok {
// 	delete(s.consumers, name)
// }
// s.mu.Unlock()

// 	if ok && unsubscribe != nil {
// 		unsubscribe()
// 	}
// }

func (s *Session) Subscribe(name SessionName, types ...event.EventType) (*Subscriber, func()) {
	consumer, ok := s.consumers[name]
	if ok && consumer != nil {
		// consumer.unsubscribe()
		// s.UnSubscribe(name)
		if err := consumer.g.Wait(); err != nil {
			log.Printf("Consumer %s ended with error: %v\n", name, err)
		}
	}
	sub, unsubscribe_ := s.bus.Subscribe(s.ctx, event.EventLLMChunk, event.EventLLMDone)
	s.mu.Lock()
	s.consumers[name] = &Consumer{
		unsubscribe: unsubscribe_,
		g:           sub.G,
		close:       func() { close(sub.Ch) },
	}
	s.mu.Unlock()

	unsubscribe := func() {

		// s.mu.Lock()
		// delete(s.consumers, name)
		// s.mu.Unlock()
		// unsubscribe_()
		s.UnSubscribe(name)
	}
	return sub, unsubscribe
}
func (s *Session) UnSubscribe(name SessionName) {
	s.mu.Lock()
	consumer, ok := s.consumers[name]
	if ok && consumer != nil {
		consumer.unsubscribe()
	}

	// if ok {
	// 	delete(s.consumers, name)
	// }

	s.mu.Unlock()

}

func (s *Session) MonitorSubSize() {
	s.g.Go(func() error {
		for size := range s.bus.SubSize {
			log.Printf("Received zero subscribers signal, size: %d", size)
		}
		log.Println("ZeroSubscribers channel closed")

		close(s.output)

		// s.cancel()
		if len(s.consumers) != 0 {
			for name := range s.consumers {
				delete(s.consumers, name)
			}
		}
		if s.callback != nil {
			s.callback.OnFinish()
		}
		return nil
	})
	// go func() {
	// 	for size := range s.bus.ZeroSubscribers {
	// 		log.Printf("Received zero subscribers signal, size: %d", size)
	// 	}
	// 	log.Println("ZeroSubscribers channel closed")
	// }()
}

// func (s *Session) addConsumer(name string, unsubscribe func(), g *errgroup.Group) {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()

// 	if s.consumers == nil {
// 		s.consumers = make(map[string]*Consumer)
// 	}

// 	s.consumers[name] = &Consumer{
// 		unsubscribe: unsubscribe,
// 		g:           g,
// 	}
// }

// func (s *Session) CancelConsumer(name string) {
// 	s.mu.Lock()
// 	consumer, ok := s.consumers[name]
// 	if ok {
// 		delete(s.consumers, name)
// 	}
// 	s.mu.Unlock()

// 	if ok && consumer != nil {
// 		consumer.unsubscribe()
// 	}
// }

// func (s *Session) waitConsumers(name string) {
// 	s.mu.Lock()
// 	consumer, ok := s.consumers[name]
// 	s.mu.Unlock()

// 	if ok && consumer != nil {
// 		consumer.unsubscribe() // 先取消，触发协程退出
// 		if err := consumer.g.Wait(); err != nil {
// 			log.Printf("Consumer %s ended with error: %v\n", name, err)
// 		} else {
// 			log.Printf("Consumer %s completed successfully\n", name)
// 		}
// 	}
// }

// func (s *Session) RunPipeline() {
// 	for {
// 		select {
// 		// case t := <-s.textIn:
// 		// 	// 处理文本 → LLM → TextOut
// 		// 	// out := processLLM(t) // 假设返回 string
// 		// 	// s.output <- SessionMessage{Type: SessionText, Data: []byte(out)}

// 		// 	// // 可选 TTS
// 		// 	// audio := processTTS(out)
// 		// 	// if audio != nil {
// 		// 	// 	s.output <- SessionMessage{Type: SessionAudio, Data: audio}
// 		// 	// }
// 		// 	// go s.processLLMStream(t, s.ctx)
// 		// 	// go s.processLLMStream(t)
// 		// 	// time.Sleep(3 * time.Second)
// 		// 	// s.output <- SessionMessage{Type: SessionText, Data: []byte(t)}

// 		// 	return
// 		// case a := <-s.audioIn:

// 		// 	s.output <- SessionMessage{Type: SessionAudio, Data: a}

// 		case <-s.ctx.Done():
// 			// close(s.output)
// 			return
// 		}
// 	}
// }

// func (s *Session) processLLMStream(inputText string) {
// 	// 假设使用某种流式调用 LLM（例如 OpenAI stream API）
// 	// 模拟流式生成（可以根据实际情况进行实现）
// 	// LLM API 调用部分

// 	// 假设 `LLMStream` 是一个流式 API 返回的文本片段
// 	LLMStream(inputText, func(chunk string) {
// 		// 当 LLM 返回一个片段时，发送到 output
// 		s.output <- SessionMessage{
// 			Type: SessionText,
// 			Data: []byte(chunk),
// 		}
// 	})

// 	// 在 LLM 流式返回完毕后，还可以执行其他逻辑，比如 TTS 转换等
// }

func (s *Session) PublishBinaryStream(bytes []byte) {
	if len(bytes) == 0 {
		return
	}
	msg_type := bytes[0]
	log.Println("msg_type ", msg_type)
	switch msg_type {
	case 0x01:
		s.AudioRecognitionConsumer()
	case 0x02:
		bytes = bytes[1:]
		// log.Printf("Received binary message of length %d", len(bytes))
		s.bus.Publish(event.Event{Type: event.EventAudioChunk, Data: bytes})
	case 0x03:
		s.bus.Publish(event.Event{Type: event.EventAudioDone, Data: nil})
	case 0x04:
		s.cancel() // end session on audio done
	default:
		log.Printf("Unknown audio message type: %x", msg_type)
	}
}

// func (s *Session) PublishTextStream(text string) {
// 	s.bus.Publish(Event{Type: EventTextChunk, Data: text})
// 	if s.ttsEnabled {
// 		s.TTsConsumer()
// 		log.Println("tts open!")
// 	}
// }

func (s *Session) PublishEvent(event event.Event) {
	s.bus.Publish(event)
}

func (s *Session) LLMTaskConsumer() func() {
	sub, unsubscribe := s.bus.Subscribe(s.ctx, event.EventUserMessage, event.EventTitleGenerated)
	s.g.Go(func() error {
		defer func() {
			unsubscribe()
		}()
		for {
			select {
			case <-s.ctx.Done():
				return nil
			case message, ok := <-sub.Ch:
				if !ok {
					log.Println("LLMTaskConsumer completed")
					return nil
				}
				// log.Println("LLMTaskConsumer received message:", message.Data.(string))

				// s.LLMStream(message.Data.([]map[string]any), false, func(chunk string) {
				// 	// log.Println("LLMStream received chunk:", chunk)
				// 	// s.sendJson(SessionText, "message", chunk)
				// 	// s.FullResponse.WriteString(chunk)
				// 	// s.bus.Publish(Event{Type: EventLLMChunk, Data: chunk})
				// 	if s.callback != nil {
				// 		s.callback.OnEventResult(s.ctx, message.Type, chunk, func(msgType SessionMessageType, data map[string]any) {
				// 			s.sendJsonMap(msgType, data)
				// 		})
				// 	}
				// })
				llm := llm.NewQwenChatModel(&llm.ChatModelConfig{
					Model: "qwen-plus",
					Tools: []map[string]any{},
					Ctx:   s.ctx,
					G:     sub.G,
				})

				msg, err := llm.Generate(message.Data.([]map[string]any))
				if err != nil {
					log.Println("LLMStream error:", err)
				} else {
					if msg != nil && msg.Content != nil && s.callback != nil {
						s.callback.OnEvent(s.eventCtx, message.Type, *msg.Content)
					}
				}

				// s.sendJson(SessionText, "message", message.Data.(string))
			}
		}
	})
	return func() {
		close(sub.Ch)
	}

}

func (s *Session) LLMConsumer() {
	sub, unsubscribe := s.bus.Subscribe(s.ctx, event.EventTextChunk)
	close := s.LLMTaskConsumer()

	s.g.Go(func() error {
		defer func() {
			unsubscribe()
		}()

		for {
			select {
			case <-s.ctx.Done():
				return nil
			case message, ok := <-sub.Ch:
				if !ok {
					log.Println("TTSStream completed")
					return nil
				}
				if s.callback != nil {
					// s.callback.OnEvent(EventUserMessage, message.Data.(string), func(subMessage []map[string]any) {
					// 	s.bus.Publish(Event{Type: EventUserMessage, Data: subMessage})
					// })
					s.callback.OnEvent(s.eventCtx, event.EventUserMessage, message.Data.(string))
				}
				input := s.callback.GetMessage(message.Data.(string))
				// s.LLMStream(input, true, func(chunk string) {
				// 	// log.Println("LLMStream received chunk:", chunk)
				// 	s.sendJson(SessionText, "message", chunk)
				// 	s.FullResponse.WriteString(chunk)
				// 	s.bus.Publish(Event{Type: EventLLMChunk, Data: chunk})
				// })
				tools := []map[string]any{
					{
						"type": "function",
						"function": map[string]any{
							"name":        "get_weather",
							"description": "当你想查询指定城市的天气时非常有用。",
							"parameters": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"location": map[string]string{
										"type":        "string",
										"description": "城市或县区，比如北京市、杭州市、余杭区等。",
									},
								},
								"required": []string{"location"},
							},
						},
					},
				}
				llm := llm.NewQwenChatModel(&llm.ChatModelConfig{
					Model: "qwen-plus",
					Tools: tools,
					Ctx:   s.ctx,
					G:     sub.G,
				})

				llm.Stream(input)

				if s.callback != nil {
					defer log.Println("llm-life: llm recv done")
					for {
						msg, err := llm.Recv()
						if err != nil {
							if err != io.EOF {
								log.Println("LLMStream error:", err)
							}
							break
						}
						if msg != nil {
							if msg.Event == "OnText" && msg.Content != nil {
								// onChunkReceived(*msg.Content)
								chunk := *msg.Content
								s.sendJson(SessionText, "message", chunk)
								s.FullResponse.WriteString(chunk)
								s.bus.Publish(event.Event{Type: event.EventLLMChunk, Data: chunk})
							}
						}
					}

				}

				s.bus.Publish(event.Event{Type: event.EventLLMDone, Data: nil})
				// 获取完整响应（此时才转为 string）
				finalResponse := s.FullResponse.String()
				if s.callback != nil && finalResponse != "" {
					// s.callback.OnEventResult(s.ctx, EventLLMResponseComplete, finalResponse, func(msgType SessionMessageType, data map[string]any) {
					// 	s.sendJsonMap(msgType, data)
					// })
					// s.callback.OnEvent(EventLLMResponseComplete, finalResponse, func(subMessage []map[string]any) {
					// 	s.bus.Publish(Event{Type: EventUserMessage, Data: subMessage})
					// })
					s.callback.OnEvent(s.eventCtx, event.EventLLMResponseComplete, finalResponse)
					// mock data for citations and thought chain events
					s.callback.OnCitationsEvent(map[string]model.Citations{
						"12345679": {
							Title:   "cite1",
							Number:  1,
							ChunkID: "12345679",
						}, "89454131": {
							Title:   "cite2",
							Number:  2,
							ChunkID: "89454131",
						},
					})
					s.callback.OnThoughtChainEvent(model.ThoughtChain{
						Status: "success",
						Title:  "thought chain title",
						Items: []model.ThoughtItem{
							{
								Title:   "thought1",
								Content: "thought content 1",
							}, {
								Title:   "thought2",
								Content: "thought content 2",
							},
						},
					})
				}
				log.Println("Final LLM Response:", finalResponse)

				// if !s.ttsEnabled {
				// 	// close(s.Done)
				// 	log.Println(">> llm close session, tts is not open! ")
				// }
				// close(s.Done)

				close() // close llm task consumer
				log.Println("llm consumer close!")

				return nil
			}
		}

	})

	// go func() {
	// 	for {
	// 		select {
	// 		case size, ok := <-s.bus.ZeroSubscribers:
	// 			if ok {
	// 				log.Printf("Received zero subscribers signal for event type, size: %d", size)

	// 			}
	// 			return

	// 		}
	// 	}
	// }()
}

// func (s *Session) ProcessLLMStream(inputText string) {
// 	// 假设使用某种流式调用 LLM（例如 OpenAI stream API）
// 	// 模拟流式生成（可以根据实际情况进行实现）
// 	// LLM API 调用部分

// 	// 假设 `LLMStream` 是一个流式 API 返回的文本片段
// 	LLMStream(inputText, s.ctx, func(chunk string) {
// 		log.Println("LLMStream received chunk:", chunk)
// 		s.sendSafe(SessionText, []byte(chunk))
// 		s.FullResponse.WriteString(chunk)
// 		s.bus.Publish(Event{Type: EventLLMChunk, Data: chunk})
// 		// select {
// 		// case <-s.ctx.Done():
// 		// 	return
// 		// case s.ttsInput <- chunk:
// 		// }
// 		// select {
// 		// case <-s.ctx.Done(): // Check if the session context is done
// 		// 	log.Println("Close llm Stream!")
// 		// 	return
// 		// case s.output <- SessionMessage{
// 		// 	Type: SessionText,
// 		// 	Data: []byte(chunk),
// 		// }:
// 		// 	// Successfully sent data
// 		// }
// 	})
// 	// close(s.ttsInput)

// 	// 可选：检查上下文是否被取消
// 	// if s.ctx.Err() != nil {
// 	// 	log.Printf("Session %s cancelled, skipping DB save", s.ID)
// 	// 	return
// 	// }
// 	s.bus.Publish(Event{Type: EventLLMDone, Data: nil})
// 	// 获取完整响应（此时才转为 string）
// 	finalResponse := s.FullResponse.String()
// 	log.Println("Final LLM Response:", finalResponse)

//		// 存入数据库
//		// err := s.saveToDatabase(inputText, finalResponse)
//		// if err != nil {
//		// 	log.Printf("DB save failed for session %s: %v", s.ID, err)
//		// }
//		// s.cancel()
//		// 在 LLM 流式返回完毕后，还可以执行其他逻辑，比如 TTS 转换等
//	}
// func (s *Session) LLMStream(input []map[string]any, stream bool, onChunkReceived func(string)) {
// 	// cb := &PrintHandler{}
// 	// llm := llm.NewQwenChatModel(
// 	// 	"qwen-plus",
// 	// 	[]map[string]any{
// 	// {
// 	// 	"type": "function",
// 	// 	"function": map[string]any{
// 	// 		"name":        "get_weather",
// 	// 		"description": "当你想查询指定城市的天气时非常有用。",
// 	// 		"parameters": map[string]any{
// 	// 			"type": "object",
// 	// 			"properties": map[string]any{
// 	// 				"location": map[string]string{
// 	// 					"type":        "string",
// 	// 					"description": "城市或县区，比如北京市、杭州市、余杭区等。",
// 	// 				},
// 	// 			},
// 	// 			"required": []string{"location"},
// 	// 		},
// 	// 	},
// 	// },
// 	// 	}, func(delta string) {
// 	// 		onChunkReceived(delta)
// 	// 	},
// 	// )
// 	tools := []map[string]any{
// 		{
// 			"type": "function",
// 			"function": map[string]any{
// 				"name":        "get_weather",
// 				"description": "当你想查询指定城市的天气时非常有用。",
// 				"parameters": map[string]any{
// 					"type": "object",
// 					"properties": map[string]any{
// 						"location": map[string]string{
// 							"type":        "string",
// 							"description": "城市或县区，比如北京市、杭州市、余杭区等。",
// 						},
// 					},
// 					"required": []string{"location"},
// 				},
// 			},
// 		},
// 	}
// 	llm := llm.NewQwenChatModel(&llm.ChatModelConfig{
// 		Model: "qwen-plus",
// 		Tools: tools,
// 		Ctx:   s.ctx,
// 		G:     sub.G,
// 	})

// 	llm.Stream(input)

// 	if s.callback != nil {

// 		// llm.Call(s.ctx, input, stream)
// 		defer log.Println("llm-life: llm recv done")
// 		for {
// 			msg, err := llm.Recv()
// 			if err != nil {
// 				if err != io.EOF {
// 					log.Println("LLMStream error:", err)
// 				}
// 				break
// 			}
// 			if msg != nil {
// 				if msg.Event == "OnText" && msg.Content != nil {
// 					onChunkReceived(*msg.Content)
// 				}
// 			}
// 		}

// 	}
// 	// else {
// 	// 	llm.Call(ctx, []map[string]any{
// 	// 		{"role": "user", "content": inputText},
// 	// 	}, true)
// 	// }
// }

// func LLMStream2(inputText string, ctx context.Context, onChunkReceived func(string)) {
// 	// 假设这是流式调用 LLM API 并且逐步接收响应的实现
// 	// 这里可以是调用 OpenAI Stream API 或其他模型的流式响应

// 	// 模拟 LLM 输出逐步返回数据的过程
// 	parts := []string{"这是", "一些", "文本", "输出", "的", "示例"} // 假设这是 LLM 返回的逐步文本片段

//		for _, part := range parts {
//			select {
//			case <-ctx.Done():
//				// 如果收到取消信号，则停止流式调用
//				return
//			default:
//				// 模拟延迟并返回文本片段
//				time.Sleep(1 * time.Second)
//				onChunkReceived(part) // 返回每个文本片段
//				// log.Printf("LLMStream sent chunk: %s", part)
//			}
//		}
//		log.Println("LLMStream completed")
//	}
func (s *Session) AudioRecognitionConsumer() {
	sub, unsubscribe := s.bus.Subscribe(s.ctx, event.EventAudioChunk, event.EventAudioDone)
	asrStream := asr.NewAsrStream(
		// unsubscribe,
		sub.Ch,
		"paraformer-realtime-v2",
		"pcm",
		16000,
		// s.Done,
		func(data []byte) {
			// s.sendSafe(SessionText, data)
		},
		func(data string) {
			if data == "" {
				s.sendJson(SessionText, "no_asr_result", "未识别到内容, 请重试")
				s.cancel()

				return
			}

			// s.TTsConsumer()

			s.sendJson(SessionText, "asr_result", data)
			// s.PublishTextStream(data)
			s.PublishEvent(event.Event{
				Type: event.EventTextChunk,
				Data: data,
			})

			log.Println("AudioRecognitionConsumer complete data:", data)
		},
	)

	s.g.Go(func() error {
		defer unsubscribe()
		err := asrStream.Call(sub.G, sub.Ctx)
		// 这个 error 被上层（errgroup.WithContext）接住后，统一 cancel 了共享的 context
		if err != nil {
			log.Println("ASRStream error:", err)
			return err
		}
		return nil

	})
}

func (s *Session) TTsConsumer() {

	sub, unsubscribe := s.Subscribe(TTS, event.EventLLMChunk, event.EventLLMDone)
	// longanyang longwan_v3 longanhuan
	// g, ctx := errgroup.WithContext(ctx)
	// s.addConsumer("tts", unsubscribe)
	ttsStream := tts.NewTtsStream(
		// unsubscribe,
		sub.Ch,
		"cosyvoice-v3-flash",
		"longwan_v3",
		tts.PCM_22050HZ_MONO_16BIT,
		// s.Done,
		func(data []byte) {
			s.SendSafe(SessionAudio, data)
		},
	)
	sub.G.Go(func() error {
		// time.Sleep(5 * time.Second) // 确保 LLMConsumer 已经启动并订阅了事件

		defer func() {
			// unsubscribe()
			// 存在问题，重复启动TTsConsumer后，导致多个协程在等待同一个 unsubscribe，第一次取消后，其他协程继续等待，导致死锁
			// s.CancelConsumer("tts")
			unsubscribe()
		}()
		err := ttsStream.Call(sub.G, sub.Ctx)
		if err != nil {
			log.Println("TTSStream error:", err)
			return err
		}
		return nil

	})
}

// func (s *Session) RunProcessTTsStream() {
// 	s.g.Go(func() error {
// 		s.processTTsStream()
// 		return nil
// 	})
// }

// func (s *Session) processTTsStream(ch <-chan string) {
// 	for {
// 		select {
// 		case message, ok := <-s.ttsInput:
// 			if !ok {
// 				log.Println("TTSStream completed")

// 				return
// 			}
// 			log.Println("Start TTSStream for text:", message)
// 			TTSStream(message, s.ctx, func(chunk []byte) {
// 				s.sendSafe(SessionAudio, chunk)
// 				// select {
// 				// case <-s.ctx.Done():
// 				// 	return
// 				// case s.output <- chunk:
// 				// }
// 			})
// 		case <-s.ctx.Done():
// 			return
// 		}
// 	}
// }

// func TTSStream(text string, ctx context.Context, onChunkReceived func([]byte)) {
// 	// 假设这是流式调用 LLM API 并且逐步接收响应的实现
// 	// 这里可以是调用 OpenAI Stream API 或其他模型的流式响应

// 	// 模拟 LLM 输出逐步返回数据的过程
// 	parts := [][]byte{[]byte("PMC1"), []byte("PMC2")} // 假设这是 LLM 返回的逐步文本片段

// 	for _, part := range parts {
// 		select {
// 		case <-ctx.Done():
// 			// 如果收到取消信号，则停止流式调用
// 			return
// 		default:
// 			// 模拟延迟并返回文本片段
// 			time.Sleep(1 * time.Second)
// 			onChunkReceived(part) // 返回每个文本片段
// 			// log.Printf("LLMStream sent chunk: %s", part)
// 		}
// 	}
// }

func (s *Session) sendJson(msgType SessionMessageType, event string, data string) {
	msg := BuildMessage(event, data)
	s.SendSafe(msgType, msg)
}

func (s *Session) sendJsonMap(msgType SessionMessageType, data map[string]any) {
	// msg := buildMessage(event, data)
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		log.Println("json marshal error:", err)
	}
	s.SendSafe(msgType, jsonBytes)
}
func (s *Session) SendSafe(msgType SessionMessageType, data []byte) {
	select {
	case s.output <- SessionMessage{Type: msgType, Data: data}:
	case <-s.ctx.Done():
		// session 已关闭，丢弃消息
	}
}

// func (s *Session) PushText(text string) {
// 	select {
// 	case s.textIn <- text:
// 	case <-s.ctx.Done():
// 	}
// }

// func (s *Session) PushAudio(audio []byte) {
// 	select {
// 	case s.audioIn <- audio:
// 	case <-s.ctx.Done():
// 	}
// }
// func (s *Session) TryPushAudio(audio []byte) bool {
// 	select {
// 	case s.audioIn <- audio:
// 		return true
// 	case <-s.ctx.Done():
// 		return false
// 	default:
// 		return false // channel full
// 	}
// }

// func (s *Session) Output() <-chan SessionMessage {
// 	return s.output
// }

// func (s *Session) Recv() (*SessionMessage, error) {
// 	msg, ok := <-s.output
// 	if !ok {
// 		return nil, io.EOF
// 	}
// 	return &msg, nil

// }
func (s *Session) Close() {

	// _ = s.g.Wait() // 等待所有协程退出
	if err := s.g.Wait(); err != nil {
		log.Printf("Session ended with error: %v\n", err)
	} else {
		log.Println("All streams completed successfully")
	}
	s.cancel()
	// close(s.output)
}

// <-s.done
// close(s.textIn)
// close(s.audioIn)
// close(s.output)

// func NewSession(asr asr.ASRProvider, llm llm.LLMProvider, tts tts.TTSProvider) *Session {
// 	ctx, cancel := context.WithCancel(context.Background())

// 	audioIn := make(chan []byte)

// 	asrText, _ := asr.Stream(ctx, audioIn)

// 	llmTextIn := make(chan string)
// 	llmText, _ := llm.Stream(ctx, llmTextIn)

// 	ttsAudio, _ := tts.Stream(ctx, llmText)

// 	// ASR → LLM
// 	go func() {
// 		defer close(llmTextIn)
// 		for t := range asrText {
// 			llmTextIn <- t
// 		}
// 	}()

// 	return &Session{
// 		ctx:      ctx,
// 		cancel:   cancel,
// 		audioIn:  audioIn,
// 		textOut:  llmText,
// 		audioOut: ttsAudio,
// 	}
// }
