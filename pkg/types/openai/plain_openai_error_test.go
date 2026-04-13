package openai

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpstreamPlainOpenAIError(t *testing.T) {
	body := `{"error":{"message":"This model is not available in your region.","code":403}}`
	resp := &http.Response{
		StatusCode: 403,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Request:    &http.Request{URL: &url.URL{Path: "/v1/chat/completions"}},
	}
	errResp, err := ParseErrorResponse(resp, []byte(body))
	require.NoError(t, err)
	require.NotNil(t, errResp)
	assert.Equal(t, 403, errResp.Status)
	assert.True(t, errResp.FromUpstream)
	assert.Equal(t, "This model is not available in your region.", errResp.ErrorBody.Message)
	assert.NotNil(t, errResp.ErrorBody.Code)
	assert.Equal(t, "403", *errResp.ErrorBody.Code)
}
