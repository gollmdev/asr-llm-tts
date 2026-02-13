package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gollmdev/asr-llm-tts/ai/ws"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 生产中可做跨域或 token 检查
	},
}

func ServeWs(hub *ws.Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrader failed: ", err)
		return
	}

	session := ws.NewSession()

	client := &ws.Client{Hub: hub, Conn: conn, Session: session}
	client.Hub.Register <- client

	go client.ReadPump()
	go client.WritePump()

}

func main() {
	router := gin.Default()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	hub := ws.NewHub()
	go hub.Run()
	router.GET("/ws", func(c *gin.Context) {
		ServeWs(hub, c.Writer, c.Request)
	})
	addr := ":8081"
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		log.Printf("HTTP server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()
	<-ctx.Done()
	log.Println("shutting down...")

}
