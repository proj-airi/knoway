package tts

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/moeru-ai/unspeech/pkg/backend/alibaba"
	"github.com/moeru-ai/unspeech/pkg/backend/deepgram"
	"github.com/moeru-ai/unspeech/pkg/backend/elevenlabs"
	"github.com/moeru-ai/unspeech/pkg/backend/microsoft"
	"github.com/moeru-ai/unspeech/pkg/backend/openai"
	"github.com/moeru-ai/unspeech/pkg/backend/types"
	"github.com/moeru-ai/unspeech/pkg/backend/volcengine"
	"github.com/samber/lo"

	v1alpha1 "knoway.dev/api/clusters/v1alpha1"
	clustermanager "knoway.dev/pkg/clusters/manager"
	"knoway.dev/pkg/filters/auth"
	"knoway.dev/pkg/metadata"
)

// headerValue returns the first matching header value from a cluster's upstream config.
// Keys are matched case-insensitively because cluster CRDs don't canonicalise them.
func headerValue(cluster *v1alpha1.Cluster, key string) string {
	for _, h := range cluster.GetUpstream().GetHeaders() {
		if strings.EqualFold(h.GetKey(), key) {
			return h.GetValue()
		}
	}

	return ""
}

// bearerToken strips a "Bearer " prefix from an Authorization header value.
func bearerToken(cluster *v1alpha1.Cluster) string {
	return strings.TrimPrefix(headerValue(cluster, "Authorization"), "Bearer ")
}

// microsoftRegion extracts the Azure region subdomain from an upstream URL
// shaped like https://{region}.tts.speech.microsoft.com/...
func microsoftRegion(upstreamURL string) (string, error) {
	u, err := url.Parse(upstreamURL)
	if err != nil {
		return "", fmt.Errorf("parse upstream URL: %w", err)
	}

	host, _, _ := strings.Cut(u.Hostname(), ".")
	if host == "" {
		return "", fmt.Errorf("unexpected hostname format: %s", u.Hostname())
	}

	return host, nil
}

// listVoicesForCluster dispatches to the right unspeech provider for this cluster's upstream.
// The cluster config supplies credentials; unspeech owns the provider-specific fetch+mapping.
func listVoicesForCluster(request *http.Request, cluster *v1alpha1.Cluster) ([]types.Voice, error) {
	ctx := request.Context()

	switch cluster.GetProvider() {
	case v1alpha1.ClusterProvider_OPEN_AI, v1alpha1.ClusterProvider_OPEN_AI_V1_SPEECH:
		return openai.ListVoices(ctx)

	case v1alpha1.ClusterProvider_MICROSOFT_SPEECH_SERVICE_V1:
		region, err := microsoftRegion(cluster.GetUpstream().GetUrl())
		if err != nil {
			return nil, err
		}

		key := headerValue(cluster, "Ocp-Apim-Subscription-Key")
		if key == "" {
			key = bearerToken(cluster)
		}

		return microsoft.ListVoices(ctx, microsoft.VoicesCredentials{
			Region:          region,
			SubscriptionKey: key,
		})

	case v1alpha1.ClusterProvider_ELEVEN_LABS_V1:
		apiKey := headerValue(cluster, "xi-api-key")
		if apiKey == "" {
			apiKey = bearerToken(cluster)
		}

		return elevenlabs.ListVoices(ctx, elevenlabs.VoicesCredentials{APIKey: apiKey})

	case v1alpha1.ClusterProvider_DEEPGRAM_WEBSOCKET_V1:
		apiKey := bearerToken(cluster)
		// Deepgram clusters may carry the key as "Authorization: Token xxx" instead of Bearer.
		apiKey = strings.TrimPrefix(apiKey, "Token ")

		return deepgram.ListVoices(ctx, deepgram.VoicesCredentials{APIKey: apiKey})

	case v1alpha1.ClusterProvider_ALIBABA_COSY_VOICE_SERVICE:
		return alibaba.ListVoices(ctx)

	case v1alpha1.ClusterProvider_VOLCENGINE_SEED_SPEECH_V1:
		return volcengine.ListVoices(ctx)

	default:
		return nil, nil
	}
}

func (l *OpenAITextToSpeechListener) listVoices(_ http.ResponseWriter, request *http.Request) (any, error) {
	// Run pre-filters (auth, etc.)
	for _, f := range l.filters.OnRequestPreFilters() {
		fResult := f.OnRequestPre(request.Context(), request)
		if fResult.IsFailed() {
			return nil, fResult.Error
		}
	}

	clusters := clustermanager.ListTTSClusters()

	// Apply auth filtering
	rMeta := metadata.RequestMetadataFromCtx(request.Context())

	if rMeta.EnabledAuthFilter {
		if rMeta.AuthInfo != nil {
			clusters = lo.Filter(clusters, func(item *v1alpha1.Cluster, _ int) bool {
				return auth.CanAccessModel(item.GetName(), rMeta.AuthInfo.GetAllowModels(), rMeta.AuthInfo.GetDenyModels())
			})
		}
	}

	sort.Slice(clusters, func(i, j int) bool {
		return strings.Compare(clusters[i].GetName(), clusters[j].GetName()) < 0
	})

	// Aggregate voices from all clusters into unspeech's rich schema.
	allVoices := make([]types.Voice, 0)

	for _, c := range clusters {
		voices, err := listVoicesForCluster(request, c)
		if err != nil {
			slog.Error("failed to list voices for cluster", "cluster", c.GetName(), "error", err)

			continue
		}

		// Align CompatibleModels with this cluster's knoway name so the frontend's
		// "does this voice support the selected model?" filter matches what
		// listModels() returns. unspeech sets upstream-specific values like "v1"
		// that don't correspond to any knoway cluster id.
		for i := range voices {
			voices[i].CompatibleModels = []string{c.GetName()}
		}

		allVoices = append(allVoices, voices...)
	}

	return types.ListVoicesResponse{Voices: allVoices}, nil
}
