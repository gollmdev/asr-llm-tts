package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gollmdev/asr-llm-tts/ai/dag/dagtypes"
	openai "github.com/sashabaranov/go-openai"
)

// OpenAICompatible streams chat completions using OpenAI-compatible API endpoints.
// It supports OpenAI and DeepSeek by customizing BaseURL and API key.
type OpenAICompatible struct {
	Client *openai.Client
}

type OpenAIConfig struct {
	APIKey     string
	BaseURL    string // optional; for DeepSeek or self-hosted servers
	HTTPClient *http.Client
}

func NewOpenAICompatible(cfg OpenAIConfig) (*OpenAICompatible, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("missing API key")
	}
	config := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		config.BaseURL = cfg.BaseURL
	}
	if cfg.HTTPClient != nil {
		config.HTTPClient = cfg.HTTPClient
	} else {
		config.HTTPClient = &http.Client{Timeout: 90 * time.Second}
	}
	return &OpenAICompatible{Client: openai.NewClientWithConfig(config)}, nil
}

func (p *OpenAICompatible) StreamChat(ctx context.Context, model string, messages []dagtypes.Message) (<-chan TokenEvent, error) {
	in := make([]openai.ChatCompletionMessage, 0, len(messages))
	for _, m := range messages {
		in = append(in, openai.ChatCompletionMessage{Role: m.Role, Content: m.Content})
	}
	req := openai.ChatCompletionRequest{
		Model:    model,
		Messages: in,
		Stream:   true,
	}
	stream, err := p.Client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, err
	}
	ch := make(chan TokenEvent, 32)
	go func() {
		defer close(ch)
		defer stream.Close()
		for {
			resp, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					ch <- TokenEvent{Done: true}
					return
				}
				ch <- TokenEvent{Err: err}
				return
			}
			for _, choice := range resp.Choices {
				ch <- TokenEvent{Delta: choice.Delta.Content}
			}
		}
	}()
	return ch, nil
}
