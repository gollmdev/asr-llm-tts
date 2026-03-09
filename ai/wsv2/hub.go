package wsv2

import (
	"context"
	"log"
)

type Hub struct {
	clients       map[*Client]bool
	userClients   map[int64]map[*Client]struct{}
	broadcastByte chan []byte
	Register      chan *Client
	unregister    chan *Client
	maxPerUser    int
}

func NewHub() *Hub {
	hub := Hub{
		clients:       make(map[*Client]bool),
		userClients:   make(map[int64]map[*Client]struct{}),
		broadcastByte: make(chan []byte),
		Register:      make(chan *Client),
		unregister:    make(chan *Client),
		maxPerUser:    3, // 每个用户最大连接数
	}
	return &hub
}

func (h *Hub) delete(client *Client) {
	delete(h.clients, client)

	uid := client.userId
	if group, ok := h.userClients[uid]; ok {
		delete(group, client)

		if len(group) == 0 {
			delete(h.userClients, uid)
		}
	}
	// close(client.send)
	log.Printf("ws-conn: 客户端 %d 离开，当前在线: %d", uid, len(h.clients))
}
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Hub received shutdown signal")
			return

		case client := <-h.Register:
			uid := client.userId

			if h.userClients[uid] == nil {
				h.userClients[uid] = make(map[*Client]struct{})
			}
			if len(h.userClients[uid]) >= h.maxPerUser {
				log.Println("ws-conn: 用户达到最大连接数，拒绝新客户端")
				// client.Session.cancel()
				// client.cancel()

				client.registerCh <- false

				for c := range h.userClients[uid] {

					// c.sendMessage(buildMessage("many_connections", "连接数已达上限, 系统关闭旧连接!"))
					// c.Session.SendSafe(session.SessionText, session.BuildMessage("many_connections", "连接数已达上限, 系统关闭旧连接!"))
					c.cancel()
					// h.delete(c)
					break
				}
				continue
			}
			h.clients[client] = true
			h.userClients[uid][client] = struct{}{}
			client.registerCh <- true
			log.Printf("ws-conn: 新客户 %d 端加入，当前在线: %d", uid, len(h.clients))
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				// delete map item by key
				h.delete(client)
			}
			// case message := <-h.broadcastByte:
			// 	for client := range h.clients {
			// 		client.PublishBinaryStream(message)
			// 		// if !client.session.TryPushAudio(message) {
			// 		// 	// close(client.send)
			// 		// 	client.session.Close()
			// 		// 	delete(h.clients, client)
			// 		// }

			// 		// select {
			// 		// case client.send <- message:
			// 		// default:
			// 		// 	close(client.send)
			// 		// 	delete(h.clients, client)

			// 		// }
			// 	}
		}
	}
}
