package asr

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

type eventType string

const (
	eventStarted   eventType = "task-started"
	eventFinished  eventType = "task-finished"
	eventFailed    eventType = "task-failed"
	eventGenerated eventType = "result-generated"
)
const (
	wsURL            = "wss://dashscope.aliyuncs.com/api-ws/v1/inference/"
	silenceTimeout   = 23 * time.Second
	audioChunkSize   = 12800
	streamPollPeriod = 10 * time.Millisecond
)

type RecognitionResult struct {
	StatusCode int
	RequestID  string
	Code       string
	Message    string
	Output     map[string]any
	Usage      map[string]any
	Usages     []map[string]any
}

func (r RecognitionResult) String() string {
	data, _ := json.Marshal(r)
	return string(data)
}

func (r RecognitionResult) GetSentence() any {
	if r.Output == nil {
		return nil
	}
	return r.Output["sentence"]
}

func (r RecognitionResult) GetRequestID() string {
	return r.RequestID
}

func (r RecognitionResult) GetUsage(sentence map[string]any) map[string]any {
	if sentence == nil || r.Usages == nil {
		return nil
	}
	endTime, ok := sentence["end_time"]
	if !ok || endTime == nil {
		return nil
	}
	for _, usage := range r.Usages {
		if usage["end_time"] == endTime {
			if raw, ok := usage["usage"].(map[string]any); ok {
				return raw
			}
			return nil
		}
	}
	return nil
}

func IsSentenceEnd(sentence map[string]any) bool {
	if sentence == nil {
		return false
	}
	endTime, ok := sentence["end_time"]
	return ok && endTime != nil
}

type RecognitionCallback interface {
	OnOpen()
	OnComplete(result string)
	OnError(result *RecognitionResult)
	OnClose()
	OnEvent(result *RecognitionResult)
}
type StreamMessage struct {
	Event string
	Data  string
	Err   error
}

type Recognition struct {
	model      string
	format     string
	sampleRate int
	workspace  string
	// callback   RecognitionCallback
	apiKey     string
	isStarted  bool
	headers    http.Header
	startCh    chan struct{}
	completeCh chan struct{}

	running          bool
	recognitionOnce  bool
	streamClosed     bool
	streamData       chan []byte
	workerDone       chan struct{}
	silenceTimer     *time.Timer
	kwargs           map[string]any
	phraseID         string
	startStreamTS    int64
	firstPackageTS   int64
	stopStreamTS     int64
	onCompleteTS     int64
	requestIDConfirm bool
	lastRequestID    string
	taskID           string

	connMu  sync.Mutex
	conn    *websocket.Conn
	g       *errgroup.Group
	result  string
	message chan *StreamMessage
	ctx     context.Context
}

// , workspace string, kwargs map[string]any
func NewRecognition(model string, format string, sampleRate int, ctx context.Context, g *errgroup.Group) (*Recognition, error) {
	// if model == "" {
	// 	return nil, errors.New("model is required")
	// }
	// if format == "" {
	// 	return nil, errors.New("format is required")
	// }
	// if sampleRate <= 0 {
	// 	return nil, errors.New("sampleRate is required")
	// }
	// if kwargs == nil {
	// 	kwargs = make(map[string]any)
	// }
	return &Recognition{
		model:      model,
		format:     format,
		sampleRate: sampleRate,
		headers:    http.Header{},
		workspace:  "",
		// callback:       callback,
		apiKey:         os.Getenv("DASHSCOPE_API_KEY"),
		streamData:     make(chan []byte, 128),
		kwargs:         nil,
		lastRequestID:  uuid.NewString(),
		startStreamTS:  -1,
		firstPackageTS: -1,
		stopStreamTS:   -1,
		onCompleteTS:   -1,
		startCh:        make(chan struct{}, 1),
		completeCh:     make(chan struct{}, 1),
		taskID:         uuid.NewString(),
		ctx:            ctx,
		g:              g,
		message:        make(chan *StreamMessage, 128),
	}, nil
}

func (r *Recognition) Close() {
	if r.conn != nil {
		r.conn.Close()
	}
}

func (r *Recognition) Output() <-chan *StreamMessage {
	return r.message
}
func (r *Recognition) startPayload() map[string]any {
	// payload :=
	// if r.phraseID != "" {
	// 	payload["resources"] = []map[string]any{{
	// 		"resource_id":   r.phraseID,
	// 		"resource_type": "asr_phrase",
	// 	}}
	// }
	payload := map[string]any{
		"header": map[string]any{
			"action":    "run-task",
			"task_id":   r.taskID,
			"streaming": "duplex",
		},
		"payload": map[string]any{
			"task_group": "audio",
			"task":       "asr",
			"function":   "recognition",
			"model":      r.model,
			"parameters": map[string]any{
				"format":      r.format,
				"sample_rate": r.sampleRate,
			},
			"input": map[string]any{},
		},
	}
	return payload
}

func (r *Recognition) finishPayload() map[string]any {
	cmd := map[string]any{
		"header": map[string]any{
			"action":    "finish-task",
			"task_id":   r.taskID,
			"streaming": "duplex",
		},
		"payload": map[string]any{"input": map[string]any{}},
	}
	return cmd
}

// func (r *Recognition) SendAudioFrame() error {

// }

// func (r *Recognition) StreamingCall(audioData []byte) error {
// 	const (
// 		sampleRate      = 16000
// 		bytesPerSample  = 2
// 		channels        = 1
// 		chunkDurationMs = 100
// 	)

// 	bytesPer100ms := sampleRate * bytesPerSample * channels * chunkDurationMs / 1000
// 	ticker := time.NewTicker(100 * time.Millisecond)
// 	defer ticker.Stop()

// 	for offset := 0; offset < len(audioData); offset += bytesPer100ms {
// 		end := offset + bytesPer100ms
// 		if end > len(audioData) {
// 			end = len(audioData)
// 		}

// 		<-ticker.C // 控制发送节奏

// 		if err := r.conn.WriteMessage(
// 			websocket.BinaryMessage,
// 			audioData[offset:end],
// 		); err != nil {
// 			return err
// 		}
// 	}

// 	return nil
// }

func (r *Recognition) StreamingCall(audioData []byte) error {

	if err := r.conn.WriteMessage(websocket.BinaryMessage, audioData); err != nil {
		return err
	}
	return nil
}

func (r *Recognition) StreamingComplete(timeout time.Duration) error {
	if !r.isStarted {
		return errors.New("recognition has not started")
	}
	payload := r.finishPayload()
	if err := r.sendJSON(payload); err != nil {
		return err
	}
	if timeout > 0 {
		select {
		case <-r.completeCh:
			return nil
		case <-time.After(timeout):
			return errors.New("finish recognition timeout")
		}
	}
	r.isStarted = false
	// r.Close()
	return nil

}

func (r *Recognition) StartStream() error {
	if r.conn == nil {
		if err := r.Connect(); err != nil {
			return err
		}
	}
	payload := r.startPayload()
	if err := r.sendJSON(payload); err != nil {
		return err
	}
	select {
	case <-r.startCh:
		r.isStarted = true
		// if r.callback != nil {
		// 	r.callback.OnOpen()
		// }
		log.Println("Recognition started!")
		return nil
	case <-time.After(10 * time.Second):
		return errors.New("start recognition timeout")
	}
}

func (r *Recognition) sendJSON(data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return r.conn.WriteMessage(websocket.TextMessage, b)
}

func (r *Recognition) Connect() error {
	if r.conn != nil {
		return nil
	}
	hdr := r.headers.Clone()
	hdr.Set("Authorization", "bearer "+r.apiKey)
	hdr.Set("X-DashScope-DataInspection", "enable")
	if r.workspace != "" {
		hdr.Set("X-DashScope-WorkSpace", r.workspace)
	}
	conn, _, err := websocket.DefaultDialer.DialContext(r.ctx, wsURL, hdr)
	if err != nil {
		return err
	}
	r.conn = conn
	r.g.Go(func() error {
		return r.readLoop()
	})
	return nil
}

func (r *Recognition) send(msg *StreamMessage) {
	select {
	case r.message <- msg:
	case <-r.ctx.Done():
		return
	}
}
func (r *Recognition) readLoop() error {
	defer func() {
		// close(r.cos)
		close(r.message)
		log.Println("Recognition readloop exited.")
	}()
	for {
		// select {
		// case <-ctx.Done():
		// 	// log.Println("Context done, exiting tts readLoop.")
		// 	return nil
		// default:
		// }
		if r.conn == nil {
			return nil
		}
		msgType, payload, err := r.conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			return err
		}
		switch msgType {
		case websocket.TextMessage:
			var obj map[string]any
			if err := json.Unmarshal(payload, &obj); err != nil {
				log.Println("Recognition Error unmarshaling message:", err)
				return err
			}
			// log.Printf("Recognition Received message: %s", string(payload))

			hdr, _ := obj["header"].(map[string]any)
			if hdr == nil {
				log.Println("Recognition Invalid message header")
				continue
			}
			event, _ := hdr["event"].(string)
			switch eventType(strings.ToLower(event)) {
			case eventStarted:

				select {
				case r.startCh <- struct{}{}:
				default:
				}
				r.send(&StreamMessage{Event: "OnOpen"})

			case eventFinished:
				r.send(&StreamMessage{Event: "OnComplete", Data: r.result})
				// close(r.message)
				select {
				case r.completeCh <- struct{}{}:
				default:
				}
				// if r.callback != nil {
				// 	r.callback.OnComplete(r.result)
				// 	r.callback.OnClose()
				// }

				return nil
			case eventFailed:
				r.send(&StreamMessage{Event: "OnError", Data: r.result})
				// close(r.message)
				select {
				case r.startCh <- struct{}{}:
				default:
				}
				select {
				case r.completeCh <- struct{}{}:
				default:
				}
				// if r.callback != nil {
				// 	// r.callback.OnError(string(payload))
				// 	r.callback.OnClose()
				// }
				return errors.New("recognition failed: " + string(payload))
			case eventGenerated:
				// if r.callback != nil {
				// 	// r.callback.OnEvent()
				// 	// log.Println(payload)

				// }
				if sentence, ok := obj["payload"].(map[string]any)["output"].(map[string]any)["sentence"].(map[string]any); ok {
					text := sentence["text"].(string)
					if text != "" {
						// r.result.WriteString(text)
						r.result = text
					}
				}

				// callback
			}

		}
	}
}

func (r *Recognition) submitAudioFrame(audioData []byte) error {
	return r.SendAudioFrame(audioData)
}

func (r *Recognition) Start(phraseID string, kwargs map[string]any) error {
	// if r.callback == nil {
	// 	return errors.New("callback is required")
	// }
	if r.running {
		return errors.New("speech recognition has started")
	}
	// if kwargs != nil {
	// 	for k, v := range kwargs {
	// 		r.kwargs[k] = v
	// 	}
	// }
	r.resetState(phraseID, false)
	r.running = true
	r.workerDone = make(chan struct{})
	go r.receiveWorker()
	// if r.callback != nil {
	// 	r.callback.OnOpen()
	// }
	r.silenceTimer = time.AfterFunc(silenceTimeout, r.silenceStopTimer)
	return nil
}

// func (r *Recognition) Call(file string, phraseID string, kwargs map[string]any) (*RecognitionResult, error) {
// 	if r.running {
// 		return nil, errors.New("speech recognition has been called")
// 	}
// 	info, err := os.Stat(file)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if info.IsDir() {
// 		return nil, errors.New("is a directory: " + file)
// 	}
// 	if info.Size() == 0 {
// 		return nil, errors.New("the supplied file was empty (zero bytes long)")
// 	}
// 	if kwargs != nil {
// 		for k, v := range kwargs {
// 			r.kwargs[k] = v
// 		}
// 	}
// 	r.resetState(phraseID, true)
// 	r.running = true
// 	if err := r.readFileToStream(file); err != nil {
// 		r.running = false
// 		return nil, err
// 	}
// 	result, err := r.receiveOnce()
// 	r.running = false
// 	return result, err
// }

func (r *Recognition) Stop() error {
	if !r.running {
		return errors.New("speech recognition has stopped")
	}
	r.stopStreamTS = nowMs()
	r.running = false
	r.streamClosed = true
	if r.silenceTimer != nil {
		r.silenceTimer.Stop()
		r.silenceTimer = nil
	}
	r.sendFinishTask()
	if r.workerDone != nil {
		<-r.workerDone
	}
	// if r.callback != nil {
	// 	r.callback.OnClose()
	// }
	r.closeConn()
	return nil
}

func (r *Recognition) SendAudioFrame(buffer []byte) error {
	if !r.running {
		return errors.New("speech recognition has stopped")
	}
	if r.startStreamTS < 0 {
		r.startStreamTS = nowMs()
	}
	if len(buffer) == 0 {
		return nil
	}
	r.streamData <- buffer
	return nil
}

func (r *Recognition) GetFirstPackageDelay() int64 {
	return r.firstPackageTS - r.startStreamTS
}

func (r *Recognition) GetLastPackageDelay() int64 {
	return r.onCompleteTS - r.stopStreamTS
}

func (r *Recognition) GetLastRequestID() string {
	return r.lastRequestID
}

func (r *Recognition) resetState(phraseID string, recognitionOnce bool) {
	r.phraseID = phraseID
	r.recognitionOnce = recognitionOnce
	r.streamClosed = false
	r.streamData = make(chan []byte, 128)
	r.startStreamTS = -1
	r.firstPackageTS = -1
	r.stopStreamTS = -1
	r.onCompleteTS = -1
}

func (r *Recognition) receiveWorker() {
	defer func() {
		if r.workerDone != nil {
			close(r.workerDone)
		}
	}()
	if err := r.receiveLoop(nil); err != nil {
		// if r.callback != nil {
		// 	r.callback.OnError(res)
		// 	r.callback.OnClose()
		// }
		// res := &RecognitionResult{StatusCode: http.StatusBadRequest, Message: err.Error()}

		r.running = false
	}
}

// func (r *Recognition) receiveOnce() (*RecognitionResult, error) {
// 	var sentences []map[string]any
// 	var usages []map[string]any
// 	var lastResponse *RecognitionResult
// 	if err := r.receiveLoop(func(res *RecognitionResult) bool {
// 		lastResponse = res
// 		if res.Output == nil {
// 			return false
// 		}
// 		sentence, ok := res.Output["sentence"].(map[string]any)
// 		if ok && IsSentenceEnd(sentence) {
// 			sentences = append(sentences, sentence)
// 			if res.Usage != nil {
// 				usages = append(usages, map[string]any{
// 					"end_time": sentence["end_time"],
// 					"usage":    res.Usage,
// 				})
// 			}
// 		}
// 		return false
// 	}); err != nil {
// 		return &RecognitionResult{StatusCode: http.StatusBadRequest, Message: err.Error()}, err
// 	}
// 	r.onCompleteTS = nowMs()
// 	if lastResponse == nil {
// 		return &RecognitionResult{StatusCode: http.StatusBadRequest, Message: "empty response"}, nil
// 	}
// 	if len(sentences) == 0 {
// 		return lastResponse, nil
// 	}
// 	lastResponse.Usages = usages
// 	lastResponse.Usage = nil
// 	lastResponse.Output = map[string]any{"sentence": sentences}
// 	return lastResponse, nil
// }

func (r *Recognition) receiveLoop(onResult func(res *RecognitionResult) bool) error {
	conn, err := r.openConn()
	if err != nil {
		return err
	}
	r.setConn(conn)
	if err := r.sendRunTask(conn); err != nil {
		return err
	}
	go func() {
		_ = r.sendAudioLoop(conn)
	}()
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var event wsEvent
		if err := json.Unmarshal(message, &event); err != nil {
			continue
		}
		if event.Header.Event == "task-started" {
			continue
		}
		if event.Header.Event == "task-failed" {
			res := &RecognitionResult{
				StatusCode: http.StatusBadRequest,
				RequestID:  event.Header.TaskID,
				Code:       event.Header.ErrorCode,
				Message:    event.Header.ErrorMessage,
				Output:     event.Payload.Output,
			}
			// if r.callback != nil {
			// 	r.callback.OnError(res)
			// 	r.callback.OnClose()
			// }
			return errors.New(res.Message)
		}
		if event.Header.Event == "task-finished" {
			r.onCompleteTS = nowMs()
			// if r.callback != nil {
			// 	r.callback.OnComplete("")
			// 	r.callback.OnClose()
			// }
			return nil
		}
		if event.Header.Event != "result-generated" {
			continue
		}
		res := r.resultFromEvent(&event)
		if onResult != nil {
			if onResult(res) {
				return nil
			}
		}
		// if r.callback != nil {
		// 	r.callback.OnEvent(res)
		// }
	}
}

func (r *Recognition) resultFromEvent(event *wsEvent) *RecognitionResult {
	res := &RecognitionResult{
		StatusCode: http.StatusOK,
		RequestID:  event.Header.TaskID,
		Output:     event.Payload.Output,
		Usage:      event.Payload.Usage,
	}
	if res.Output != nil {
		if sentence, ok := res.Output["sentence"].(map[string]any); ok {
			if heartbeat, ok := sentence["heartbeat"].(bool); ok && heartbeat {
				return res
			}
			if r.firstPackageTS < 0 {
				r.firstPackageTS = nowMs()
			}
			if IsSentenceEnd(sentence) && res.Usage != nil {
				res.Usages = []map[string]any{{
					"end_time": sentence["end_time"],
					"usage":    res.Usage,
				}}
			}
		}
	}
	if !r.requestIDConfirm && res.RequestID != "" {
		r.requestIDConfirm = true
		r.lastRequestID = res.RequestID
	}
	return res
}

func (r *Recognition) sendAudioLoop(conn *websocket.Conn) error {
	for {
		if r.streamClosed && len(r.streamData) == 0 {
			r.stopStreamTS = nowMs()
			return r.sendFinishTaskConn(conn)
		}
		select {
		case frame := <-r.streamData:
			if len(frame) == 0 {
				continue
			}
			if r.startStreamTS < 0 {
				r.startStreamTS = nowMs()
			}
			r.resetSilenceTimer()
			if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
				return err
			}
		case <-time.After(streamPollPeriod):
			if !r.running && !r.recognitionOnce {
				r.stopStreamTS = nowMs()
				return r.sendFinishTaskConn(conn)
			}
		}
	}
}

func (r *Recognition) readFileToStream(file string) error {
	fd, err := os.Open(file)
	if err != nil {
		return err
	}
	defer fd.Close()
	buf := make([]byte, audioChunkSize)
	for {
		n, err := fd.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			r.streamData <- chunk
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	r.stopStreamTS = nowMs()
	r.streamClosed = true
	return nil
}

func (r *Recognition) sendRunTask(conn *websocket.Conn) error {
	payload := map[string]any{
		"task_group": "audio",
		"task":       "asr",
		"function":   "recognition",
		"model":      r.model,
		"parameters": r.buildParams(),
		"input":      map[string]any{},
	}
	if r.phraseID != "" {
		payload["resources"] = []map[string]any{{
			"resource_id":   r.phraseID,
			"resource_type": "asr_phrase",
		}}
	}
	cmd := map[string]any{
		"header": map[string]any{
			"action":    "run-task",
			"task_id":   r.lastRequestID,
			"streaming": "duplex",
		},
		"payload": payload,
	}
	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

func (r *Recognition) sendFinishTask() error {
	r.connMu.Lock()
	conn := r.conn
	r.connMu.Unlock()
	if conn == nil {
		return nil
	}
	return r.sendFinishTaskConn(conn)
}

func (r *Recognition) sendFinishTaskConn(conn *websocket.Conn) error {
	cmd := map[string]any{
		"header": map[string]any{
			"action":    "finish-task",
			"task_id":   r.lastRequestID,
			"streaming": "duplex",
		},
		"payload": map[string]any{"input": map[string]any{}},
	}
	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

func (r *Recognition) buildParams() map[string]any {
	params := map[string]any{
		"format":      r.format,
		"sample_rate": r.sampleRate,
	}
	for k, v := range r.kwargs {
		if v != nil {
			params[k] = v
		}
	}
	return params
}

func (r *Recognition) resetSilenceTimer() {
	if r.silenceTimer != nil {
		r.silenceTimer.Stop()
	}
	r.silenceTimer = time.AfterFunc(silenceTimeout, r.silenceStopTimer)
}

func (r *Recognition) silenceStopTimer() {
	r.running = false
	r.streamClosed = true
	if r.silenceTimer != nil {
		r.silenceTimer.Stop()
	}
	r.silenceTimer = nil
}

func (r *Recognition) openConn() (*websocket.Conn, error) {
	if r.apiKey == "" {
		return nil, errors.New("DASHSCOPE_API_KEY is not set")
	}
	hdr := http.Header{}
	hdr.Set("Authorization", "bearer "+r.apiKey)
	hdr.Set("X-DashScope-DataInspection", "enable")
	if r.workspace != "" {
		hdr.Set("X-DashScope-WorkSpace", r.workspace)
	}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, hdr)
	return conn, err
}

func (r *Recognition) setConn(conn *websocket.Conn) {
	r.connMu.Lock()
	r.conn = conn
	r.connMu.Unlock()
}

func (r *Recognition) closeConn() {
	r.connMu.Lock()
	conn := r.conn
	r.conn = nil
	r.connMu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

func nowMs() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

type wsEvent struct {
	Header struct {
		Action       string `json:"action"`
		TaskID       string `json:"task_id"`
		Streaming    string `json:"streaming"`
		Event        string `json:"event"`
		ErrorCode    string `json:"error_code,omitempty"`
		ErrorMessage string `json:"error_message,omitempty"`
	} `json:"header"`
	Payload struct {
		Output map[string]any `json:"output"`
		Usage  map[string]any `json:"usage,omitempty"`
	} `json:"payload"`
}
