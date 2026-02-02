/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Provider",type=string,JSONPath=`.spec.provider`
//+kubebuilder:printcolumn:name="Model Name",type=string,JSONPath=`.spec.modelName`
//+kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.spec.upstream.baseUrl`
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`

// TextToSpeechBackend is the Schema for the texttospeechbackends API.
type TextToSpeechBackend struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TextToSpeechBackendSpec   `json:"spec,omitempty"`
	Status TextToSpeechBackendStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TextToSpeechBackendList contains a list of TextToSpeechBackend.
type TextToSpeechBackendList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TextToSpeechBackend `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TextToSpeechBackend{}, &TextToSpeechBackendList{})
}

// TextToSpeechBackendSpec defines the desired state of TextToSpeechBackend.
type TextToSpeechBackendSpec struct {
	// ModelName specifies the name of the model
	// +kubebuilder:validation:Optional
	// +optional
	ModelName *string `json:"modelName,omitempty"`
	// Provider indicates the organization providing the model
	// +kubebuilder:validation:Enum=OpenAI;vLLM;Ollama;OpenAIV1Speech;DeepgramWebSocketV1;ElevenLabsV1;KoemotionV1;VolcengineSeedSpeechServiceV1;AlibabaCosyVoiceService;MicrosoftSpeechServiceV1
	Provider Provider `json:"provider,omitempty"`
	// Upstream contains information about the upstream configuration
	Upstream TextToSpeechBackendUpstream `json:"upstream,omitempty"`
	// Filters are applied to the model's requests
	Filters []TextToSpeechFilter `json:"filters,omitempty"`
}

// TextToSpeechBackendUpstream defines the upstream server configuration.
type TextToSpeechBackendUpstream struct {
	// BaseUrl define upstream endpoint url
	// Example:
	// 		https://api.openai.com/v1/audio/speech
	//
	//  	http://tts.default.svc.cluster.local:8080/v1/audio/speech
	BaseURL string `json:"baseUrl,omitempty"`

	// Headers defines the common headers for the model, such as the authentication header for the API key.
	// Example:
	//
	// headers：
	// 	- key: apikey
	// 	  value: "sk-or-v1-xxxxxxxxxx"
	Headers []Header `json:"headers,omitempty"`
	// Headers defines the common headers for the model, such as the authentication header for the API key.
	// Example:
	//
	// headersFrom：
	// 	- prefix: sk-or-v1-
	//	  refType: Secret
	//	  refName: common-gpt4-apikey
	HeadersFrom []HeaderFromSource `json:"headersFrom,omitempty"`

	DefaultParams   *TextToSpeechModelParams `json:"defaultParams,omitempty"`
	OverrideParams  *TextToSpeechModelParams `json:"overrideParams,omitempty"`
	RemoveParamKeys []string                 `json:"RemoveParamKeys,omitempty"`

	Timeout int32 `json:"timeout,omitempty"`
}

type TextToSpeechModelParams struct {
	// OpenAI model parameters
	OpenAI *OpenAITextToSpeechParam `json:"openai,omitempty"`
}

type TextToSpeechCommonParams struct {
	Model string `json:"model,omitempty"`

	// The text to generate audio for.
	Input *string `json:"input,omitempty"`
	// The voice to use when generating the audio.
	Voice *string `json:"voice,omitempty"`
}

type OpenAITextToSpeechResponseFormat string

const (
	OpenAITextToSpeechResponseFormatMP3  OpenAITextToSpeechResponseFormat = "mp3"
	OpenAITextToSpeechResponseFormatOpus OpenAITextToSpeechResponseFormat = "opus"
	OpenAITextToSpeechResponseFormatAAC  OpenAITextToSpeechResponseFormat = "aac"
	OpenAITextToSpeechResponseFormatFLAC OpenAITextToSpeechResponseFormat = "flac"
	OpenAITextToSpeechResponseFormatWAV  OpenAITextToSpeechResponseFormat = "wav"
	OpenAITextToSpeechResponseFormatPCM  OpenAITextToSpeechResponseFormat = "pcm"
)

type OpenAITextToSpeechParam struct {
	TextToSpeechCommonParams `json:",inline"`

	// The format to audio in. Supported formats are mp3, opus, aac, flac, wav, and pcm.
	ResponseFormat *OpenAITextToSpeechResponseFormat `json:"response_format,omitempty"`
	// The speed of the generated audio. Select a value from 0.25 to 4.0.
	Speed *string `json:"speed,omitempty" floatString:"true"`
}

// TextToSpeechFilter represents the text-to-speech backend filter configuration.
type TextToSpeechFilter struct {
	Name string `json:"name,omitempty"` // Filter name

	TextToSpeechFilterConfig `json:",inline"`
}

// TextToSpeechFilterConfig represents the configuration for filters.
// At least one of the following must be specified: CustomConfig
// +kubebuilder:validation:Required
type TextToSpeechFilterConfig struct {
	// Custom: Custom plugin configuration
	// Example:
	//
	// 	custom:
	// 		pluginName: examplePlugin
	// 		pluginVersion: "1.0.0"
	// 		settings:
	//   		setting1: value1
	//   		setting2: value2
	//
	// +kubebuilder:validation:OneOf
	// +optional
	Custom *runtime.RawExtension `json:"custom,omitempty"`
}

// TextToSpeechBackendStatus defines the observed state of TextToSpeechBackend.
type TextToSpeechBackendStatus struct {
	// Status indicates the health of the backend: Unknown, Healthy, or Failed
	// +kubebuilder:validation:Enum=Unknown;Healthy;Failed
	Status StatusEnum `json:"status,omitempty"`

	// Conditions represent the current conditions of the backend
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Endpoints holds the upstream addresses of the current model (pod IP addresses)
	Endpoints []string `json:"endpoints,omitempty"`
}
