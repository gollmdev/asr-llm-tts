package ws

import (
	"context"
	"log"
)

type Hub struct {
	clients       map[*Client]bool
	broadcastByte chan []byte
	Register      chan *Client
	unregister    chan *Client
}

func NewHub() *Hub {
	hub := Hub{
		clients:       make(map[*Client]bool),
		broadcastByte: make(chan []byte),
		Register:      make(chan *Client),
		unregister:    make(chan *Client),
	}
	return &hub
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Hub received shutdown signal")
			return

		case client := <-h.Register:
			h.clients[client] = true
			log.Println("新客户端加入，当前在线:", len(h.clients))
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				// delete map item by key
				delete(h.clients, client)
				// close(client.send)
				log.Println("客户端离开，当前在线:", len(h.clients))
			}
		case message := <-h.broadcastByte:
			for client := range h.clients {
				client.Session.PublishBinaryStream(message)
				// if !client.session.TryPushAudio(message) {
				// 	// close(client.send)
				// 	client.session.Close()
				// 	delete(h.clients, client)
				// }

				// select {
				// case client.send <- message:
				// default:
				// 	close(client.send)
				// 	delete(h.clients, client)

				// }
			}
		}
	}
}
