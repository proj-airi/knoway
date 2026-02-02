package seedspeechv1

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"knoway.dev/pkg/object"
	"knoway.dev/pkg/types/openai"
	"knoway.dev/pkg/types/tts"
	"knoway.dev/pkg/utils"
)

const (
	defaultVolcengineSpeechURL = "https://openspeech.bytedance.com/api/v1/tts"
)

type speechRequestOptionsApp struct {
	AppID   string `json:"appid"`
	Token   string `json:"token"`
	Cluster string `json:"cluster"`
}

type speechRequestOptionsUser struct {
	UserID string `json:"uid"`
}

type speechRequestOptionsAudio struct {
	VoiceType        string   `json:"voice_type"`
	Emotion          *string  `json:"emotion,omitempty"`
	EnableEmotion    *bool    `json:"enable_emotion,omitempty"`
	EmotionScale     *float64 `json:"emotion_scale,omitempty"`
	Encoding         *string  `json:"encoding,omitempty"`
	SpeedRatio       *float64 `json:"speed_ratio,omitempty"`
	Rate             *int     `json:"rate,omitempty"`
	BitRate          *int     `json:"bit_rate,omitempty"`
	ExplicitLanguage *string  `json:"explicit_language,omitempty"`
	ContextLanguage  *string  `json:"context_language,omitempty"`
	LoudnessRatio    *float64 `json:"loudness_ratio,omitempty"`
}

type speechRequestOptionsRequest struct {
	RequestID             string         `json:"reqid"`
	Text                  string         `json:"text"`
	TextType              *string        `json:"text_type,omitempty"`
	SilenceDuration       *float64       `json:"silence_duration,omitempty"`
	WithTimestamp         *string        `json:"with_timestamp,omitempty"`
	Operation             *string        `json:"operation,omitempty"`
	ExtraParam            *string        `json:"extra_param,omitempty"`
	DisableMarkdownFilter *bool          `json:"disable_markdown_filter,omitempty"`
	EnableLatexTone       *bool          `json:"enable_latex_tn,omitempty"`
	CacheConfig           map[string]any `json:"cache_config,omitempty"`
	UseCache              *bool          `json:"use_cache,omitempty"`
}

type speechRequestOptions struct {
	App     speechRequestOptionsApp     `json:"app"`
	User    speechRequestOptionsUser    `json:"user"`
	Audio   speechRequestOptionsAudio   `json:"audio"`
	Request speechRequestOptionsRequest `json:"request"`
}

func BuildSpeechRequest(ctx context.Context, baseURL string, authHeader string, req tts.Request, _ http.Header, _ http.Header) (*http.Request, error) {
	if baseURL == "" {
		baseURL = defaultVolcengineSpeechURL
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")

	cluster := utils.GetByJSONPath[string](req.GetExtraBody(), "{ .app.cluster }")
	if cluster == "" {
		cluster = "volcano_tts"
	}

	userID := utils.GetByJSONPath[string](req.GetExtraBody(), "{ .user.uid }")
	if userID == "" {
		userID = uuid.New().String()
	}

	requestID := utils.GetByJSONPath[string](req.GetExtraBody(), "{ .request.reqid }")
	if requestID == "" {
		requestID = uuid.New().String()
	}

	operation := utils.GetByJSONPath[*string](req.GetExtraBody(), "{ .request.operation }")
	if operation == nil || *operation == "" {
		operation = lo.ToPtr("query")
	}

	speedRatio := utils.GetByJSONPath[*float64](req.GetExtraBody(), "{ .audio.speed_ratio }")
	if speedRatio == nil || *speedRatio == 0 {
		speedRatio = lo.ToPtr(1.0)
	}

	newReqParams := &speechRequestOptions{
		App: speechRequestOptionsApp{
			AppID:   utils.GetByJSONPath[string](req.GetExtraBody(), "{ .app.appid }"),
			Token:   token,
			Cluster: cluster,
		},
		User: speechRequestOptionsUser{
			UserID: userID,
		},
		Audio: speechRequestOptionsAudio{
			VoiceType:        req.GetVoice(),
			Emotion:          utils.GetByJSONPath[*string](req.GetExtraBody(), "{ .audio.emotion }"),
			EnableEmotion:    utils.GetByJSONPath[*bool](req.GetExtraBody(), "{ .audio.enable_emotion }"),
			EmotionScale:     utils.GetByJSONPath[*float64](req.GetExtraBody(), "{ .audio.emotion_scale }"),
			Encoding:         lo.Ternary(req.GetResponseFormat() != nil && *req.GetResponseFormat() != "", lo.ToPtr(*req.GetResponseFormat()), lo.ToPtr("mp3")),
			SpeedRatio:       speedRatio,
			Rate:             utils.GetByJSONPath[*int](req.GetExtraBody(), "{ .audio.rate }"),
			BitRate:          utils.GetByJSONPath[*int](req.GetExtraBody(), "{ .audio.bit_rate }"),
			ExplicitLanguage: utils.GetByJSONPath[*string](req.GetExtraBody(), "{ .audio.explicit_language }"),
			ContextLanguage:  utils.GetByJSONPath[*string](req.GetExtraBody(), "{ .audio.context_language }"),
			LoudnessRatio:    utils.GetByJSONPath[*float64](req.GetExtraBody(), "{ .audio.loudness_ratio }"),
		},
		Request: speechRequestOptionsRequest{
			RequestID:             requestID,
			Text:                  req.GetInput(),
			TextType:              utils.GetByJSONPath[*string](req.GetExtraBody(), "{ .request.text_type }"),
			SilenceDuration:       utils.GetByJSONPath[*float64](req.GetExtraBody(), "{ .request.silence_duration }"),
			WithTimestamp:         utils.GetByJSONPath[*string](req.GetExtraBody(), "{ .request.with_timestamp }"),
			Operation:             operation,
			ExtraParam:            utils.GetByJSONPath[*string](req.GetExtraBody(), "{ .request.extra_param }"),
			DisableMarkdownFilter: utils.GetByJSONPath[*bool](req.GetExtraBody(), "{ .request.disable_markdown_filter }"),
			EnableLatexTone:       utils.GetByJSONPath[*bool](req.GetExtraBody(), "{ .request.enable_latex_tn }"),
			CacheConfig:           utils.GetByJSONPath[map[string]any](req.GetExtraBody(), "{ .request.cache_config }"),
			UseCache:              utils.GetByJSONPath[*bool](req.GetExtraBody(), "{ .request.use_cache }"),
		},
	}

	jsonBytes, err := json.Marshal(newReqParams)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer;"+token)

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

	audioBase64String := utils.GetByJSONPath[string](resBody, "{ .data }")
	if audioBase64String == "" {
		return nil, openai.NewErrorBadGateway().WithMessage("upstream returned empty audio base64 string")
	}

	audioBytes, err := base64.StdEncoding.DecodeString(audioBase64String)
	if err != nil {
		return nil, openai.NewErrorBadGateway().WithMessage("failed to decode audio base64 string")
	}

	return tts.NewAudioResponseFromBytes(http.StatusOK, "audio/mp3", model, audioBytes), nil
}
