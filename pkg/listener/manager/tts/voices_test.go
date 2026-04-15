package tts

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1alpha1 "knoway.dev/api/clusters/v1alpha1"
)

func TestListVoicesForCluster_OpenAI(t *testing.T) {
	cluster := &v1alpha1.Cluster{
		Name:     "test-openai-tts",
		Provider: v1alpha1.ClusterProvider_OPEN_AI_V1_SPEECH,
	}

	req := httptest.NewRequest("GET", "/", nil)
	voices, err := listVoicesForCluster(req, cluster)
	require.NoError(t, err)
	assert.Len(t, voices, 9)

	expectedIDs := []string{"alloy", "ash", "coral", "echo", "fable", "nova", "onyx", "sage", "shimmer"}
	gotIDs := make([]string, len(voices))

	for i, v := range voices {
		gotIDs[i] = v.ID
		assert.NotEmpty(t, v.PreviewAudioURL, "preview URL should be populated")
		assert.NotEmpty(t, v.Languages, "languages should be populated")
	}

	assert.ElementsMatch(t, expectedIDs, gotIDs)
}

func TestListVoicesForCluster_Unimplemented(t *testing.T) {
	// Koemotion returns nil voices because unspeech hasn't implemented listing yet;
	// knoway silently skips such clusters rather than surfacing the error.
	cluster := &v1alpha1.Cluster{
		Name:     "test-koemotion",
		Provider: v1alpha1.ClusterProvider_KOEMOTION_V1,
	}

	req := httptest.NewRequest("GET", "/", nil)
	voices, err := listVoicesForCluster(req, cluster)
	// Default branch returns (nil, nil) for unknown/unmapped providers.
	require.NoError(t, err)
	assert.Empty(t, voices)
}

func TestMicrosoftRegion(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantRegion string
		wantErr    bool
	}{
		{
			name:       "eastasia region",
			url:        "https://eastasia.tts.speech.microsoft.com/cognitiveservices/v1",
			wantRegion: "eastasia",
		},
		{
			name:       "westus2 region",
			url:        "https://westus2.tts.speech.microsoft.com/cognitiveservices/v1",
			wantRegion: "westus2",
		},
		{
			name:    "invalid URL",
			url:     "://invalid-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			region, err := microsoftRegion(tt.url)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantRegion, region)
		})
	}
}
