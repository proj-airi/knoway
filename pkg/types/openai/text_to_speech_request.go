package openai

import (
	"bytes"
	"fmt"
	"net/http"

	jsonpatch "github.com/evanphx/json-patch/v5"
	structpb "github.com/golang/protobuf/ptypes/struct"

	"knoway.dev/pkg/object"
	"knoway.dev/pkg/utils"
)

var _ object.LLMRequest = (*TextToSpeechRequest)(nil)

// TextToSpeechRequest represents OpenAI-compatible text-to-speech requests.
// API reference: https://platform.openai.com/docs/api-reference/audio/createSpeech
type TextToSpeechRequest struct {
	Model          string         `json:"model,omitempty"`
	Input          string         `json:"input,omitempty"`
	Voice          string         `json:"voice,omitempty"`
	ResponseFormat *string        `json:"response_format,omitempty"`
	Speed          *float64       `json:"speed,omitempty"`
	ExtraBody      map[string]any `json:"extra_body,omitempty"`

	bodyParsed      map[string]any
	bodyBuffer      *bytes.Buffer
	incomingRequest *http.Request
}

func NewTextToSpeechRequest(httpRequest *http.Request) (*TextToSpeechRequest, error) {
	buffer, parsed, err := utils.ReadAsJSONWithClose(httpRequest.Body)
	if err != nil {
		return nil, NewErrorInvalidBody()
	}

	req := &TextToSpeechRequest{
		Model:           utils.GetByJSONPath[string](parsed, "{ .model }"),
		Input:           utils.GetByJSONPath[string](parsed, "{ .input }"),
		Voice:           utils.GetByJSONPath[string](parsed, "{ .voice }"),
		ResponseFormat:  utils.GetByJSONPath[*string](parsed, "{ .response_format }"),
		Speed:           utils.GetByJSONPath[*float64](parsed, "{ .speed }"),
		ExtraBody:       utils.GetByJSONPath[map[string]any](parsed, "{ .extra_body }"),
		bodyParsed:      parsed,
		bodyBuffer:      buffer,
		incomingRequest: httpRequest,
	}

	return req, nil
}

func (r *TextToSpeechRequest) MarshalJSON() ([]byte, error) {
	return r.bodyBuffer.Bytes(), nil
}

func (r *TextToSpeechRequest) IsStream() bool {
	return false
}

func (r *TextToSpeechRequest) GetModel() string {
	return r.Model
}

func (r *TextToSpeechRequest) GetInput() string {
	return r.Input
}

func (r *TextToSpeechRequest) GetVoice() string {
	return r.Voice
}

func (r *TextToSpeechRequest) GetResponseFormat() *string {
	return r.ResponseFormat
}

func (r *TextToSpeechRequest) GetSpeed() *float64 {
	return r.Speed
}

func (r *TextToSpeechRequest) GetExtraBody() map[string]any {
	return r.ExtraBody
}

func (r *TextToSpeechRequest) SetModel(model string) error {
	var err error

	r.bodyBuffer, r.bodyParsed, err = modifyBufferBodyAndParsed(r.bodyBuffer, nil, NewReplace("/model", model))
	if err != nil {
		return err
	}

	r.Model = model

	return nil
}

func (r *TextToSpeechRequest) SetDefaultParams(params map[string]*structpb.Value) error {
	for k, v := range params {
		if _, exists := r.bodyParsed[k]; exists {
			continue
		}

		var err error

		r.bodyBuffer, r.bodyParsed, err = modifyBufferBodyAndParsed(r.bodyBuffer, nil, NewAdd("/"+k, &v))
		if err != nil {
			return fmt.Errorf("failed to add key %s: %w", k, err)
		}
	}

	changedModel := r.bodyParsed["model"]
	if model, ok := changedModel.(string); ok && r.Model != model {
		r.Model = model
	}

	return nil
}

func (r *TextToSpeechRequest) SetOverrideParams(params map[string]*structpb.Value) error {
	applyOpt := jsonpatch.NewApplyOptions()
	applyOpt.EnsurePathExistsOnAdd = true

	for k, v := range params {
		var err error

		r.bodyBuffer, r.bodyParsed, err = modifyBufferBodyAndParsed(r.bodyBuffer, applyOpt, NewAdd("/"+k, &v))
		if err != nil {
			return err
		}
	}

	changedModel := r.bodyParsed["model"]
	if model, ok := changedModel.(string); ok && r.Model != model {
		r.Model = model
	}

	return nil
}

func (r *TextToSpeechRequest) RemoveParamKeys(keys []string) error {
	applyOpt := jsonpatch.NewApplyOptions()
	applyOpt.AllowMissingPathOnRemove = true

	for _, v := range keys {
		var err error

		r.bodyBuffer, r.bodyParsed, err = modifyBufferBodyAndParsed(r.bodyBuffer, applyOpt, NewRemove("/"+v))
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *TextToSpeechRequest) GetRequestType() object.RequestType {
	return object.RequestTypeTextToSpeech
}

func (r *TextToSpeechRequest) GetRawRequest() *http.Request {
	return r.incomingRequest
}

func (r *TextToSpeechRequest) GetBodyParsed() map[string]any {
	return r.bodyParsed
}

func (r *TextToSpeechRequest) GetBodyBuffer() *bytes.Buffer {
	return r.bodyBuffer
}
