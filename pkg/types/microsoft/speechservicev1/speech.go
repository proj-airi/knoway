package speechservicev1

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/samber/lo"
	"github.com/samber/mo"

	"knoway.dev/pkg/object"
	"knoway.dev/pkg/types/openai"
	"knoway.dev/pkg/types/tts"
)

const (
	defaultMicrosoftRegion = "eastasia"
)

type voice struct {
	XMLName xml.Name `xml:"voice"`
	Lang    string   `xml:"lang,attr"`
	Gender  string   `xml:"gender,attr"`
	Name    string   `xml:"name,attr"`
	Text    string   `xml:",chardata"`
}

type ssml struct {
	XMLName  xml.Name `xml:"speak"`
	Version  string   `xml:"version,attr"`
	Lang     string   `xml:"lang,attr"`
	Voice    voice    `xml:"voice"`
	TextOnly string   `xml:",chardata"`
}

type extraBody struct {
	DisableSSML  mo.Option[bool]   `json:"disable_ssml,omitempty"`
	Region       string            `json:"region"`
	DeploymentID mo.Option[string] `json:"deployment_id,omitempty"`
	Lang         mo.Option[string] `json:"lang,omitempty"`
	Gender       mo.Option[string] `json:"gender,omitempty"`
	SampleRate   mo.Option[uint]   `json:"sample_rate,omitempty"`
}

var (
	supportedOutputFormats = map[string]map[uint][]string{
		"mp3": {
			16000: {"audio-16khz-32kbitrate-mono-mp3", "audio-16khz-64kbitrate-mono-mp3", "audio-16khz-128kbitrate-mono-mp3"},
			24000: {"audio-24khz-48kbitrate-mono-mp3", "audio-24khz-96kbitrate-mono-mp3", "audio-24khz-160kbitrate-mono-mp3"},
			48000: {"audio-48khz-96kbitrate-mono-mp3", "audio-48khz-192kbitrate-mono-mp3"},
		},
		"opus": {
			16000: {"audio-16khz-16bit-32kbps-mono-opus", "ogg-16khz-16bit-mono-opus", "webm-16khz-16bit-mono-opus"},
			24000: {"audio-24khz-16bit-24kbps-mono-opus", "audio-24khz-16bit-48kbps-mono-opus", "ogg-24khz-16bit-mono-opus", "webm-24khz-16bit-24kbps-mono-opus", "webm-24khz-16bit-mono-opus"},
			48000: {"ogg-48khz-16bit-mono-opus"},
		},
		"wav": {
			8000:  {"raw-8khz-16bit-mono-pcm", "raw-8khz-8bit-mono-alaw", "raw-8khz-8bit-mono-mulaw"},
			16000: {"raw-16khz-16bit-mono-pcm", "raw-16khz-16bit-mono-truesilk"},
			22050: {"raw-22050hz-16bit-mono-pcm"},
			24000: {"raw-24khz-16bit-mono-pcm", "raw-24khz-16bit-mono-truesilk"},
			44100: {"raw-44100hz-16bit-mono-pcm"},
			48000: {"raw-48khz-16bit-mono-pcm"},
		},
	}
)

func getOutputFormat(format string, sampleRate uint) mo.Option[string] {
	formatsWithSampleRate, ok := supportedOutputFormats[format]
	if !ok {
		return mo.None[string]()
	}

	formatFull, ok := formatsWithSampleRate[sampleRate]
	if !ok {
		return mo.None[string]()
	}

	return mo.Some(formatFull[0])
}

func formatAsSSML(text string, lang string, gender string, voiceName string) string {
	return fmt.Sprintf(`<speak version='1.0' xml:lang='%s'>
  <voice xml:lang='%s' xml:gender='%s' name='%s'>
    %s
  </voice>
</speak>`, lang, lang, gender, voiceName, text)
}

func processSSML(input string, option tts.Request, extra mo.Option[extraBody]) string {
	if extra.OrEmpty().DisableSSML.OrEmpty() {
		return input
	}

	defaultLang := "en-US"
	defaultGender := "Male"
	defaultVoiceName := lo.CoalesceOrEmpty(option.GetVoice(), "en-US-ChristopherNeural")

	if extra.OrEmpty().Lang.IsPresent() {
		defaultLang = extra.MustGet().Lang.MustGet()
	}

	if extra.OrEmpty().Gender.IsPresent() {
		defaultGender = extra.MustGet().Gender.MustGet()
	}

	if !strings.Contains(input, "<speak") {
		return formatAsSSML(input, defaultLang, defaultGender, defaultVoiceName)
	}

	var s ssml
	err := xml.Unmarshal([]byte(input), &s)
	if err != nil {
		return formatAsSSML(input, defaultLang, defaultGender, defaultVoiceName)
	}

	lang := defaultLang
	if s.Lang != "" {
		lang = s.Lang
	}

	gender := defaultGender
	if s.Voice.Gender != "" {
		gender = s.Voice.Gender
	}

	voiceName := defaultVoiceName
	if s.Voice.Name != "" {
		voiceName = s.Voice.Name
	}

	text := s.Voice.Text
	if strings.TrimSpace(text) == "" {
		text = strings.TrimSpace(s.TextOnly)
	}

	if strings.TrimSpace(text) != "" {
		return formatAsSSML(text, lang, gender, voiceName)
	}

	return input
}

func BuildSpeechRequest(ctx context.Context, baseURL string, authHeader string, req tts.Request, upstreamHeaders http.Header, downstreamHeaders http.Header) (*http.Request, error) {
	var extra mo.Option[extraBody]

	if req.GetExtraBody() != nil {
		extraBodyJSON, err := json.Marshal(req.GetExtraBody())
		if err != nil {
			return nil, err
		}

		var body extraBody
		if err := json.Unmarshal(extraBodyJSON, &body); err != nil {
			return nil, err
		}

		extra = mo.Some(body)
	} else {
		extra = mo.None[extraBody]()
	}

	region := defaultMicrosoftRegion
	if extra.IsPresent() && extra.OrEmpty().Region != "" {
		region = extra.MustGet().Region
	}

	reqURL := baseURL
	if reqURL == "" {
		reqURL = fmt.Sprintf("https://%s.tts.speech.microsoft.com/cognitiveservices/v1", region)
	}

	reqSearchParams := url.Values{}
	if extra.IsPresent() && extra.OrEmpty().DeploymentID.IsPresent() {
		reqSearchParams.Add("deploymentId", extra.MustGet().DeploymentID.MustGet())
	}

	formattedText := processSSML(req.GetInput(), req, extra)

	targetURL := reqURL
	if len(reqSearchParams) > 0 {
		targetURL = targetURL + "?" + reqSearchParams.Encode()
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewBufferString(formattedText))
	if err != nil {
		return nil, err
	}

	format := ""
	if downstreamHeaders != nil {
		format = downstreamHeaders.Get("X-Microsoft-Outputformat")
	}
	if format == "" && upstreamHeaders != nil {
		format = upstreamHeaders.Get("X-Microsoft-Outputformat")
	}
	if format == "" {
		if req.GetResponseFormat() == nil || *req.GetResponseFormat() == "" {
			format = "audio-48khz-192kbitrate-mono-mp3"
		} else {
			format = getOutputFormat(*req.GetResponseFormat(), extra.OrEmpty().SampleRate.OrElse(48000)).OrEmpty()
			if format == "" {
				return nil, openai.NewErrorBadRequest().WithMessage("unsupported output format for microsoft speech service")
			}
		}
	}

	subscriptionKey := ""
	if downstreamHeaders != nil {
		subscriptionKey = downstreamHeaders.Get("Ocp-Apim-Subscription-Key")
	}
	if subscriptionKey == "" && upstreamHeaders != nil {
		subscriptionKey = upstreamHeaders.Get("Ocp-Apim-Subscription-Key")
	}
	if subscriptionKey == "" {
		subscriptionKey = strings.TrimPrefix(authHeader, "Bearer ")
	}
	if subscriptionKey != "" {
		httpReq.Header.Set("Ocp-Apim-Subscription-Key", subscriptionKey)
	}
	httpReq.Header.Set("Content-Type", "application/ssml+xml")
	httpReq.Header.Set("X-Microsoft-Outputformat", format)

	return httpReq, nil
}

func ParseSpeechResponse(resp *http.Response, model string) (object.LLMResponse, error) {
	if resp == nil {
		return nil, openai.NewErrorBadGateway().WithMessage("upstream response is nil")
	}

	if resp.StatusCode >= http.StatusBadRequest {
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, openai.NewErrorBadGateway().WithMessage("upstream error: " + resp.Status)
		}

		return nil, tts.ParseUpstreamError(resp, body)
	}

	return tts.NewAudioResponseFromHTTP(resp, model), nil
}
