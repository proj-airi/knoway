package tts

import (
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

	voices, err := listVoicesForCluster(cluster)
	require.NoError(t, err)
	assert.Len(t, voices, 9)

	expectedIDs := []string{"alloy", "ash", "coral", "echo", "fable", "nova", "onyx", "sage", "shimmer"}
	gotIDs := make([]string, len(voices))

	for i, v := range voices {
		gotIDs[i] = v.ID
		assert.Equal(t, v1alpha1.ClusterProvider_OPEN_AI_V1_SPEECH.String(), v.Provider)
	}

	assert.ElementsMatch(t, expectedIDs, gotIDs)
}

func TestListVoicesForCluster_UnsupportedProvider(t *testing.T) {
	cluster := &v1alpha1.Cluster{
		Name:     "test-deepgram",
		Provider: v1alpha1.ClusterProvider_DEEPGRAM_WEBSOCKET_V1,
	}

	voices, err := listVoicesForCluster(cluster)
	require.NoError(t, err)
	assert.Empty(t, voices)
}

func TestExtractRegionFromURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantRegion  string
		wantErr     bool
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
			name:    "invalid URL with no dot in hostname",
			url:     "://invalid-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			region, err := extractRegionFromURL(tt.url)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantRegion, region)
		})
	}
}
