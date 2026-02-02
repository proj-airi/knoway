package tts

import "bytes"

type Request interface {
	GetModel() string
	GetInput() string
	GetVoice() string
	GetResponseFormat() *string
	GetSpeed() *float64
	GetExtraBody() map[string]any
	GetBodyParsed() map[string]any
	GetBodyBuffer() *bytes.Buffer
}
