package ws

import (
	"context"
	// "gin-quickstart/pkg/ai/provider/asr"
	// "gin-quickstart/pkg/ai/provider/llm"
	// "gin-quickstart/pkg/ai/provider/tts"
	// "gin-quickstart/pkg/eventbus"
	"log"
	"strings"
	"sync"

	"github.com/golangllm/asr-llm-tts/ai/provider/asr"
	"github.com/golangllm/asr-llm-tts/ai/provider/llm"
	"github.com/golangllm/asr-llm-tts/ai/provider/tts"
	"github.com/golangllm/asr-llm-tts/eventbus"
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
	g           *errgroup.Group
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
	bus          *eventbus.Bus
	output       chan SessionMessage
	FullResponse strings.Builder
	// Done         chan struct{}
	ttsEnabled bool
	// consumers  map[string]func() // key -> cancel func
	consumers map[SessionName]*Consumer

	// // 新增：用于等待 runPipeline 退出
	// Done chan struct{}
}

func NewSession() *Session {
	ctx, cancel := context.WithCancel(context.Background())
	// gCtx, gCancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx) //  关键：绑定 context

	s := &Session{
		ctx:    ctx,
		cancel: cancel,
		g:      g,
		// gCtx:    gCtx,
		// gCancel: gCancel,
		// textOut:  make(chan string, 32),
		// ttsInput: make(chan string, 32),
		// audioOut: make(chan []byte, 32),
		bus:    eventbus.NewBus(1024),
		output: make(chan SessionMessage, 32),
		// Done:       make(chan struct{}),
		ttsEnabled: false,
		consumers:  make(map[SessionName]*Consumer),
		// textIn:  make(chan string, 32),
		// audioIn: make(chan []byte, 32),
		// done:    make(chan struct{}), // 标记 runPipeline 是否退出

	}

	// 启动处理协程
	// go s.runPipeline()
	return s
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

func (s *Session) Subscribe(name SessionName, types ...eventbus.EventType) (*eventbus.Subscriber, func()) {
	consumer, ok := s.consumers[name]
	if ok && consumer != nil {
		// consumer.unsubscribe()
		// s.UnSubscribe(name)
		if err := consumer.g.Wait(); err != nil {
			log.Printf("Consumer %s ended with error: %v\n", name, err)
		}
	}
	sub, unsubscribe_ := s.bus.Subscribe(s.ctx, eventbus.EventLLMChunk, eventbus.EventLLMDone)
	s.mu.Lock()
	s.consumers[name] = &Consumer{
		unsubscribe: unsubscribe_,
		g:           sub.G,
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

		if len(s.consumers) != 0 {
			for name := range s.consumers {
				delete(s.consumers, name)
			}
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
		s.bus.Publish(eventbus.Event{Type: eventbus.EventAudioChunk, Data: bytes})
	case 0x03:
		s.bus.Publish(eventbus.Event{Type: eventbus.EventAudioDone, Data: nil})
	case 0x04:
		s.cancel() // end session on audio done
	default:
		log.Printf("Unknown audio message type: %x", msg_type)
	}
}
func (s *Session) PublishTextStream(text string) {
	s.bus.Publish(eventbus.Event{Type: eventbus.EventTextChunk, Data: text})
	if s.ttsEnabled {
		s.TTsConsumer()
		log.Println("tts open!")
	}
}

func (s *Session) LLMConsumer() {
	sub, unsubscribe := s.bus.Subscribe(s.ctx, eventbus.EventTextChunk)
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
				LLMStream(message.Data.(string), s.ctx, func(chunk string) {
					// log.Println("LLMStream received chunk:", chunk)
					s.sendJson(SessionText, "message", chunk)
					s.FullResponse.WriteString(chunk)
					s.bus.Publish(eventbus.Event{Type: eventbus.EventLLMChunk, Data: chunk})
				})

				s.bus.Publish(eventbus.Event{Type: eventbus.EventLLMDone, Data: nil})
				// 获取完整响应（此时才转为 string）
				finalResponse := s.FullResponse.String()
				log.Println("Final LLM Response:", finalResponse)
				if !s.ttsEnabled {
					// close(s.Done)
					log.Println(">> llm close session, tts is not open! ")
				}
				// close(s.Done)
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
// 		s.bus.Publish(eventbus.Event{Type: eventbus.EventLLMChunk, Data: chunk})
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
// 	s.bus.Publish(eventbus.Event{Type: eventbus.EventLLMDone, Data: nil})
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
func LLMStream(inputText string, ctx context.Context, onChunkReceived func(string)) {
	// cb := &PrintHandler{}
	llm := llm.NewLLMStream(
		"qwen-plus",
		[]map[string]any{
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
		}, func(delta string) {
			onChunkReceived(delta)
		},
	)
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	// defer cancel()

	llm.Call(ctx, []map[string]any{
		{"role": "user", "content": inputText},
	})
}

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
	sub, unsubscribe := s.bus.Subscribe(s.ctx, eventbus.EventAudioChunk, eventbus.EventAudioDone)
	asrStream := asr.NewAsrStream(
		unsubscribe,
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
				s.cancel()
				return
			}

			// s.TTsConsumer()

			s.sendJson(SessionText, "asr_result", data)
			s.PublishTextStream(data)
			log.Println("AudioRecognitionConsumer complete data:", data)
		},
	)

	s.g.Go(func() error {
		defer unsubscribe()
		err := asrStream.Call(s.g, sub.Ctx)
		// 这个 error 被上层（errgroup.WithContext）接住后，统一 cancel 了共享的 context
		if err != nil {
			log.Println("ASRStream error:", err)
			return err
		}
		return nil

	})
}

func (s *Session) TTsConsumer() {

	sub, unsubscribe := s.Subscribe(TTS, eventbus.EventLLMChunk, eventbus.EventLLMDone)
	// longanyang longwan_v3 longanhuan
	// g, ctx := errgroup.WithContext(ctx)
	// s.addConsumer("tts", unsubscribe)
	ttsStream := tts.NewTtsStream(
		unsubscribe,
		sub.Ch,
		"cosyvoice-v3-flash",
		"longwan_v3",
		tts.PCM_22050HZ_MONO_16BIT,
		// s.Done,
		func(data []byte) {
			s.sendSafe(SessionAudio, data)
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
	msg := buildMessage(event, data)
	s.sendSafe(msgType, msg)
}

func (s *Session) sendSafe(msgType SessionMessageType, data []byte) {
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

func (s *Session) Close() {

	// _ = s.g.Wait() // 等待所有协程退出
	if err := s.g.Wait(); err != nil {
		log.Printf("Session ended with error: %v\n", err)
	} else {
		log.Println("All streams completed successfully")
	}
	// s.cancel()
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
