package tts

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/samber/lo"

	v1alpha1 "knoway.dev/api/clusters/v1alpha1"
	clustermanager "knoway.dev/pkg/clusters/manager"
	"knoway.dev/pkg/filters/auth"
	"knoway.dev/pkg/metadata"
)

type Voice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Locale   string `json:"locale,omitempty"`
	Gender   string `json:"gender,omitempty"`
}

type VoicesResponse struct {
	Object string  `json:"object"`
	Data   []Voice `json:"data"`
}

// Hardcoded OpenAI TTS voices
var openAIVoices = []Voice{
	{ID: "alloy", Name: "Alloy"},
	{ID: "ash", Name: "Ash"},
	{ID: "coral", Name: "Coral"},
	{ID: "echo", Name: "Echo"},
	{ID: "fable", Name: "Fable"},
	{ID: "nova", Name: "Nova"},
	{ID: "onyx", Name: "Onyx"},
	{ID: "sage", Name: "Sage"},
	{ID: "shimmer", Name: "Shimmer"},
}

type microsoftVoice struct {
	ShortName   string `json:"ShortName"`
	DisplayName string `json:"DisplayName"`
	Gender      string `json:"Gender"`
	Locale      string `json:"Locale"`
}

// listMicrosoftVoices calls the Microsoft cognitive services voices list API.
func listMicrosoftVoices(region string, subscriptionKey string) ([]Voice, error) {
	voicesURL := fmt.Sprintf("https://%s.tts.speech.microsoft.com/cognitiveservices/voices/list", region)

	req, err := http.NewRequest(http.MethodGet, voicesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Ocp-Apim-Subscription-Key", subscriptionKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch voices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var msVoices []microsoftVoice
	if err := json.NewDecoder(resp.Body).Decode(&msVoices); err != nil {
		return nil, fmt.Errorf("failed to decode voices: %w", err)
	}

	voices := make([]Voice, 0, len(msVoices))
	for _, v := range msVoices {
		voices = append(voices, Voice{
			ID:       v.ShortName,
			Name:     v.DisplayName,
			Provider: v1alpha1.ClusterProvider_MICROSOFT_SPEECH_SERVICE_V1.String(),
			Locale:   v.Locale,
			Gender:   v.Gender,
		})
	}

	return voices, nil
}

// extractRegionFromURL extracts region from a Microsoft TTS upstream URL.
// Expected format: https://{region}.tts.speech.microsoft.com/cognitiveservices/v1
func extractRegionFromURL(upstreamURL string) (string, error) {
	u, err := url.Parse(upstreamURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse upstream URL: %w", err)
	}

	// Host looks like "{region}.tts.speech.microsoft.com"
	parts := strings.SplitN(u.Hostname(), ".", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("unexpected hostname format: %s", u.Hostname())
	}

	return parts[0], nil
}

// listVoicesForCluster returns voices available for a given cluster based on its provider.
func listVoicesForCluster(cluster *v1alpha1.Cluster) ([]Voice, error) {
	switch cluster.GetProvider() {
	case v1alpha1.ClusterProvider_OPEN_AI, v1alpha1.ClusterProvider_OPEN_AI_V1_SPEECH:
		provider := cluster.GetProvider().String()
		voices := make([]Voice, len(openAIVoices))
		copy(voices, openAIVoices)
		for i := range voices {
			voices[i].Provider = provider
		}

		return voices, nil

	case v1alpha1.ClusterProvider_MICROSOFT_SPEECH_SERVICE_V1:
		upstreamURL := cluster.GetUpstream().GetUrl()

		region, err := extractRegionFromURL(upstreamURL)
		if err != nil {
			return nil, fmt.Errorf("failed to extract region: %w", err)
		}

		var subscriptionKey string
		for _, h := range cluster.GetUpstream().GetHeaders() {
			if h.GetKey() == "Ocp-Apim-Subscription-Key" {
				subscriptionKey = h.GetValue()

				break
			}
		}

		if subscriptionKey == "" {
			return nil, fmt.Errorf("missing Ocp-Apim-Subscription-Key in upstream headers")
		}

		return listMicrosoftVoices(region, subscriptionKey)

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
			clusters = lo.Filter(clusters, func(item *v1alpha1.Cluster, index int) bool {
				return auth.CanAccessModel(item.GetName(), rMeta.AuthInfo.GetAllowModels(), rMeta.AuthInfo.GetDenyModels())
			})
		}
	}

	sort.Slice(clusters, func(i, j int) bool {
		return strings.Compare(clusters[i].GetName(), clusters[j].GetName()) < 0
	})

	// Aggregate voices from all clusters
	allVoices := make([]Voice, 0)

	for _, c := range clusters {
		voices, err := listVoicesForCluster(c)
		if err != nil {
			slog.Error("failed to list voices for cluster", "cluster", c.GetName(), "error", err)

			continue
		}

		allVoices = append(allVoices, voices...)
	}

	return VoicesResponse{
		Object: "list",
		Data:   allVoices,
	}, nil
}
