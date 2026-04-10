package openai

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	v1alpha12 "knoway.dev/api/clusters/v1alpha1"

	"google.golang.org/protobuf/types/known/anypb"

	"knoway.dev/api/filters/v1alpha1"
	"knoway.dev/pkg/bootkit"
	clusterfilters "knoway.dev/pkg/clusters/filters"
	"knoway.dev/pkg/object"
	"knoway.dev/pkg/protoutils"
	"knoway.dev/pkg/types/openai"
	"knoway.dev/pkg/types/tts"
)

func NewResponseHandlerWithConfig(cfg *anypb.Any, _ bootkit.LifeCycle) (clusterfilters.ClusterFilter, error) {
	c, err := protoutils.FromAny(cfg, &v1alpha1.OpenAIResponseHandlerConfig{})
	if err != nil {
		return nil, fmt.Errorf("invalid config type %T", cfg)
	}

	return &responseHandler{
		cfg: c,
	}, nil
}

var _ clusterfilters.ClusterFilterResponseUnmarshaller = (*responseHandler)(nil)
var _ clusterfilters.ClusterFilterResponseModifier = (*responseHandler)(nil)

type responseHandler struct {
	clusterfilters.ClusterFilter

	cfg *v1alpha1.OpenAIResponseHandlerConfig
}

func (f *responseHandler) UnmarshalResponseBody(ctx context.Context, cluster *v1alpha12.Cluster, req object.LLMRequest, rawResponse *http.Response, reader *bufio.Reader, pre object.LLMResponse) (object.LLMResponse, error) {
	contentType := rawResponse.Header.Get("Content-Type")

	switch req.GetRequestType() {
	case
		object.RequestTypeChatCompletions,
		object.RequestTypeCompletions:
		switch {
		case strings.HasPrefix(contentType, "application/json"):
			return openai.NewChatCompletionResponse(req, rawResponse, reader)
		case strings.HasPrefix(contentType, "text/event-stream"):
			return openai.NewChatCompletionStreamResponse(req, rawResponse, reader)
		default:
			break
		}
	case
		object.RequestTypeImageGenerations:
		switch {
		case strings.HasPrefix(contentType, "application/json"):
			return openai.NewImageGenerationsResponse(ctx, req, rawResponse, reader,
				openai.NewImageGenerationsResponseWithUsage(cluster.GetMeteringPolicy()),
			)
		default:
			break
		}
	case object.RequestTypeTextToSpeech:
		if rawResponse.StatusCode >= http.StatusBadRequest {
			tryReadBody := new(bytes.Buffer)

			_, err := tryReadBody.ReadFrom(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to read body: %w", err)
			}

			_ = rawResponse.Body.Close()
			rawResponse.Body = io.NopCloser(bytes.NewBuffer(tryReadBody.Bytes()))

			errResp, err := openai.ParseErrorResponse(rawResponse, tryReadBody.Bytes())
			if err != nil || errResp == nil {
				upstreamBody := tryReadBody.String()
				errMsg := fmt.Sprintf("upstream returned status code %d", rawResponse.StatusCode)
				if upstreamBody != "" {
					errMsg += " with body " + upstreamBody
				}

				newErr := openai.NewErrorBadGateway().WithMessage(errMsg)
				newErr.FromUpstream = true
				newErr.UpstreamErrorBody = upstreamBody

				return nil, newErr
			}

			return nil, errResp
		}

		return tts.NewAudioResponseFromHTTP(rawResponse, req.GetModel()), nil
	default:
		return nil, fmt.Errorf("unsupported request type %s", req.GetRequestType())
	}

	if rawResponse.StatusCode >= http.StatusBadRequest {
		tryReadBody := new(bytes.Buffer)

		_, err := tryReadBody.ReadFrom(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read body: %w", err)
		}

		_ = rawResponse.Body.Close()
		rawResponse.Body = io.NopCloser(bytes.NewBuffer(tryReadBody.Bytes()))

		upstreamBody := tryReadBody.String()
		errMsg := fmt.Sprintf("upstream returned status code %d", rawResponse.StatusCode)
		if upstreamBody != "" {
			errMsg += " with body " + upstreamBody
		}

		newErr := openai.NewErrorBadGateway().WithMessage(errMsg)
		newErr.FromUpstream = true
		newErr.UpstreamErrorBody = upstreamBody

		return nil, newErr
	}

	return nil, fmt.Errorf("unsupported content type %s", contentType)
}

func (f *responseHandler) ResponseModifier(ctx context.Context, cluster *v1alpha12.Cluster, request object.LLMRequest, response object.LLMResponse) (object.LLMResponse, error) {
	err := response.SetModel(cluster.GetName())
	if err != nil {
		return response, err
	}

	return response, nil
}
