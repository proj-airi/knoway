package tts

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/samber/lo"

	"knoway.dev/pkg/object"
)

func ParseUpstreamError(resp *http.Response, body []byte) error {
	if resp == nil {
		return object.NewErrorBadGateway(errors.New("upstream response is nil"))
	}

	var parsed map[string]any
	err := json.Unmarshal(body, &parsed)
	if err != nil {
		return object.NewErrorBadGateway(err)
	}

	errorMap, ok := parsed["error"].(map[string]any)
	if ok {
		var codePtr *object.LLMErrorCode
		if code, ok := errorMap["code"].(string); ok {
			codePtr = lo.ToPtr(object.LLMErrorCode(code))
		}

		message := resp.Status
		if msg, ok := errorMap["message"].(string); ok && msg != "" {
			message = msg
		}

		return &object.BaseLLMError{
			Status: resp.StatusCode,
			ErrorBody: &object.BaseError{
				Code:    codePtr,
				Message: message,
			},
		}
	}

	return object.NewErrorBadGateway(errors.New("upstream error: " + resp.Status))
}

func ReadBodyError(resp *http.Response) error {
	if resp == nil {
		return object.NewErrorBadGateway(errors.New("upstream response is nil"))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return object.NewErrorBadGateway(err)
	}

	return ParseUpstreamError(resp, body)
}
