package tts

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"knoway.dev/pkg/object"
	"knoway.dev/pkg/utils"
)

var _ object.LLMResponse = (*AudioResponse)(nil)

type AudioResponse struct {
	Status      int
	Model       string
	RequestID   string
	ContentType string

	BodyBytes []byte
	Body      io.ReadCloser
	Error     object.LLMError
}

func NewAudioResponseFromHTTP(resp *http.Response, model string) *AudioResponse {
	if resp == nil {
		return &AudioResponse{
			Status: http.StatusInternalServerError,
			Model:  model,
			Error:  object.NewErrorInternalError(errors.New("nil upstream response")),
		}
	}

	return &AudioResponse{
		Status:      resp.StatusCode,
		Model:       model,
		RequestID:   resp.Header.Get("x-request-id"),
		ContentType: resp.Header.Get("Content-Type"),
		Body:        resp.Body,
	}
}

func NewAudioResponseFromBytes(status int, contentType string, model string, body []byte) *AudioResponse {
	return &AudioResponse{
		Status:      status,
		Model:       model,
		ContentType: contentType,
		BodyBytes:   body,
	}
}

func (r *AudioResponse) MarshalJSON() ([]byte, error) {
	if r == nil {
		return []byte("null"), nil
	}

	if r.Error != nil {
		return json.Marshal(r.Error)
	}

	return json.Marshal(map[string]any{
		"status": r.Status,
		"model":  r.Model,
	})
}

func (r *AudioResponse) IsStream() bool {
	return false
}

func (r *AudioResponse) GetRequestID() string {
	return r.RequestID
}

func (r *AudioResponse) GetUsage() object.LLMUsage {
	return nil
}

func (r *AudioResponse) GetError() object.LLMError {
	return r.Error
}

func (r *AudioResponse) GetModel() string {
	return r.Model
}

func (r *AudioResponse) SetModel(modelName string) error {
	r.Model = modelName
	return nil
}

func (r *AudioResponse) GetStatus() int {
	if r == nil || r.Status == 0 {
		return http.StatusOK
	}

	return r.Status
}

func (r *AudioResponse) WriteTo(writer http.ResponseWriter) error {
	if r == nil {
		return nil
	}

	status := r.GetStatus()
	if r.ContentType == "" {
		r.ContentType = "audio/mpeg"
	}

	writer.Header().Set("Content-Type", r.ContentType)
	writer.WriteHeader(status)
	utils.SafeFlush(writer)

	if r.Body != nil {
		defer func() { _ = r.Body.Close() }()

		_, err := io.Copy(writer, r.Body)

		return err
	}

	if len(r.BodyBytes) > 0 {
		_, err := writer.Write(r.BodyBytes)
		return err
	}

	return nil
}
