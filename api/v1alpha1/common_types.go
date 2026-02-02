package v1alpha1

type Header struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// HeaderFromSource represents the source of a set of ConfigMaps or Secrets
type HeaderFromSource struct {
	// An optional identifier to prepend to each key in the ref.
	Prefix string `json:"prefix,omitempty"`
	// Type of the source (ConfigMap or Secret)
	RefType ValueFromType `json:"refType,omitempty"`
	// Name of the source
	RefName string `json:"refName,omitempty"`
}

// ValueFromType defines the type of source for headers.
// +kubebuilder:validation:Enum=ConfigMap;Secret
type ValueFromType string

const (
	// ConfigMap indicates that the header source is a ConfigMap.
	ConfigMap ValueFromType = "ConfigMap"
	// Secret indicates that the header source is a Secret.
	Secret ValueFromType = "Secret"
)

// StatusEnum defines the possible statuses for the LLMBackend, ImageGenerationBackend, and other types.
type StatusEnum string

const (
	Unknown StatusEnum = "Unknown"
	Healthy StatusEnum = "Healthy"
	Failed  StatusEnum = "Failed"
)

type Provider string

const (
	ProviderOpenAI Provider = "OpenAI"
	ProviderVLLM   Provider = "vLLM"
	ProviderOllama Provider = "Ollama"

	ProviderOpenAIV1Speech           Provider = "OpenAIV1Speech"
	ProviderDeepgramWebSocketV1      Provider = "DeepgramWebSocketV1"
	ProviderElevenLabsV1             Provider = "ElevenLabsV1"
	ProviderKoemotionV1              Provider = "KoemotionV1"
	ProviderVolcengineSeedSpeechV1   Provider = "VolcengineSeedSpeechServiceV1"
	ProviderAlibabaCosyVoiceService  Provider = "AlibabaCosyVoiceService"
	ProviderMicrosoftSpeechServiceV1 Provider = "MicrosoftSpeechServiceV1"
)

type BackendType string

const (
	BackendTypeLLM             BackendType = "LLM"
	BackendTypeImageGeneration BackendType = "ImageGeneration"
)
