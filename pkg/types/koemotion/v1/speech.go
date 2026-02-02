package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/vincent-petithory/dataurl"

	"knoway.dev/pkg/object"
	"knoway.dev/pkg/types/openai"
	"knoway.dev/pkg/types/tts"
	"knoway.dev/pkg/utils"
)

const (
	defaultKoemotionSpeechURL = "https://api.rinna.co.jp/koemotion/infer"
)

func BuildSpeechRequest(ctx context.Context, baseURL string, authHeader string, req tts.Request, _ http.Header, _ http.Header) (*http.Request, error) {
	if baseURL == "" {
		baseURL = defaultKoemotionSpeechURL
	}

	payload := map[string]any{}
	for k, v := range req.GetBodyParsed() {
		payload[k] = v
	}

	delete(payload, "model")
	delete(payload, "voice")
	delete(payload, "input")

	payload["text"] = req.GetInput()

	if req.GetExtraBody() != nil {
		for k, v := range req.GetExtraBody() {
			payload[k] = v
		}
	}

	bs, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewBuffer(bs))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Ocp-Apim-Subscription-Key", strings.TrimPrefix(authHeader, "Bearer "))
	httpReq.Header.Set("Content-Type", "application/json")

	return httpReq, nil
}

func ParseSpeechResponse(resp *http.Response, model string) (object.LLMResponse, error) {
	if resp == nil {
		return nil, openai.NewErrorBadGateway().WithMessage("upstream response is nil")
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, openai.NewErrorBadGateway().WithMessage("upstream error: " + resp.Status)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, tts.ParseUpstreamError(resp, body)
	}

	var resBody map[string]any
	if err := json.Unmarshal(body, &resBody); err != nil {
		return nil, openai.NewErrorBadGateway().WithMessage("invalid upstream response")
	}

	audioDataURLString := utils.GetByJSONPath[string](resBody, "{ .audio }")
	if audioDataURLString == "" {
		return nil, openai.NewErrorBadGateway().WithMessage("upstream returned empty audio data URL")
	}

	audioDataURL, err := dataurl.DecodeString(audioDataURLString)
	if err != nil {
		return nil, openai.NewErrorBadGateway().WithMessage("failed to decode audio data URL")
	}

	return tts.NewAudioResponseFromBytes(http.StatusOK, "audio/mp3", model, audioDataURL.Data), nil
}
