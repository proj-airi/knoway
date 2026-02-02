package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"knoway.dev/pkg/object"
	"knoway.dev/pkg/types/openai"
	"knoway.dev/pkg/types/tts"
)

const (
	defaultElevenLabsSpeechURL = "https://api.elevenlabs.io/v1/text-to-speech"
)

func BuildSpeechRequest(ctx context.Context, baseURL string, authHeader string, req tts.Request, _ http.Header, _ http.Header) (*http.Request, error) {
	if baseURL == "" {
		baseURL = defaultElevenLabsSpeechURL
	}

	reqURL := baseURL

	parsed, err := url.Parse(baseURL)

	if err == nil {
		reqURL = parsed.JoinPath(req.GetVoice()).String()
	}

	payload := map[string]any{}
	for k, v := range req.GetBodyParsed() {
		payload[k] = v
	}

	delete(payload, "model")
	delete(payload, "voice")
	delete(payload, "input")

	payload["text"] = req.GetInput()
	payload["model_id"] = req.GetModel()

	if req.GetExtraBody() != nil {
		for k, v := range req.GetExtraBody() {
			payload[k] = v
		}
	}

	bs, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewBuffer(bs))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Xi-Api-Key", strings.TrimPrefix(authHeader, "Bearer "))
	httpReq.Header.Set("Content-Type", "application/json")

	return httpReq, nil
}

func ParseSpeechResponse(resp *http.Response, model string) (object.LLMResponse, error) {
	if resp == nil {
		return nil, openai.NewErrorBadGateway().WithMessage("upstream response is nil")
	}

	if resp.StatusCode >= http.StatusBadRequest {
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, openai.NewErrorBadGateway().WithMessage("upstream error: " + resp.Status)
		}

		return nil, tts.ParseUpstreamError(resp, body)
	}

	return tts.NewAudioResponseFromHTTP(resp, model), nil
}
