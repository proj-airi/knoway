package websocketv1

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
	defaultDeepgramSpeechURL = "https://api.deepgram.com/v1/speak"
)

func BuildSpeechRequest(ctx context.Context, baseURL string, authHeader string, req tts.Request, _ http.Header, _ http.Header) (*http.Request, error) {
	if baseURL == "" {
		baseURL = defaultDeepgramSpeechURL
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	if req.GetVoice() != "" {
		q.Set("model", req.GetVoice())
	}

	u.RawQuery = q.Encode()

	payload, err := json.Marshal(map[string]string{
		"text": req.GetInput(),
	})
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	auth := authHeader
	if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
		auth = "Token " + after
	}

	httpReq.Header.Set("Authorization", auth)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "audio/*")

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
