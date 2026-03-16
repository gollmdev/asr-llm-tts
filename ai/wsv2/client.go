package wsv2

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
	"github.com/gollmdev/asr-llm-tts/ai/session"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

// type WsMessageType string

// const (
// 	SessionText  WsMessageType = "text"
// 	SessionAudio WsMessageType = "audio"
// )

// type WsMessage struct {
// 	Type WsMessageType
// 	Data []byte
// }

// ==================== WebSocket 客户端 ====================
// Client represents a single WebSocket connection.
type Client struct {
	Hub  *Hub
	Conn *websocket.Conn
	// send    chan WsMessage
	Session *dag.Session

	g      *errgroup.Group
	ctx    context.Context
	cancel context.CancelFunc

	closeOnce sync.Once

	userId int64

	registerCh chan bool

	sessionTimeOut time.Duration
}

type ClientConfig struct {
	Hub  *Hub
	Conn *websocket.Conn
	// Callback session.Callback
	UserId int64
	// SessionTimeOut time.Duration
}

func NewClient(ctx context.Context, cancel context.CancelFunc, session *dag.Session, config *ClientConfig) *Client {

	g, ctx := errgroup.WithContext(ctx)

	return &Client{
		Hub:        config.Hub,
		Conn:       config.Conn,
		Session:    session,
		ctx:        ctx,
		cancel:     cancel,
		g:          g,
		userId:     config.UserId,
		registerCh: make(chan bool, 1), // ⭐ 必须带缓冲
	}
}

func (c *Client) Start() {
	// 启动 readPump 和 writePump
	c.Hub.Register <- c

	// 等待 Hub 处理完成
	ok := <-c.registerCh
	if !ok {

		//  resp := WsResponse{
		// 	Code:    1001,
		// 	Message: "连接数已达上限",
		// }

		// _ = c.Conn.WriteJSON(resp)
		// _ = c.Conn.WriteJSON(map[string]any{
		// 	"type":    "error",
		// 	"code":    1001,
		// 	"message": "连接数已达上限",
		// })
		// too many connections, send error message and close connection
		c.sendMessage(session.BuildMessage("many_connections", "连接数已达上限, 请重试"))
		// 优雅关闭
		_ = c.Conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "too many connections"),
		)
		c.Conn.Close()
		log.Println("ws-conn: 注册失败，连接已关闭")
		return
	}

	c.g.Go(func() error {
		return c.ReadPump()
	})

	c.g.Go(func() error {
		return c.WritePump()
	})

	go func() {
		if err := c.g.Wait(); err != nil {
			// c.Session.cancel()
			log.Println("ws-conn: error client closed:", err)
		}
		// c.Close()
		c.Hub.unregister <- c
		log.Println("ws-conn: client closed")
	}()
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.Session.Close()
		c.cancel()
		// c.Close()
		c.Conn.Close()
		// c.Hub.unregister <- c
		// log.Println("ws-conn: client closed")
		// c.Session.Close()
		// c.Conn.Close()
	})
}

// Client (WebSocket)
//         /             \
//   readPump           writePump
//      |                   ^
//      v                   |
//   Session  ───────→  output chan
//  (ASR/LLM/TTS)

// read pump pumps messages from the WebSocket connection to the hub.
// The application runs readPump in a per-connection goroutine.
// The application ensures that there is at most one reader on a connection by executing all reads from this goroutine.
func (c *Client) ReadPump() error {
	defer func() {
		// On exit, unregister the client and close the connection
		// send c to unregister channel
		// c.session.Close()
		// close(c.send)
		// close the session
		// close the connection
		// c.conn.Close()
		log.Println("ws-conn: close readPump!")
	}()

	// set read limit
	// c.conn.SetReadLimit(512)
	// set read deadline
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	// set pong handler
	c.Conn.SetPongHandler(func(appData string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// c.Session.LLMConsumer()
	// c.Session.MonitorSubSize()
	c.Session.Start()
	log.Printf(">>>node:  %d  start", c.Session.ID)

	// c.Session.LLMTaskConsumer() // 启动 LLM 任务消费者

	for {
		// select {
		// // case <-c.session.Done:
		// // 	log.Println("Session processing done, exiting readPump.")
		// // 	return
		// case <-c.ctx.Done():
		// 	log.Println("ws-conn: ws-conn: Client context done, exiting readPump.")
		// 	return c.ctx.Err()
		// case <-c.Session.ctx.Done():
		// 	log.Println("Session is closed, exiting readPump.")
		// 	return nil
		// default:
		// 	// broadcast the received message to all clients
		// 	// c.hub.broadcast <- message

		// }
		// 一直读取消息，直到发生错误（如连接关闭）
		msgType, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("read error: %v", err)

			} else {
				log.Printf("read info: %v", err)
			}
			// c.Session.cancel()
			c.cancel()
			return nil
		}
		switch msgType {
		case websocket.TextMessage:
			// process text message
			// log.Printf("Received text message: %s", string(message))
			// c.session.PushText(string(message))
			// 		    _wsClient.sendJSON({
			//   "type": "message",
			//   "config":{
			//       "tts": autoPlay,
			//   },
			//   "content": text,
			// });
			var obj map[string]any
			if err := json.Unmarshal(message, &obj); err != nil {
				log.Printf("invalid message format: %v", err)
				// c.Session.PublishTextStream(string(message))
				// c.Session.PublishEvent(event.Event{
				// 	Type: event.EventTextChunk,
				// 	Data: string(message),
				// })
				c.Session.Dispatch("text", string(message))
			} else {
				if objType, ok := obj["type"].(string); ok {
					switch objType {
					case "config":
						if config, ok := obj["config"].(map[string]any); ok {
							if tts, ok := config["tts"].(bool); ok {
								if tts {
									// c.Session.SessionConfig.TTSEnabled = true
									// c.Session.TTsConsumer()
									c.Session.SetTTS(tts)
									log.Println("tts open!")
								}
							}
						}
					case "message":

						if text, ok := obj["content"].(string); ok {
							// c.Session.PublishTextStream(text)
							// c.Session.PublishEvent(event.Event{
							// 	Type: event.EventTextChunk,
							// 	Data: string(text),
							// })
							c.Session.Dispatch("text", string(text))
						}

					case "control":
						if config, ok := obj["config"].(map[string]any); ok {
							if tts, ok := config["tts"].(bool); ok {
								c.Session.SetTTS(tts)
								// if tts != c.Session.SessionConfig.TTSEnabled {

								// 	if tts {
								// 		c.Session.SessionConfig.TTSEnabled = true
								// 		// c.Session.waitConsumers("tts")
								// 		c.Session.TTsConsumer()
								// 		log.Println("tts open!")
								// 	} else {
								// 		c.Session.SessionConfig.TTSEnabled = false
								// 		// c.Session.CancelConsumer("tts")
								// 		c.Session.UnSubscribe(session.TTS)
								// 		log.Println("tts close!")
								// 	}
								// }

							}
						}
					}
				}

			}
			// return

		case websocket.BinaryMessage:
			// process binary message

			// c.Session.PublishBinaryStream(message)
			c.DispatchBinaryStream(message)

		default:
			log.Printf("Received unsupported message type: %d", msgType)
		}

	}

}
func (c *Client) DispatchBinaryStream(bytes []byte) {
	if len(bytes) == 0 {
		return
	}
	msg_type := bytes[0]
	log.Println("msg_type ", msg_type)
	switch msg_type {
	// case 0x01:
	// 	s.AudioRecognitionConsumer()
	case 0x02:
		bytes = bytes[1:]
		// log.Printf("Received binary message of length %d", len(bytes))
		// s.bus.Publish(event.Event{Type: event.EventAudioChunk, Data: bytes})
		c.Session.Dispatch("audio", bytes)

	case 0x03:
		// s.bus.Publish(event.Event{Type: event.EventAudioDone, Data: nil})
		c.Session.Dispatch("audio_end", nil)
	case 0x04:
		c.cancel() // end session on audio done
	default:
		log.Printf("Unknown audio message type: %x", msg_type)
	}
}

//    Client WS
//       ┌───────────┐
//       │ readPump  │
//       │(收 Text/Bin)│
//       └─────┬─────┘
//             │
//             ▼
//       ┌───────────────┐
//       │  SessionLayer │  <- 中间层处理
//       │  (ASR/LLM/TTS)│
//       └─────┬─────────┘
//             │
//             ▼
//       ┌───────────┐
//       │ writePump │
//       │(发送 Text/Audio)│
//       └───────────┘
// writePump pumps messages from the hub to the WebSocket connection.

// func (c *Client) forwardMessage() {
// 	for {
// 		select {
// 		case <-c.session.ctx.Done(): // 监听 session 的结束信号
// 			log.Println("Session is closed, exiting forwardMessage.")
// 			return
// 		case text, ok := <-c.session.textOut:
// 			if !ok {
// 				return
// 			}
// 			c.send <- WsMessage{Type: SessionText, Data: []byte(text)}

// 		case audio, ok := <-c.session.audioOut:
// 			if !ok {
// 				return
// 			}
// 			c.send <- WsMessage{Type: SessionAudio, Data: audio}
// 		}
// 	}
// }

func (c *Client) WritePump() error {
	ticker := time.NewTicker(50 * time.Second)
	defer func() {
		ticker.Stop()
		c.Close()
		// c.Session.Close()

		// c.Conn.Close()
		// c.Hub.unregister <- c
		log.Println("ws-conn: close writePump!")
	}()
	for {
		select {
		// case <-c.ctx.Done():
		// 	log.Println("Client context done, exiting writePump.")
		// 	return c.ctx.Err()
		// case <-c.Session.ctx.Done(): // 监听 session 的结束信号
		// 	log.Println("Session is closed, exiting writePump.")
		// 	return nil
		case message, ok := <-c.Session.Output():
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Hub 关闭 channel
				// c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				log.Println("Session output channel closed.")
				return nil
			}
			switch message.Type {
			case "asr_text":
				data := message.Data.(string)
				log.Printf("Sending text message: %s", string(data))
				sendMsg := map[string]interface{}{
					"data": map[string]string{
						"content": data,
					},
					"event": "asr_result",
				}
				if msg, err := json.Marshal(sendMsg); err == nil {
					c.sendMessage(msg)
				}
			case "no_asr_text":
				sendMsg := map[string]interface{}{
					"data": map[string]string{
						"content": "未识别到内容, 请重试",
					},
					"event": "no_asr_result",
				}
				if msg, err := json.Marshal(sendMsg); err == nil {
					c.sendMessage(msg)
				}
			case "llm_chunk":
				data := message.Data.(string)
				log.Printf("Sending text message: %s", string(data))
				sendMsg := map[string]interface{}{
					"data": map[string]string{
						"content": data,
					},
					"event": "message",
				}
				if msg, err := json.Marshal(sendMsg); err == nil {
					c.sendMessage(msg)
				}
			case "tts_audio":
				// log.Printf("Sending text message: %s", string(message.Data))
				// if err := c.conn.WriteMessage(websocket.TextMessage, message.Data); err != nil {
				// 	log.Println("write error:", err)
				// 	return
				// }
				data := message.Data.([]byte)
				log.Printf("Sending audio message of length %d", len(data))

				if err := c.Conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
					// c.Session.cancel()
					c.cancel()
					log.Println("write error:", err)

					return nil
				}
			}

			// if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			// 	log.Println("write error:", err)
			// 	return
			// }

		// case <-c.session.Done:
		// 	log.Println("Session processing done, exiting writePump.")
		// 	return
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				// c.Session.cancel()
				c.cancel()
				log.Println("ping error:", err)
				return nil
			}
		}
	}
}

func (c *Client) sendMessage(jsonBytes []byte) {

	if err := c.Conn.WriteMessage(websocket.TextMessage, jsonBytes); err != nil {
		// c.Session.cancel()
		c.cancel()
		log.Println("write error:", err)
		return
	}
}

// func (c *Client) Close() {
// 	log.Println("client closed")
// 	c.Session.Close()
// 	c.Conn.Close()
// }
