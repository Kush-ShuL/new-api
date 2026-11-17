package relay

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/MoeForge/moego/ojson"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/wsmanager"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func (s *responsesWSSession) warmHTTPBridge(c *gin.Context, state *responsesWSCallState, create responsesWSCreateRequest) *types.NewAPIError {
	id := "resp_" + common.NewRequestId()
	response := map[string]any{
		"id": id, "object": "response", "created_at": time.Now().Unix(), "status": "completed",
		"model": create.Request.Model, "output": []any{},
		"usage": map[string]any{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0},
	}
	body, err := common.Marshal(response)
	if err != nil {
		return newResponsesWSInvalidRequestError(err)
	}
	if err := service.SaveResponsesContext(c, id, &create.Request, body); err != nil {
		return newResponsesWSInvalidRequestError(err)
	}
	s.lastResponseID = id
	for index, kind := range []string{"response.created", "response.completed"} {
		response["status"] = "in_progress"
		if kind == "response.completed" {
			response["status"] = "completed"
		}
		event, err := common.Marshal(map[string]any{"type": kind, "sequence_number": index, "stream_id": create.StreamID, "response": response})
		if err != nil {
			return newResponsesWSInvalidRequestError(err)
		}
		if kind == "response.completed" {
			state.terminal = &responsesWSMessage{kind: websocket.TextMessage, body: event}
		} else if err := s.writeClient(websocket.TextMessage, event); err != nil {
			return types.NewError(err, types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
		}
	}
	return nil
}

func (s *responsesWSSession) runHTTPBridge(c *gin.Context, state *responsesWSCallState, create responsesWSCreateRequest, info *relaycommon.RelayInfo) *types.NewAPIError {
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	unregister := wsmanager.Register(common.GetContextKeyInt(c, constant.ContextKeyChannelId), wsmanager.KindResponses, func(reason string) { s.closeForPolicy(reason) })
	defer unregister()
	var responseID atomic.Value
	responseID.Store("")
	var cancelled atomic.Bool
	var interrupted atomic.Bool
	lastResponseID := s.lastResponseID
	controlsDone := make(chan struct{})
	go func() {
		defer close(controlsDone)
		for {
			select {
			case control := <-state.controls:
				requestedID := ojson.GetBytes(control.body, "response_id").String()
				if (control.streamID != "" && control.streamID != create.StreamID) || (requestedID != "" && requestedID != responseID.Load().(string)) {
					if control.interrupted != nil {
						_ = s.sendInterruptAcknowledgement(control, requestedID, requestedID != "" && requestedID == lastResponseID)
						control.interrupted <- false
					} else {
						s.sendError(control.eventID, control.streamID, newResponsesWSInvalidRequestError(fmt.Errorf("response is not active on this stream")))
					}
					continue
				}
				if control.interrupted != nil {
					id := responseID.Load().(string)
					if id == "" {
						id = "resp_" + common.NewRequestId()
						responseID.Store(id)
					}
					control.interrupted <- true
					if err := s.sendInterruptAcknowledgement(control, id, true); err != nil {
						s.shutdown(websocket.CloseInternalServerErr, "")
					}
					interrupted.Store(true)
				}
				cancelled.Store(true)
				cancel()
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	defer func() { cancel(); <-controlsDone }()
	var generated strings.Builder
	accepted := false
	nextSequenceNumber := 0
	info.StreamStatus = relaycommon.NewStreamStatus()
	info.StreamStatus.RequireTerminal()
	c.Set("responses_event_writer", func(data string) error {
		var event dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &event); err != nil {
			return err
		}
		if event.SequenceNumber != nil {
			nextSequenceNumber = max(nextSequenceNumber, *event.SequenceNumber+1)
		}
		accepted = true
		if event.Response != nil && event.Response.ID != "" {
			responseID.Store(event.Response.ID)
		}
		switch event.Type {
		case "response.output_text.delta", "response.function_call_arguments.delta", "response.reasoning_summary_text.delta":
			generated.WriteString(event.Delta)
		}
		body := []byte(data)
		if create.StreamID != "" {
			var err error
			body, err = ojson.SetBytes(body, "stream_id", create.StreamID)
			if err != nil {
				return err
			}
		}
		switch event.Type {
		case "response.completed", "response.done", "response.failed", "response.incomplete", "response.cancelled", "response.canceled":
			if event.Response != nil {
				s.lastResponseID = event.Response.ID
			}
			state.terminal = &responsesWSMessage{kind: websocket.TextMessage, body: body}
			return nil
		}
		return s.writeClient(websocket.TextMessage, body)
	})
	defer delete(c.Keys, "responses_event_writer")

	usage, apiErr := relayResponsesHTTP(c, info, &create.Request)
	if apiErr != nil && !accepted && !cancelled.Load() {
		return apiErr
	}
	if usage == nil {
		usage = service.ResponseText2Usage(c, generated.String(), info.UpstreamModelName, info.GetEstimatePromptTokens())
		if !accepted {
			usage = &dto.Usage{}
		}
	}
	if state.terminal == nil && s.ctx.Err() == nil {
		kind, status := "response.failed", "failed"
		if cancelled.Load() {
			kind, status = "response.cancelled", "cancelled"
		}
		response := map[string]any{"id": responseID.Load().(string), "object": "response", "status": status, "model": create.Request.Model, "output": []any{}}
		if apiErr != nil && !cancelled.Load() {
			response["error"] = apiErr.ToOpenAIError(common.UpstreamAddresses(c)...)
		}
		var body []byte
		var err error
		if interrupted.Load() {
			body, err = buildResponsesWSInterruptedEvent(responseID.Load().(string), create.Request.Model, create.StreamID, nextSequenceNumber)
		} else {
			body, err = common.Marshal(map[string]any{"type": kind, "stream_id": create.StreamID, "response": response})
		}
		if err != nil {
			return types.NewError(err, types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
		}
		if interrupted.Load() {
			s.lastResponseID = responseID.Load().(string)
			if err := service.SaveResponsesContext(c, s.lastResponseID, &create.Request, []byte(ojson.GetBytesNoCopy(body, "response").Raw)); err != nil {
				ConsumeResponsesQuota(c, info, usage)
				return types.NewError(err, types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
			}
		}
		state.terminal = &responsesWSMessage{kind: websocket.TextMessage, body: body}
	}
	ConsumeResponsesQuota(c, info, usage)
	return nil
}
