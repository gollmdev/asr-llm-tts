package node

import (
	"crypto/rand"
	"encoding/hex"
	"log"

	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
	"github.com/gollmdev/asr-llm-tts/ai/dag/dagtypes"
)

type ConversationContextNode struct {
	RecentLimit int
}

func (n *ConversationContextNode) ID() string { return "conversation_context" }
func (n *ConversationContextNode) Mode() dag.NodeMode {
	return dag.ModeLazy
}

func (n *ConversationContextNode) Run(rt dag.NodeRuntime) error {
	recentLimit := n.RecentLimit
	if recentLimit <= 0 {
		recentLimit = 12
	}
	// rtx := rt.RuntimeContext()

	// sessionID := rtx.SessionID
	rtx := rt.RuntimeContext()
	sessionID := rtx.SessionID
	ctx := rt.Context()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-rt.Input():
			if !ok {
				return nil
			}
			msgs, ok := ev.Data.([]*dagtypes.Message)
			if !ok || len(msgs) == 0 {
				log.Println("[conversation_context] invalid or empty input")
				continue
			}

			last := msgs[len(msgs)-1]
			if last == nil || last.Role != "user" {
				log.Println("[conversation_context] last message is not user")
				continue
			}

			if err := rtx.Memory.Append(sessionID, last); err != nil {
				log.Printf("[conversation_context] append memory error: %v", err)
			}
			history, err := rtx.Memory.GetRecent(sessionID, recentLimit)
			if err != nil {
				log.Printf("[conversation_context] get recent history error: %v", err)
				history = []*dagtypes.Message{last}
			}
			summary, err := rtx.Memory.GetSummary(sessionID)
			if err != nil {
				log.Printf("[conversation_context] get summary error: %v", err)
				summary = ""
			}
			// id := uuid.New()

			tc := &dagtypes.TurnContext{
				SessionID: sessionID,
				TurnID:    randomTurnID(),
				UserInput: last,
				History:   history,
				Summary:   summary,
				Metadata: map[string]any{
					"source_event": ev.Type,
				},
			}
			rt.Emit(&dag.Event{
				Type: "context_ready",
				Data: tc,
			})

			return nil
		}
	}
}

func randomTurnID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
