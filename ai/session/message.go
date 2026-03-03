package session

import (
	"encoding/json"
	"log"
)

func BuildMessage(event string, text string) []byte {

	jsonBytes, err := json.Marshal(map[string]any{
		"event": event,
		"data": map[string]any{
			"content": text,
		},
	})
	if err != nil {
		log.Println("json marshal error:", err)
	}
	return jsonBytes
}
