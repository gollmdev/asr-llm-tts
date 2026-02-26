package ws

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
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
	Session *Session
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
func (c *Client) ReadPump() {
	defer func() {
		// On exit, unregister the client and close the connection
		// send c to unregister channel
		// c.session.Close()
		// close(c.send)
		// close the session
		// close the connection
		// c.conn.Close()
		log.Println("close readPump!")
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

	c.Session.LLMConsumer()
	c.Session.MonitorSubSize()
	// c.Session.LLMTaskConsumer() // 启动 LLM 任务消费者

	for {
		select {
		// case <-c.session.Done:
		// 	log.Println("Session processing done, exiting readPump.")
		// 	return
		case <-c.Session.ctx.Done():
			log.Println("Session is closed, exiting readPump.")
			return
		default:
			// broadcast the received message to all clients
			// c.hub.broadcast <- message

		}
		msgType, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("read error: %v", err)
				c.Session.cancel()
			} else {
				log.Printf("read info: %v", err)
			}

			return
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
				c.Session.PublishTextStream(string(message))
			} else {
				if objType, ok := obj["type"].(string); ok {
					switch objType {
					case "config":
						if config, ok := obj["config"].(map[string]any); ok {
							if tts, ok := config["tts"].(bool); ok {
								if tts {
									c.Session.ttsEnabled = true
								}
							}
						}
					case "message":

						if text, ok := obj["content"].(string); ok {
							c.Session.PublishTextStream(text)
						}

					case "control":
						if config, ok := obj["config"].(map[string]any); ok {
							if tts, ok := config["tts"].(bool); ok {
								if tts != c.Session.ttsEnabled {

									if tts {
										c.Session.ttsEnabled = true
										// c.Session.waitConsumers("tts")
										c.Session.TTsConsumer()
										log.Println("tts open!")
									} else {
										c.Session.ttsEnabled = false
										// c.Session.CancelConsumer("tts")
										c.Session.UnSubscribe(TTS)
										log.Println("tts close!")
									}
								}

							}
						}
					}
				}

			}
			// return

		case websocket.BinaryMessage:
			// process binary message

			c.Session.PublishBinaryStream(message)

		default:
			log.Printf("Received unsupported message type: %d", msgType)
		}

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

func (c *Client) WritePump() {
	ticker := time.NewTicker(50 * time.Second)
	defer func() {

		c.Session.Close()
		ticker.Stop()
		c.Conn.Close()
		c.Hub.unregister <- c
		log.Println("close writePump!")
	}()
	for {
		select {

		case message, ok := <-c.Session.output:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Hub 关闭 channel
				// c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				log.Println("Session output channel closed.")
				return
			}
			switch message.Type {
			case SessionText:
				log.Printf("Sending text message: %s", string(message.Data))
				c.sendMessage(message.Data)
			case SessionAudio:
				// log.Printf("Sending text message: %s", string(message.Data))
				// if err := c.conn.WriteMessage(websocket.TextMessage, message.Data); err != nil {
				// 	log.Println("write error:", err)
				// 	return
				// }
				log.Printf("Sending audio message of length %d", len(message.Data))

				if err := c.Conn.WriteMessage(websocket.BinaryMessage, message.Data); err != nil {
					c.Session.cancel()
					log.Println("write error:", err)

					return
				}
			}

			// if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			// 	log.Println("write error:", err)
			// 	return
			// }
		case <-c.Session.ctx.Done(): // 监听 session 的结束信号
			log.Println("Session is closed, exiting writePump.")
			return
		// case <-c.session.Done:
		// 	log.Println("Session processing done, exiting writePump.")
		// 	return
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.Session.cancel()
				log.Println("ping error:", err)
				return
			}
		}
	}
}

func (c *Client) sendMessage(jsonBytes []byte) {

	if err := c.Conn.WriteMessage(websocket.TextMessage, jsonBytes); err != nil {
		c.Session.cancel()
		log.Println("write error:", err)
		return
	}
}

// func (c *Client) Close() {
// 	log.Println("client closed")
// 	c.Session.Close()
// 	c.Conn.Close()
// }
