package tts

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

// --- Protocol constants (mirroring dashscope.protocol.websocket) ---
const (
	headerKey       = "header"
	actionKey       = "action"
	eventKey        = "event"
	taskIDKey       = "task_id"
	streamingKey    = "streaming"
	streamingDuplex = "duplex"
)

type actionType string

const (
	actionStart    actionType = "start"
	actionContinue actionType = "continue"
	actionFinished actionType = "finished"
)

type eventType string

const (
	eventStarted   eventType = "task-started"
	eventFinished  eventType = "task-finished"
	eventFailed    eventType = "task-failed"
	eventGenerated eventType = "result-generated"
)

// ResultCallback defines hooks for receiving synthesis results.
type ResultCallback interface {
	OnOpen()
	OnComplete()
	OnError(message string)
	OnClose()
	OnEvent(message string)
	OnData(data []byte)
}

// AudioFormat represents desired output format.
type AudioFormat struct {
	Format     string // mp3, wav, pcm, opus
	SampleRate int
	Channels   string // mono
	BitRate    int    // for opus/mp3
}

var (
	// Common presets (subset of Python enum)
	AudioFormatDefault            = AudioFormat{Format: "mp3", SampleRate: 22050, Channels: "mono", BitRate: 256}
	AudioFormatWAV16000Mono16bit  = AudioFormat{Format: "wav", SampleRate: 16000, Channels: "mono", BitRate: 16}
	AudioFormatMP316000Mono128kbp = AudioFormat{Format: "mp3", SampleRate: 16000, Channels: "mono", BitRate: 128}
	AudioFormatOpus24000Mono32kbp = AudioFormat{Format: "opus", SampleRate: 24000, Channels: "mono", BitRate: 32}
	PCM_22050HZ_MONO_16BIT        = AudioFormat{Format: "pcm", SampleRate: 22050, Channels: "mono", BitRate: 16}
)

// Request builds websocket messages for the speech synthesizer.
// type Request struct {
// 	TaskID        string
// 	APIKey        string
// 	Voice         string
// 	Model         string
// 	Format        string
// 	SampleRate    int
// 	BitRate       int
// 	Volume        int
// 	SpeechRate    float64
// 	PitchRate     float64
// 	Seed          int
// 	SynthesisType int
// 	Instruction   *string
// 	LanguageHints []string
// }

// func NewRequest(apiKey, model, voice string, fmt AudioFormat, volume int, speechRate, pitchRate float64, seed, synthType int, instruction *string, languageHints []string) *Request {
// 	return &Request{
// 		TaskID:        newUID(),
// 		APIKey:        apiKey,
// 		Voice:         voice,
// 		Model:         model,
// 		Format:        fmt.Format,
// 		SampleRate:    fmt.SampleRate,
// 		BitRate:       fmt.BitRate,
// 		Volume:        volume,
// 		SpeechRate:    speechRate,
// 		PitchRate:     pitchRate,
// 		Seed:          seed,
// 		SynthesisType: synthType,
// 		Instruction:   instruction,
// 		LanguageHints: languageHints,
// 	}
// }

// func (r *Request) startPayload() map[string]any {

// 	runTaskCmd := map[string]interface{}{
// 		"header": map[string]interface{}{
// 			"action":    "run-task",
// 			"task_id":   r.TaskID,
// 			"streaming": "duplex",
// 		},
// 		"payload": map[string]interface{}{
// 			"task_group": "audio",
// 			"task":       "tts",
// 			"function":   "SpeechSynthesizer",
// 			"model":      r.Model,
// 			"parameters": map[string]interface{}{
// 				"text_type":   "PlainText",
// 				"voice":       r.Voice,
// 				"format":      r.Format,
// 				"sample_rate": r.SampleRate,
// 				"volume":      r.Volume,
// 				"rate":        r.BitRate,
// 				"pitch":       r.PitchRate,
// 				// 如果enable_ssml设为true，只允许发送一次continue-task指令，否则会报错“Text request limit violated, expected 1.”
// 				"enable_ssml": r.SynthesisType,
// 			},
// 			"input": map[string]interface{}{},
// 		},
// 	}
// 	return runTaskCmd
// 	// params := map[string]any{
// 	// 	"voice":       r.Voice,
// 	// 	"volume":      r.Volume,
// 	// 	"text_type":   "PlainText",
// 	// 	"sample_rate": r.SampleRate,
// 	// 	"rate":        r.SpeechRate,
// 	// 	"format":      r.Format,
// 	// 	"pitch":       r.PitchRate,
// 	// 	"seed":        r.Seed,
// 	// 	"type":        r.SynthesisType,
// 	// }
// 	// if r.Format == "opus" {
// 	// 	params["bit_rate"] = r.BitRate
// 	// }
// 	// if r.Instruction != nil {
// 	// 	params["instruction"] = *r.Instruction
// 	// }
// 	// if len(r.LanguageHints) > 0 {
// 	// 	params["language_hints"] = r.LanguageHints
// 	// }
// 	// for k, v := range additional {
// 	// 	params[k] = v
// 	// }
// 	// return map[string]any{
// 	// 	headerKey: map[string]any{
// 	// 		actionKey:    actionStart,
// 	// 		taskIDKey:    r.TaskID,
// 	// 		streamingKey: streamingDuplex,
// 	// 	},
// 	// 	"payload": map[string]any{
// 	// 		"model":      r.Model,
// 	// 		"task_group": "audio",
// 	// 		"task":       "tts",
// 	// 		"function":   "SpeechSynthesizer",
// 	// 		"input":      map[string]any{},
// 	// 		"parameters": params,
// 	// 	},
// 	// }
// }

// func (r *Request) continuePayload(text string) map[string]any {
// 	return map[string]any{
// 		headerKey: map[string]any{
// 			actionKey:    actionContinue,
// 			taskIDKey:    r.TaskID,
// 			streamingKey: streamingDuplex,
// 		},
// 		"payload": map[string]any{
// 			"model":      r.Model,
// 			"task_group": "audio",
// 			"task":       "tts",
// 			"function":   "SpeechSynthesizer",
// 			"input": map[string]any{
// 				"text": text,
// 			},
// 		},
// 	}
// }

// func (r *Request) finishPayload() map[string]any {
// 	return map[string]any{
// 		headerKey: map[string]any{
// 			actionKey:    actionFinished,
// 			taskIDKey:    r.TaskID,
// 			streamingKey: streamingDuplex,
// 		},
// 		"payload": map[string]any{
// 			"input": map[string]any{},
// 		},
// 	}
// }

//	func newUID() string {
//		return strings.ReplaceAll(time.Now().Format("20060102150405.000000000"), ".", "")
//	}
//
// SpeechSynthesizer encapsulates the websocket session.
type SpeechSynthesizer struct {
	// immutable setup
	url       string
	apiKey    string
	headers   http.Header
	workspace string

	// request params
	model string
	voice string
	// aformat     string
	// sampleRate  int
	aformat     AudioFormat
	volume      int
	speechRate  float64
	pitchRate   float64
	seed        int
	synthType   int
	instruction *string
	langHints   []string
	additional  map[string]any

	// runtime
	conn       *websocket.Conn
	dialer     *websocket.Dialer
	startCh    chan struct{}
	completeCh chan struct{}
	closed     chan struct{}
	isStarted  bool
	audioBuf   []byte
	// asyncCall       bool
	// callback        ResultCallback
	lastResponse    map[string]any
	lastRequestID   string
	closeWSAfterUse bool

	// timing
	startStreamTS   int64
	firstPkgTS      int64
	recvAudioMillis float64
	g               *errgroup.Group
	taskID          string
	message         chan *StreamAudioMessage
}

// NewSpeechSynthesizer creates a synthesizer.
func NewSpeechSynthesizer(
	model, voice string,
	fmt AudioFormat,
	// volume int,
	// speechRate,
	// pitchRate float64,
	// seed,
	// synthType int,
	// instruction *string,
	// languageHints []string,
	// headers http.Header,
	// message chan *StreamAudioMessage,
	// callback ResultCallback,
	g *errgroup.Group,
	// workspace string,
	// additional map[string]any
) (*SpeechSynthesizer, error) {
	if model == "" {
		return nil, errors.New("model is required")
	}
	if fmt.Format == "" {
		return nil, errors.New("format is required")
	}
	// if url == "" {
	// 	return nil, errors.New("url is required")
	// }
	// if apiKey == "" {
	// 	return nil, errors.New("apikey is required")
	// }

	// af := fmt.Format
	// if strings.ToLower(af) == "default" {
	// 	af = "mp3"
	// }
	// sr := fmt.SampleRate
	// if sr == 0 {
	// 	sr = 22050
	// }

	s := &SpeechSynthesizer{
		url:       "wss://dashscope.aliyuncs.com/api-ws/v1/inference/",
		apiKey:    os.Getenv("DASHSCOPE_API_KEY"),
		headers:   http.Header{}, //headers.Clone(), headers := http.Header{}
		workspace: "",
		model:     model,
		voice:     voice,
		aformat:   fmt,
		// sampleRate:      sr,
		volume:      50,
		speechRate:  1.0,
		pitchRate:   1.0,
		seed:        0,
		synthType:   0,
		instruction: nil,
		langHints:   nil,
		additional:  nil,
		dialer:      &websocket.Dialer{HandshakeTimeout: 5 * time.Second},
		startCh:     make(chan struct{}, 1),
		completeCh:  make(chan struct{}, 1),
		closed:      make(chan struct{}, 1),
		// asyncCall:       callback != nil,
		// callback:        callback,
		message:         make(chan *StreamAudioMessage, 1),
		closeWSAfterUse: true,
		taskID:          uuid.NewString(),
		g:               g,
	}
	return s, nil
}

func (r *SpeechSynthesizer) startPayload() map[string]any {

	runTaskCmd := map[string]interface{}{
		"header": map[string]interface{}{
			"action":    "run-task",
			"task_id":   r.taskID,
			"streaming": "duplex",
		},
		"payload": map[string]interface{}{
			"task_group": "audio",
			"task":       "tts",
			"function":   "SpeechSynthesizer",
			"model":      r.model,
			"parameters": map[string]interface{}{
				"text_type":   "PlainText",
				"voice":       r.voice,
				"format":      r.aformat.Format,
				"sample_rate": r.aformat.SampleRate,
				"volume":      r.volume,
				"rate":        1.1,
				"pitch":       r.pitchRate,
				// 如果enable_ssml设为true，只允许发送一次continue-task指令，否则会报错“Text request limit violated, expected 1.”
				"enable_ssml": r.synthType,
			},
			"input": map[string]interface{}{},
		},
	}
	return runTaskCmd

}

func (r *SpeechSynthesizer) continuePayload(text string) map[string]any {
	continueTaskCmd := map[string]interface{}{
		"header": map[string]interface{}{
			"action":    "continue-task",
			"task_id":   r.taskID,
			"streaming": "duplex",
		},
		"payload": map[string]interface{}{
			"input": map[string]interface{}{
				"text": text,
			},
		},
	}
	return continueTaskCmd
	// return map[string]any{
	// 	headerKey: map[string]any{
	// 		actionKey:    actionContinue,
	// 		taskIDKey:    r.TaskID,
	// 		streamingKey: streamingDuplex,
	// 	},
	// 	"payload": map[string]any{
	// 		"model":      r.Model,
	// 		"task_group": "audio",
	// 		"task":       "tts",
	// 		"function":   "SpeechSynthesizer",
	// 		"input": map[string]any{
	// 			"text": text,
	// 		},
	// 	},
	// }
}

func (r *SpeechSynthesizer) finishPayload() map[string]any {

	finishTaskCmd := map[string]interface{}{
		"header": map[string]interface{}{
			"action":    "finish-task",
			"task_id":   r.taskID,
			"streaming": "duplex",
		},
		"payload": map[string]interface{}{
			"input": map[string]interface{}{},
		},
	}
	return finishTaskCmd
	// return map[string]any{
	// 	headerKey: map[string]any{
	// 		actionKey:    actionFinished,
	// 		taskIDKey:    r.TaskID,
	// 		streamingKey: streamingDuplex,
	// 	},
	// 	"payload": map[string]any{
	// 		"input": map[string]any{},
	// 	},
	// }
}

func (s *SpeechSynthesizer) String() string {
	return "[SpeechSynthesizer] model:" + s.model + ", voice:" + s.voice + ", format:" + s.aformat.Format
}

// Connect establishes the websocket connection.
func (s *SpeechSynthesizer) Connect(ctx context.Context) error {
	if s.conn != nil {
		return nil
	}
	hdr := s.headers.Clone()
	hdr.Set("Authorization", "bearer "+s.apiKey)
	hdr.Set("X-DashScope-DataInspection", "enable")
	if s.workspace != "" {
		hdr.Set("X-DashScope-WorkSpace", s.workspace)
	}
	c, _, err := s.dialer.DialContext(ctx, s.url, hdr)
	if err != nil {
		return err
	}
	s.conn = c
	s.g.Go(func() error {
		return s.readLoop()
	})
	// go s.readLoop(ctx)
	return nil
}

func (s *SpeechSynthesizer) isConnected() bool {
	return s.conn != nil
}

func (s *SpeechSynthesizer) reset() {
	s.startStreamTS = -1
	s.firstPkgTS = -1
	s.recvAudioMillis = 0
	s.isStarted = false
	s.audioBuf = nil
	s.lastResponse = nil
}

// readLoop reads messages and dispatches events.
func (s *SpeechSynthesizer) readLoop() error {
	defer func() {
		close(s.closed)
		log.Println("SpeechSynthesizer readLoop exited")
	}()
	for {
		if s.conn == nil {
			return nil
		}
		// select {
		// case <-ctx.Done():
		// 	log.Println("Context done, exiting tts readLoop.")
		// 	return nil
		// default:
		// }
		mt, payload, err := s.conn.ReadMessage()
		if err != nil {
			// connection closed
			return err
		}
		switch mt {
		case websocket.TextMessage:
			var obj map[string]any
			if err := json.Unmarshal(payload, &obj); err != nil {
				return err
			}
			s.lastResponse = obj
			// task-started
			// 			{
			//     "header": {
			//         "task_id": "2bf83b9a-baeb-4fda-8d9a-xxxxxxxxxxxx",
			//         "event": "task-started",
			//         "attributes": {}
			//     },
			//     "payload": {}
			// }
			// https://help.aliyun.com/zh/model-studio/cosyvoice-websocket-api?spm=a2c4g.11186623.help-menu-2400256.d_2_6_0_2.66b16d5bQQAMhU
			hdr, _ := obj[headerKey].(map[string]any)
			if hdr == nil {
				log.Println("SpeechSynthesizer Invalid message header")
				continue
			}
			evs, _ := hdr[eventKey].(string)
			switch eventType(strings.ToLower(evs)) {
			case eventStarted:
				select {
				case s.startCh <- struct{}{}:
				default:
				}
			case eventFinished:
				select {
				case s.completeCh <- struct{}{}:
				default:
				}
				// if s.callback != nil {
				// 	s.callback.OnComplete()
				// 	s.callback.OnClose()
				// }
				s.message <- &StreamAudioMessage{Event: "OnComplete"}
				return nil
			case eventFailed:
				select {
				case s.startCh <- struct{}{}:
				default:
				}
				select {
				case s.completeCh <- struct{}{}:
				default:
				}
				// if s.callback != nil {
				// 	s.callback.OnError(string(payload))
				// 	s.callback.OnClose()
				// }
				s.message <- &StreamAudioMessage{Event: "OnError", Err: errors.New("tts failed: " + string(payload))}

				return errors.New("tts failed: " + string(payload))
			case eventGenerated:
				// if s.callback != nil {
				// 	s.callback.OnEvent(string(payload))
				// }
				s.message <- &StreamAudioMessage{Event: "OnEvent", Data: payload}

			default:
			}
		case websocket.BinaryMessage:
			if s.recvAudioMillis == 0 {
				s.firstPkgTS = time.Now().UnixMilli()
			}
			// approximate received audio ms for 16-bit mono
			s.recvAudioMillis += float64(len(payload)) / (2 * float64(s.aformat.SampleRate) / 1000.0)
			// if s.callback == nil { // non-async, collect audio
			// 	s.audioBuf = append(s.audioBuf, payload...)
			// } else {
			// 	s.callback.OnData(payload)
			// }
			s.message <- &StreamAudioMessage{Event: "OnData", Data: payload}

		}
	}
}

func (s *SpeechSynthesizer) sendJSON(obj map[string]any) error {
	b, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return s.conn.WriteMessage(websocket.TextMessage, b)
}

func (s *SpeechSynthesizer) startStream(ctx context.Context) error {
	s.startStreamTS = time.Now().UnixMilli()
	s.firstPkgTS = -1
	s.recvAudioMillis = 0
	// if s.callback == nil {
	// 	s.asyncCall = false
	// } else {
	// 	s.asyncCall = true
	// }
	if s.conn == nil {
		if err := s.Connect(ctx); err != nil {
			return err
		}
	}
	//  AudioFormat{Format: s.aformat, SampleRate: s.sampleRate, Channels: "mono", BitRate: 0}
	// req := NewRequest(s.apiKey, s.model, s.voice, s.aformat, s.volume, s.speechRate, s.pitchRate, s.seed, s.synthType, s.instruction, s.langHints)
	// s.lastRequestID = req.TaskID
	payload := s.startPayload()

	if err := s.sendJSON(payload); err != nil {
		return err
	}
	select {
	case <-s.startCh:
		s.isStarted = true
		// if s.callback != nil {
		// 	s.callback.OnOpen()
		// }
		s.message <- &StreamAudioMessage{Event: "OnOpen"}
		log.Println("SpeechSynthesizer started!")
		return nil
	case <-time.After(10 * time.Second):
		return errors.New("start speech synthesizer timeout")
	}
}

func (s *SpeechSynthesizer) submitText(text string) error {
	if !s.isStarted {
		return errors.New("speech synthesizer not started")
	}
	// req := &Request{TaskID: s.lastRequestID, Model: s.model}
	payload := s.continuePayload(text)
	return s.sendJSON(payload)
}

// StreamingCall starts the session (if needed) and sends one text chunk.
func (s *SpeechSynthesizer) StreamingCall(ctx context.Context, text string) error {
	if !s.isStarted {
		if err := s.startStream(ctx); err != nil {
			return err
		}
	}
	return s.submitText(text)
}

// func (s *SpeechSynthesizer) Recv() (*StreamAudioMessage, error) {

// 	for {
// 		select {
// 		case <-l.ctx.Done():
// 			l.started = false
// 			l.cancel() // 取消 context，通知 Call 方法退出
// 			return nil, l.ctx.Err()
// 		case msg, ok := <-l.message:
// 			if !ok {
// 				l.started = false
// 				l.cancel() // 取消 context，通知 Call 方法退出
// 				return nil, io.EOF
// 			}
// 			return msg, nil
// 		}
// 	}

// }
func (s *SpeechSynthesizer) Output() <-chan *StreamAudioMessage {

	return s.message
}

// StreamingComplete stops the session and waits for remaining audio.
func (s *SpeechSynthesizer) StreamingComplete(ctx context.Context, timeout time.Duration) error {
	if !s.isStarted {
		return errors.New("speech synthesizer not started")
	}
	// req := &Request{TaskID: s.lastRequestID}
	payload := s.finishPayload()
	if err := s.sendJSON(payload); err != nil {
		return err
	}
	if timeout > 0 {
		// 一个典型的多路复用等待模式
		// 这个 select 会阻塞，直到 其中一个 case 条件触发。具体走哪个路径取决于运行时的情况
		select {
		case <-ctx.Done():
			log.Println("StreamingComplete canaceled by context")
		case <-s.completeCh:
			// log.Println("SpeechSynthesizer completed!")
		case <-time.After(timeout):
			return errors.New("speech synthesizer wait complete timeout")
		}
	} else {
		<-s.completeCh
	}
	s.isStarted = false
	if s.closeWSAfterUse {
		s.Close()
	}
	return nil
}

// AsyncStreamingComplete returns immediately; completion will trigger callbacks.
func (s *SpeechSynthesizer) AsyncStreamingComplete(timeout time.Duration) error {
	if !s.isStarted {
		return errors.New("speech synthesizer not started")
	}
	// req := &Request{TaskID: s.lastRequestID}
	paylod := s.finishPayload()
	if err := s.sendJSON(paylod); err != nil {
		return err
	}
	go func() {
		if timeout > 0 {
			select {
			case <-s.completeCh:
			case <-time.After(timeout):
			}
		} else {
			<-s.completeCh
		}
		s.isStarted = false
		if s.closeWSAfterUse {
			s.Close()
		}
	}()
	return nil
}

// Call performs a simple one-shot synth. If no callback, returns audio bytes.
// func (s *SpeechSynthesizer) Call(ctx context.Context, text string, timeout time.Duration) ([]byte, error) {
// 	if s.additional == nil {
// 		s.additional = map[string]any{"enable_ssml": true}
// 	} else {
// 		s.additional["enable_ssml"] = true
// 	}
// 	if s.callback == nil {
// 		s.asyncCall = false
// 	}
// 	if err := s.startStream(ctx); err != nil {
// 		return nil, err
// 	}
// 	if err := s.submitText(text); err != nil {
// 		return nil, err
// 	}
// 	if s.asyncCall {
// 		if err := s.AsyncStreamingComplete(timeout); err != nil {
// 			return nil, err
// 		}
// 		return nil, nil
// 	}
// 	if err := s.StreamingComplete(ctx, timeout); err != nil {
// 		return nil, err
// 	}
// 	return s.audioBuf, nil
// }

func (s *SpeechSynthesizer) Close() {
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
		close(s.message)
	}
}

func (s *SpeechSynthesizer) LastRequestID() string { return s.lastRequestID }

func (s *SpeechSynthesizer) FirstPackageDelay() int64 {
	if s.firstPkgTS <= 0 || s.startStreamTS <= 0 {
		return -1
	}
	return s.firstPkgTS - s.startStreamTS
}

func (s *SpeechSynthesizer) Response() map[string]any { return s.lastResponse }

//
// --- Object pool implementation ---
//

type SpeechSynthesizerObjectPool struct {
	mu            sync.Mutex
	pool          []*poolObj
	available     []bool
	borrowed      int
	maxSize       int
	stop          bool
	url           string
	apiKey        string
	headers       http.Header
	workspace     string
	model         string
	voice         string
	reconnectBase int
}

type poolObj struct {
	syn         *SpeechSynthesizer
	connectedAt time.Time
}

func NewSpeechSynthesizerObjectPool(maxSize int, url, apiKey string, headers http.Header, workspace, model, voice string) (*SpeechSynthesizerObjectPool, error) {
	if maxSize <= 0 || maxSize > 100 {
		return nil, errors.New("max_size must be 1..100")
	}
	p := &SpeechSynthesizerObjectPool{
		maxSize:       maxSize,
		url:           url,
		apiKey:        apiKey,
		headers:       headers.Clone(),
		workspace:     workspace,
		model:         model,
		voice:         voice,
		reconnectBase: 30,
	}
	for i := 0; i < maxSize; i++ {
		// syn, err := NewSpeechSynthesizer(url, apiKey, model, voice, AudioFormatDefault, 50, 1.0, 1.0, 0, 0, nil, nil, headers, nil, workspace, nil)
		syn, err := NewSpeechSynthesizer(model, voice, AudioFormatDefault, nil)

		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = syn.Connect(ctx)
		cancel()
		p.pool = append(p.pool, &poolObj{syn: syn, connectedAt: time.Now()})
		p.available = append(p.available, true)
	}
	go p.autoReconnect()
	return p, nil
}

func (p *SpeechSynthesizerObjectPool) autoReconnect() {
	for {
		time.Sleep(1 * time.Second)
		p.mu.Lock()
		if p.stop {
			p.mu.Unlock()
			return
		}
		now := time.Now()
		toRenew := []*poolObj{}
		for i, po := range p.pool {
			if !p.available[i] {
				continue
			}
			if po.syn == nil || !po.syn.isConnected() || now.Sub(po.connectedAt) > time.Duration(p.reconnectBase+rand.Intn(10)-5)*time.Second {
				p.available[i] = false
				toRenew = append(toRenew, po)
			}
		}
		p.mu.Unlock()
		for _, po := range toRenew {
			syn, err := NewSpeechSynthesizer(p.model, p.voice, AudioFormatDefault, nil)
			if err != nil {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = syn.Connect(ctx)
			cancel()
			po.syn = syn
			po.connectedAt = time.Now()
			p.mu.Lock()
			// mark available again
			for i := range p.pool {
				if p.pool[i] == po {
					p.available[i] = true
					break
				}
			}
			p.mu.Unlock()
		}
	}
}

func (p *SpeechSynthesizerObjectPool) Shutdown() {
	p.mu.Lock()
	p.stop = true
	for _, po := range p.pool {
		if po.syn != nil {
			po.syn.Close()
		}
	}
	p.pool = nil
	p.available = nil
	p.mu.Unlock()
}

// Borrow returns a synthesizer; caller should Return it when done.
func (p *SpeechSynthesizerObjectPool) Borrow() *SpeechSynthesizer {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, po := range p.pool {
		if p.available[i] && po.syn != nil && po.syn.isConnected() {
			p.available[i] = false
			p.borrowed++
			return po.syn
		}
	}
	// exhausted: create a new unconnected object
	syn, _ := NewSpeechSynthesizer(p.model, p.voice, AudioFormatDefault, nil)
	return syn
}

// Return returns a synthesizer to the pool.
func (p *SpeechSynthesizerObjectPool) Return(s *SpeechSynthesizer) bool {
	if s == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, po := range p.pool {
		if po.syn == s {
			if p.available[i] {
				return false
			}
			p.available[i] = true
			if !s.isConnected() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = s.Connect(ctx)
				cancel()
			}
			po.connectedAt = time.Now()
			p.borrowed--
			return true
		}
	}
	// if not from pool, drop
	return false
}
