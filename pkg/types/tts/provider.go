package tts

import (
	"context"
	"net/http"

	"knoway.dev/pkg/object"
)

type SpeechProvider interface {
	Name() string
	BuildSpeechRequest(ctx context.Context, baseURL string, authHeader string, req Request, upstreamHeaders http.Header, downstreamHeaders http.Header) (*http.Request, error)
	ParseSpeechResponse(resp *http.Response, model string) (object.LLMResponse, error)
}

type WebSocketSpeechProvider interface {
	Name() string
	DoSpeech(ctx context.Context, authHeader string, req Request) (object.LLMResponse, error)
}
