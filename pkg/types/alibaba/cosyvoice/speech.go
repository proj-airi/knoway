package cosyvoice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/samber/lo"

	"knoway.dev/pkg/object"
	"knoway.dev/pkg/types/openai"
	"knoway.dev/pkg/types/tts"
	"knoway.dev/pkg/utils"
)

const (
	defaultAlibabaSpeechWSURL = "wss://dashscope.aliyuncs.com/api-ws/v1/inference"
)

var ErrWebSocketOnlyProvider = errors.New("provider requires websocket execution")

type serverEventEvent string

const (
	serverEventTaskStarted     serverEventEvent = "task-started"
	serverEventResultGenerated serverEventEvent = "result-generated"
	serverEventTaskFinished    serverEventEvent = "task-finished"
	serverEventTaskFailed      serverEventEvent = "task-failed"
)

type clientEventAction string

const (
	clientEventContinueTask clientEventAction = "continue-task"
	clientEventFinishTask   clientEventAction = "finish-task"
	clientEventRunTask      clientEventAction = "run-task"
)

type serverEventHeader struct {
	TaskID     string           `json:"task_id"`
	Event      serverEventEvent `json:"event"`
	Attributes map[string]any   `json:"attributes"`

	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

type clientEventHeaderStreaming string

const (
	clientEventHeaderStreamingDuplex clientEventHeaderStreaming = "duplex"
)

type clientEventHeader struct {
	TaskID    string                     `json:"task_id"`
	Action    clientEventAction          `json:"action"`
	Streaming clientEventHeaderStreaming `json:"streaming"`
}

type event struct {
	Header  serverEventHeader `json:"header"`
	Payload json.RawMessage   `json:"payload"`
}

type clientEvent[E any] struct {
	Header  clientEventHeader `json:"header"`
	Payload E                 `json:"payload"`
}

type clientEventPayloadTaskGroup string

const (
	clientEventPayloadTaskGroupAudio clientEventPayloadTaskGroup = "audio"
)

type clientEventPayloadTask string

const (
	clientEventPayloadTaskTTS clientEventPayloadTask = "tts"
)

type clientEventPayloadFunction string

const (
	clientEventPayloadFunctionSpeechSynthesizer clientEventPayloadFunction = "SpeechSynthesizer"
)

type clientEventRunTaskPayloadParametersTextType string

const (
	clientEventRunTaskPayloadParametersTextTypePlainText clientEventRunTaskPayloadParametersTextType = "PlainText"
)

type clientEventRunTaskPayloadParameters struct {
	TextType   clientEventRunTaskPayloadParametersTextType `json:"text_type"`
	Voice      string                                      `json:"voice"`
	Format     string                                      `json:"format"`
	SampleRate int                                         `json:"sample_rate"`
	Volume     int                                         `json:"volume"`
	Rate       float64                                     `json:"rate"`
	Pitch      float64                                     `json:"pitch"`
}

type clientEventRunTaskPayload struct {
	TaskGroup  clientEventPayloadTaskGroup         `json:"task_group"`
	Task       clientEventPayloadTask              `json:"task"`
	Function   clientEventPayloadFunction          `json:"function"`
	Model      string                              `json:"model"`
	Input      map[string]any                      `json:"input"`
	Parameters clientEventRunTaskPayloadParameters `json:"parameters"`
}

type clientEventContinueTaskPayloadInput struct {
	Text string `json:"text"`
}

type clientEventContinueTaskPayload struct {
	TaskGroup clientEventPayloadTaskGroup         `json:"task_group"`
	Task      clientEventPayloadTask              `json:"task"`
	Function  clientEventPayloadFunction          `json:"function"`
	Input     clientEventContinueTaskPayloadInput `json:"input"`
}

type clientEventFinishTaskPayload struct {
	Input map[string]any `json:"input"`
}

func BuildSpeechRequest(_ context.Context, _ string, _ string, _ tts.Request, _ http.Header, _ http.Header) (*http.Request, error) {
	return nil, ErrWebSocketOnlyProvider
}

func DoSpeech(ctx context.Context, authHeader string, req tts.Request) (object.LLMResponse, error) {
	taskID := uuid.New().String()
	headers := http.Header{}

	headers.Add("Authorization", strings.TrimPrefix(authHeader, "Bearer "))
	headers.Add("X-Dashscope-Datainspection", "enable")

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, defaultAlibabaSpeechWSURL, headers)
	if err != nil {
		if resp == nil {
			return nil, openai.NewErrorBadGateway().WithMessage(err.Error())
		}

		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		return nil, tts.ParseUpstreamError(resp, body)
	}

	defer func() {
		_ = resp.Body.Close()
		_ = conn.Close()
	}()

	audioBinary := new(bytes.Buffer)
	chanResult := make(chan struct{}, 1)
	chanError := make(chan error, 1)

	go func() {
		defer close(chanResult)
		defer close(chanError)

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				chanError <- openai.NewErrorInternalError().WithCause(err)
				return
			}

			switch messageType {
			case websocket.BinaryMessage:
				if _, err := audioBinary.Write(message); err != nil {
					chanError <- openai.NewErrorInternalError().WithCause(err)
					return
				}
			default:
				var ev event
				err := json.Unmarshal(message, &ev)
				if err != nil {
					chanError <- openai.NewErrorInternalError().WithCause(err)
					return
				}

				switch ev.Header.Event {
				case serverEventTaskStarted:
					err = conn.WriteJSON(clientEvent[clientEventContinueTaskPayload]{
						Header: clientEventHeader{
							TaskID:    taskID,
							Action:    clientEventContinueTask,
							Streaming: clientEventHeaderStreamingDuplex,
						},
						Payload: clientEventContinueTaskPayload{
							TaskGroup: clientEventPayloadTaskGroupAudio,
							Task:      clientEventPayloadTaskTTS,
							Function:  clientEventPayloadFunctionSpeechSynthesizer,
							Input: clientEventContinueTaskPayloadInput{
								Text: req.GetInput(),
							},
						},
					})
					if err != nil {
						chanError <- openai.NewErrorInternalError().WithCause(err)
						return
					}

					err = conn.WriteJSON(clientEvent[clientEventFinishTaskPayload]{
						Header: clientEventHeader{
							TaskID:    taskID,
							Action:    clientEventFinishTask,
							Streaming: clientEventHeaderStreamingDuplex,
						},
						Payload: clientEventFinishTaskPayload{
							Input: make(map[string]any),
						},
					})
					if err != nil {
						chanError <- openai.NewErrorInternalError().WithCause(err)
						return
					}
				case serverEventTaskFailed:
					chanError <- openai.NewErrorBadRequest().WithMessage(
						"failed to run task, error_code: " + ev.Header.ErrorCode + ", error_message: " + ev.Header.ErrorMessage,
					)
				case serverEventResultGenerated:
					continue
				case serverEventTaskFinished:
					chanResult <- struct{}{}
				}
			}
		}
	}()

	volume := utils.GetByJSONPath[*int](req.GetExtraBody(), "{ .volume }")
	if volume == nil {
		volume = lo.ToPtr(50)
	}

	rate := utils.GetByJSONPath[*float64](req.GetExtraBody(), "{ .rate }")
	if rate == nil {
		rate = lo.ToPtr(float64(1))
	}

	pitch := utils.GetByJSONPath[*float64](req.GetExtraBody(), "{ .pitch }")
	if pitch == nil {
		pitch = lo.ToPtr(float64(1))
	}

	sampleRate := utils.GetByJSONPath[*int](req.GetExtraBody(), "{ .sample_rate }")
	if sampleRate == nil {
		sampleRate = lo.ToPtr(22050)
	}

	err = conn.WriteJSON(clientEvent[clientEventRunTaskPayload]{
		Header: clientEventHeader{
			TaskID:    taskID,
			Action:    clientEventRunTask,
			Streaming: clientEventHeaderStreamingDuplex,
		},
		Payload: clientEventRunTaskPayload{
			TaskGroup: clientEventPayloadTaskGroupAudio,
			Task:      clientEventPayloadTaskTTS,
			Function:  clientEventPayloadFunctionSpeechSynthesizer,
			Model:     req.GetModel(),
			Input:     make(map[string]any),
			Parameters: clientEventRunTaskPayloadParameters{
				TextType:   clientEventRunTaskPayloadParametersTextTypePlainText,
				Voice:      req.GetVoice(),
				Format:     lo.Ternary(req.GetResponseFormat() == nil || *req.GetResponseFormat() == "", "mp3", *req.GetResponseFormat()),
				SampleRate: lo.FromPtr(sampleRate),
				Volume:     lo.FromPtr(volume),
				Rate:       lo.FromPtr(rate),
				Pitch:      lo.FromPtr(pitch),
			},
		},
	})
	if err != nil {
		return nil, openai.NewErrorInternalError().WithCause(err)
	}

	select {
	case err := <-chanError:
		return nil, err
	case <-chanResult:
		return tts.NewAudioResponseFromBytes(http.StatusOK, "audio/mp3", req.GetModel(), audioBinary.Bytes()), nil
	}
}
