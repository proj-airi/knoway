# Voices API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `GET /v1/audio/voices` endpoint that aggregates available TTS voices across all registered providers.

**Architecture:** New handler in the TTS listener that queries the cluster manager for TTS clusters, dispatches per-provider voice listing (hardcoded for OpenAI, REST API for Microsoft), and returns a unified JSON response. Follows the `listModels` pattern from the chat listener.

**Tech Stack:** Go, gorilla/mux, existing cluster manager, Microsoft Cognitive Services REST API

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `pkg/listener/manager/tts/voices.go` | Create | Voice types, per-provider listing, handler |
| `pkg/listener/manager/tts/voices_test.go` | Create | Unit tests for voice listing logic |
| `pkg/listener/manager/tts/listener.go` | Modify | Register `/v1/audio/voices` route |
| `pkg/clusters/manager/cluster.go` | Modify | Add `ListTTSClusters()` to expose TTS clusters with provider info |

---

### Task 1: Add `ListTTSClusters` to cluster manager

**Files:**
- Modify: `pkg/clusters/manager/cluster.go`

- [ ] **Step 1: Add `ListTTSClusters` function**

```go
// In pkg/clusters/manager/cluster.go, add after ListModels():

func ListTTSClusters() []*v1alpha1.Cluster {
	if clusterRegister == nil {
		return nil
	}

	return clusterRegister.ListTTSClusters()
}
```

And add the method to Register:

```go
// Add after ListModels() method on Register:

func (cr *Register) ListTTSClusters() []*v1alpha1.Cluster {
	cr.clustersLock.RLock()
	defer cr.clustersLock.RUnlock()

	clusters := make([]*v1alpha1.Cluster, 0)
	for _, cluster := range cr.clustersDetails {
		if cluster.GetType() == v1alpha1.ClusterType_SPEECH_GENERATION {
			clusters = append(clusters, cluster)
		}
	}

	return clusters
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./pkg/clusters/manager/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add pkg/clusters/manager/cluster.go
git commit -m "feat: add ListTTSClusters to cluster manager"
```

---

### Task 2: Create voice types and per-provider listing logic

**Files:**
- Create: `pkg/listener/manager/tts/voices.go`

- [ ] **Step 1: Create voices.go with types and provider logic**

```go
package tts

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/samber/lo"

	v1alpha1clusters "knoway.dev/api/clusters/v1alpha1"
	clustermanager "knoway.dev/pkg/clusters/manager"
	"knoway.dev/pkg/filters/auth"
	"knoway.dev/pkg/metadata"
	"knoway.dev/pkg/types/openai"
)

type Voice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Locale   string `json:"locale,omitempty"`
	Gender   string `json:"gender,omitempty"`
}

type VoicesResponse struct {
	Voices []Voice `json:"voices"`
}

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

// microsoftVoice represents a voice entry from Microsoft's voices/list API.
type microsoftVoice struct {
	ShortName   string `json:"ShortName"`
	DisplayName string `json:"DisplayName"`
	LocalName   string `json:"LocalName"`
	Gender      string `json:"Gender"`
	Locale      string `json:"Locale"`
	VoiceType   string `json:"VoiceType"`
	Status      string `json:"Status"`
}

func listOpenAIVoices(clusterName string) []Voice {
	provider := v1alpha1clusters.ClusterProvider_OPEN_AI.String()

	return lo.Map(openAIVoices, func(v Voice, _ int) Voice {
		v.Provider = provider
		return v
	})
}

func listMicrosoftVoices(cluster *v1alpha1clusters.Cluster) ([]Voice, error) {
	upstream := cluster.GetUpstream()

	// Derive region from upstream URL or default to eastasia
	region := "eastasia"
	if url := upstream.GetUrl(); url != "" {
		// URL format: https://{region}.tts.speech.microsoft.com/cognitiveservices/v1
		if parts := strings.SplitN(url, ".", 2); len(parts) > 0 {
			r := strings.TrimPrefix(parts[0], "https://")
			r = strings.TrimPrefix(r, "http://")
			if r != "" {
				region = r
			}
		}
	}

	voicesURL := fmt.Sprintf("https://%s.tts.speech.microsoft.com/cognitiveservices/voices/list", region)

	req, err := http.NewRequest(http.MethodGet, voicesURL, nil)
	if err != nil {
		return nil, err
	}

	// Get subscription key from upstream headers
	for _, h := range upstream.GetHeaders() {
		if strings.EqualFold(h.GetKey(), "Ocp-Apim-Subscription-Key") {
			req.Header.Set("Ocp-Apim-Subscription-Key", h.GetValue())

			break
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Microsoft voices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("Microsoft voices API returned %d: %s", resp.StatusCode, string(body))
	}

	var msVoices []microsoftVoice
	if err := json.NewDecoder(resp.Body).Decode(&msVoices); err != nil {
		return nil, fmt.Errorf("failed to decode Microsoft voices: %w", err)
	}

	provider := v1alpha1clusters.ClusterProvider_MICROSOFT_SPEECH_SERVICE_V1.String()
	voices := make([]Voice, 0, len(msVoices))

	for _, v := range msVoices {
		voices = append(voices, Voice{
			ID:       v.ShortName,
			Name:     v.DisplayName,
			Provider: provider,
			Locale:   v.Locale,
			Gender:   v.Gender,
		})
	}

	return voices, nil
}

func listVoicesForCluster(cluster *v1alpha1clusters.Cluster) ([]Voice, error) {
	switch cluster.GetProvider() {
	case v1alpha1clusters.ClusterProvider_OPEN_AI_V1_SPEECH, v1alpha1clusters.ClusterProvider_OPEN_AI:
		return listOpenAIVoices(cluster.GetName()), nil
	case v1alpha1clusters.ClusterProvider_MICROSOFT_SPEECH_SERVICE_V1:
		return listMicrosoftVoices(cluster)
	default:
		// Unsupported providers return empty list
		return nil, nil
	}
}

func (l *OpenAITextToSpeechListener) listVoices(writer http.ResponseWriter, request *http.Request) (any, error) {
	for _, f := range l.filters.OnRequestPreFilters() {
		fResult := f.OnRequestPre(request.Context(), request)
		if fResult.IsFailed() {
			return nil, fResult.Error
		}
	}

	clusters := clustermanager.ListTTSClusters()

	// Apply auth filters
	rMeta := metadata.RequestMetadataFromCtx(request.Context())
	if rMeta.EnabledAuthFilter {
		if rMeta.AuthInfo != nil {
			clusters = lo.Filter(clusters, func(item *v1alpha1clusters.Cluster, index int) bool {
				return auth.CanAccessModel(item.GetName(), rMeta.AuthInfo.GetAllowModels(), rMeta.AuthInfo.GetDenyModels())
			})
		}
	}

	allVoices := make([]Voice, 0)

	for _, cluster := range clusters {
		voices, err := listVoicesForCluster(cluster)
		if err != nil {
			// Log error but continue with other clusters
			slog.ErrorContext(request.Context(), "failed to list voices for cluster",
				"cluster", cluster.GetName(),
				"provider", cluster.GetProvider().String(),
				"error", err,
			)

			continue
		}

		allVoices = append(allVoices, voices...)
	}

	return VoicesResponse{Voices: allVoices}, nil
}
```

Note: add `"log/slog"` to the imports.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./pkg/listener/manager/tts/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add pkg/listener/manager/tts/voices.go
git commit -m "feat: add voice listing logic for TTS providers"
```

---

### Task 3: Register the `/v1/audio/voices` route

**Files:**
- Modify: `pkg/listener/manager/tts/listener.go:65-79`

- [ ] **Step 1: Add the route in RegisterRoutes**

In `pkg/listener/manager/tts/listener.go`, add the voices route after the speech route:

```go
func (l *OpenAITextToSpeechListener) RegisterRoutes(mux *mux.Router) error {
	middlewares := listener.WithMiddlewares(
		listener.WithCancellable(l.cancellable),
		listener.WithInitMetadata(),
		listener.WithAccessLog(l.cfg.GetAccessLog().GetEnable()),
		listener.WithRequestTimer(),
		listener.WithOptions(),
		listener.WithResponseHandler(openai.ResponseHandler()),
		listener.WithRecoverWithError(),
		listener.WithRejectAfterDrainedWithError(l),
	)

	mux.HandleFunc("/v1/audio/speech", listener.HTTPHandlerFunc(middlewares(listener.CommonListenerHandler(l.filters, l.reversedFilters, l.unmarshalTextToSpeechRequestToLLMRequest))))
	mux.HandleFunc("/v1/audio/voices", listener.HTTPHandlerFunc(middlewares(l.listVoices)))

	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add pkg/listener/manager/tts/listener.go
git commit -m "feat: register /v1/audio/voices route"
```

---

### Task 4: Write unit tests

**Files:**
- Create: `pkg/listener/manager/tts/voices_test.go`

- [ ] **Step 1: Write tests for per-provider voice listing**

```go
package tts

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1alpha1clusters "knoway.dev/api/clusters/v1alpha1"
)

func TestListOpenAIVoices(t *testing.T) {
	voices := listOpenAIVoices("test-cluster")

	assert.Len(t, voices, 9)
	assert.Equal(t, "alloy", voices[0].ID)
	assert.Equal(t, "Alloy", voices[0].Name)
	assert.Equal(t, v1alpha1clusters.ClusterProvider_OPEN_AI.String(), voices[0].Provider)
}

func TestListVoicesForCluster_OpenAI(t *testing.T) {
	cluster := &v1alpha1clusters.Cluster{
		Name:     "tts-openai",
		Provider: v1alpha1clusters.ClusterProvider_OPEN_AI_V1_SPEECH,
	}

	voices, err := listVoicesForCluster(cluster)
	require.NoError(t, err)
	assert.Len(t, voices, 9)
}

func TestListVoicesForCluster_UnsupportedProvider(t *testing.T) {
	cluster := &v1alpha1clusters.Cluster{
		Name:     "tts-deepgram",
		Provider: v1alpha1clusters.ClusterProvider_DEEPGRAM_WEBSOCKET_V1,
	}

	voices, err := listVoicesForCluster(cluster)
	require.NoError(t, err)
	assert.Empty(t, voices)
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./pkg/listener/manager/tts/ -v -run TestList`
Expected: All PASS

- [ ] **Step 3: Commit**

```bash
git add pkg/listener/manager/tts/voices_test.go
git commit -m "test: add unit tests for voice listing"
```

---

### Task 5: Integration verification

- [ ] **Step 1: Run all tests to make sure nothing is broken**

Run: `go build ./... && go test ./pkg/listener/manager/tts/ ./pkg/clusters/manager/ -v`
Expected: All PASS

- [ ] **Step 2: Run linter if configured**

Run: `go vet ./...`
Expected: No errors
