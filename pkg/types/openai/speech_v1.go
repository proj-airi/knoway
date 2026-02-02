package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"knoway.dev/pkg/object"
	"knoway.dev/pkg/types/tts"
)

const (
	defaultOpenAISpeechURL = "https://api.openai.com/v1/audio/speech"
)

func BuildSpeechRequest(ctx context.Context, baseURL string, authHeader string, req tts.Request, _ http.Header, _ http.Header) (*http.Request, error) {
	if baseURL == "" {
		baseURL = defaultOpenAISpeechURL
	}

	var payload []byte
	if buffer := req.GetBodyBuffer(); buffer != nil {
		payload = buffer.Bytes()
	} else {
		var err error

		payload, err = json.Marshal(req.GetBodyParsed())
		if err != nil {
			return nil, err
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", authHeader)
	httpReq.Header.Set("Content-Type", "application/json")

	return httpReq, nil
}

func ParseSpeechResponse(resp *http.Response, model string) (object.LLMResponse, error) {
	if resp == nil {
		return nil, NewErrorBadGateway().WithMessage("upstream response is nil")
	}

	if resp.StatusCode >= http.StatusBadRequest {
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, NewErrorBadGateway().WithMessage("upstream error: " + resp.Status)
		}

		errResp, err := ParseErrorResponse(resp, body)
		if err != nil || errResp == nil {
			return nil, NewErrorBadGateway().WithMessage("upstream error: " + resp.Status)
		}

		return nil, errResp
	}

	return tts.NewAudioResponseFromHTTP(resp, model), nil
}
