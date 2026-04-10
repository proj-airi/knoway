package openai

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"knoway.dev/api/filters/v1alpha1"
	"knoway.dev/pkg/object"
	"knoway.dev/pkg/types/openai"
)

// mockTTSRequest implements both object.LLMRequest and tts.Request for testing.
type mockTTSRequest struct {
	model string
}

func (m *mockTTSRequest) IsStream() bool                                              { return false }
func (m *mockTTSRequest) GetModel() string                                            { return m.model }
func (m *mockTTSRequest) SetModel(name string) error                                  { m.model = name; return nil }
func (m *mockTTSRequest) SetOverrideParams(map[string]*structpb.Value) error          { return nil }
func (m *mockTTSRequest) SetDefaultParams(map[string]*structpb.Value) error           { return nil }
func (m *mockTTSRequest) RemoveParamKeys([]string) error                              { return nil }
func (m *mockTTSRequest) GetRequestType() object.RequestType                          { return object.RequestTypeTextToSpeech }
func (m *mockTTSRequest) GetRawRequest() *http.Request                                { return nil }
func (m *mockTTSRequest) GetInput() string                                            { return "hello" }
func (m *mockTTSRequest) GetVoice() string                                            { return "alloy" }
func (m *mockTTSRequest) GetResponseFormat() *string                                  { return nil }
func (m *mockTTSRequest) GetSpeed() *float64                                          { return nil }
func (m *mockTTSRequest) GetExtraBody() map[string]any                                { return nil }
func (m *mockTTSRequest) GetBodyParsed() map[string]any                               { return nil }
func (m *mockTTSRequest) GetBodyBuffer() *bytes.Buffer                                { return nil }

func newTestResponseHandler() *responseHandler {
	return &responseHandler{
		cfg: &v1alpha1.OpenAIResponseHandlerConfig{},
	}
}

func TestUnmarshalResponseBody_TTS_ErrorBodyFromReader(t *testing.T) {
	handler := newTestResponseHandler()
	ctx := context.Background()
	req := &mockTTSRequest{model: "tts-1"}

	t.Run("reads error body from buffered reader not raw body", func(t *testing.T) {
		// Non-JSON body to verify reader is used (not rawResponse.Body).
		// If rawResponse.Body were read instead, body would be empty.
		errorBody := `voice not found`

		rawBody := io.NopCloser(bytes.NewReader(nil)) // empty - simulates already-consumed body
		reader := bufio.NewReader(bytes.NewReader([]byte(errorBody)))

		resp := &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       rawBody,
			Header:     http.Header{},
		}

		_, err := handler.UnmarshalResponseBody(ctx, nil, req, resp, reader, nil)
		require.Error(t, err)

		var errResp *openai.ErrorResponse
		require.ErrorAs(t, err, &errResp)
		assert.True(t, errResp.FromUpstream)
		assert.Contains(t, errResp.ErrorBody.Message, errorBody, "body content should be read from reader, not rawResponse.Body")
		assert.Equal(t, errorBody, errResp.UpstreamErrorBody)
	})

	t.Run("non-JSON error body is preserved in upstream error", func(t *testing.T) {
		// Microsoft Speech Service may return plain text or XML errors
		errorBody := `SSML voice name is invalid`

		rawBody := io.NopCloser(bytes.NewReader(nil))
		reader := bufio.NewReader(bytes.NewReader([]byte(errorBody)))

		resp := &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       rawBody,
			Header:     http.Header{},
		}

		_, err := handler.UnmarshalResponseBody(ctx, nil, req, resp, reader, nil)
		require.Error(t, err)

		var errResp *openai.ErrorResponse
		require.ErrorAs(t, err, &errResp)
		assert.True(t, errResp.FromUpstream)
		assert.Equal(t, errorBody, errResp.UpstreamErrorBody)
		assert.Contains(t, errResp.ErrorBody.Message, "400")
		assert.Contains(t, errResp.ErrorBody.Message, errorBody)
	})

	t.Run("empty error body still returns structured error", func(t *testing.T) {
		rawBody := io.NopCloser(bytes.NewReader(nil))
		reader := bufio.NewReader(bytes.NewReader(nil))

		resp := &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       rawBody,
			Header:     http.Header{},
		}

		_, err := handler.UnmarshalResponseBody(ctx, nil, req, resp, reader, nil)
		require.Error(t, err)

		var errResp *openai.ErrorResponse
		require.ErrorAs(t, err, &errResp)
		assert.True(t, errResp.FromUpstream)
		assert.Contains(t, errResp.ErrorBody.Message, "400")
	})
}
