package relay

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func ResponsesHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	var responsesReq *dto.OpenAIResponsesRequest
	switch req := info.Request.(type) {
	case *dto.OpenAIResponsesRequest:
		responsesReq = req
	case *dto.OpenAIResponsesCompactionRequest:
		// Only fields documented for POST /v1/responses/compact are forwarded:
		// model, input, instructions, previous_response_id, prompt_cache_key,
		// prompt_cache_options, prompt_cache_retention, service_tier.
		// Undocumented Codex-parity fields (tools, reasoning, text) are parsed
		// for client compatibility but intentionally not sent upstream.
		responsesReq = &dto.OpenAIResponsesRequest{
			Model:                req.Model,
			Input:                req.Input,
			Instructions:         req.Instructions,
			PreviousResponseID:   req.PreviousResponseID,
			ParallelToolCalls:    req.ParallelToolCalls,
			ServiceTier:          req.ServiceTier,
			PromptCacheKey:       req.PromptCacheKey,
			PromptCacheOptions:   req.PromptCacheOptions,
			PromptCacheRetention: req.PromptCacheRetention,
		}
	default:
		return types.NewErrorWithStatusCode(
			fmt.Errorf("invalid request type, expected dto.OpenAIResponsesRequest or dto.OpenAIResponsesCompactionRequest, got %T", info.Request),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	usageDto, newAPIError := relayResponsesHTTP(c, info, responsesReq)
	if newAPIError != nil {
		if usageDto != nil {
			ConsumeResponsesQuota(c, info, usageDto)
		}
		return newAPIError
	}
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		originModelName := info.OriginModelName
		originPriceData := info.PriceData

		_, err := helper.ModelPriceHelper(c, info, info.GetEstimatePromptTokens(), &types.TokenCountMeta{})
		if err != nil {
			info.OriginModelName = originModelName
			info.PriceData = originPriceData
			return types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithSkipRetry(), types.ErrOptionWithStatusCode(http.StatusBadRequest))
		}
		service.PostTextConsumeQuota(c, info, usageDto, nil)

		info.OriginModelName = originModelName
		info.PriceData = originPriceData
		return nil
	}

	ConsumeResponsesQuota(c, info, usageDto)
	return nil
}

// relayResponsesHTTP shares request preparation and protocol conversion between
// HTTP and the WebSocket bridge. Each transport settles the returned usage once.
func relayResponsesHTTP(c *gin.Context, info *relaycommon.RelayInfo, request *dto.OpenAIResponsesRequest) (*dto.Usage, *types.NewAPIError) {
	adaptor, body, closer, apiErr := PrepareResponsesRequest(c, info, request)
	if apiErr != nil {
		return nil, apiErr
	}
	defer closer.Close()
	response, err := adaptor.DoRequest(c, info, body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	resp, ok := response.(*http.Response)
	if !ok || resp == nil {
		return nil, types.NewError(fmt.Errorf("invalid upstream response"), types.ErrorCodeBadResponse)
	}
	statusMapping := c.GetString("status_code_mapping")
	if resp.StatusCode != http.StatusOK {
		apiErr = service.RelayErrorHandler(c.Request.Context(), resp, false)
		service.ResetStatusCode(apiErr, statusMapping)
		return nil, apiErr
	}
	processor := service.ResponsesOutputProcessor{
		Context: c, Info: info, Request: request,
		SummarizeReasoning: responsesReasoningSummarizer(c, request),
	}
	var processingErr error
	var buffered *responsesJSONWriter
	var collectedResponse common.RawMessage
	clientStream := info.IsStream
	// Some providers always send SSE, even for stream=false. Run their stream
	// adaptor, then return the terminal response as one JSON object.
	upstreamStream := strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream")
	if !clientStream && upstreamStream {
		info.IsStream = true
		defer func() { info.IsStream = clientStream }()
	}
	originalWriter := c.Writer
	previousSink, hadSink := c.Get("responses_event_writer")
	if !clientStream {
		buffered = &responsesJSONWriter{ResponseWriter: originalWriter}
		c.Writer = buffered
		defer func() { c.Writer = originalWriter }()
	}
	if clientStream || upstreamStream {
		c.Set("responses_event_writer", func(data string) error {
			if processingErr != nil {
				return processingErr
			}
			var events []string
			events, processingErr = processor.ProcessEvent(data)
			if processingErr != nil {
				return processingErr
			}
			for _, event := range events {
				if !clientStream {
					var envelope struct {
						Type     string            `json:"type"`
						Response common.RawMessage `json:"response"`
					}
					if err := common.UnmarshalJsonStr(event, &envelope); err != nil {
						return err
					}
					switch envelope.Type {
					case "response.completed", "response.incomplete", "response.failed":
						collectedResponse = envelope.Response
					}
					continue
				}
				if hadSink {
					if err := previousSink.(func(string) error)(event); err != nil {
						return err
					}
					continue
				}
				var envelope dto.ResponsesStreamResponse
				if err := common.UnmarshalJsonStr(event, &envelope); err != nil {
					return err
				}
				if err := helper.WriteResponsesEvent(c, envelope, event); err != nil {
					return err
				}
			}
			return nil
		})
		defer func() {
			if hadSink {
				c.Set("responses_event_writer", previousSink)
			} else {
				delete(c.Keys, "responses_event_writer")
			}
		}()
	}
	usage, apiErr := adaptor.DoResponse(c, resp, info)
	usageDto, _ := usage.(*dto.Usage)
	if apiErr != nil {
		service.ResetStatusCode(apiErr, statusMapping)
		return usageDto, apiErr
	}
	if usageDto == nil {
		return nil, types.NewError(fmt.Errorf("missing upstream usage"), types.ErrorCodeBadResponse)
	}
	if buffered != nil {
		var body []byte
		if upstreamStream {
			body = collectedResponse
			if len(body) == 0 && processingErr == nil {
				processingErr = fmt.Errorf("upstream stream ended without a terminal Responses event")
			}
		} else {
			body, processingErr = processor.ProcessResponse(buffered.body.Bytes())
		}
		if processingErr == nil {
			c.Writer = originalWriter
			c.Header("Content-Type", "application/json")
			c.Header("Content-Length", "")
			_, processingErr = originalWriter.Write(body)
		}
	}
	if processingErr != nil {
		return usageDto, types.NewError(processingErr, types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
	}
	return usageDto, nil
}

type responsesJSONWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *responsesJSONWriter) Write(data []byte) (int, error)       { return w.body.Write(data) }
func (w *responsesJSONWriter) WriteString(data string) (int, error) { return w.body.WriteString(data) }
func (w *responsesJSONWriter) WriteHeader(int)                      {}
func (w *responsesJSONWriter) WriteHeaderNow()                      {}
func (w *responsesJSONWriter) Flush()                               {}
func (w *responsesJSONWriter) Written() bool                        { return w.body.Len() > 0 }
func (w *responsesJSONWriter) Size() int                            { return w.body.Len() }

// ConsumeResponsesQuota applies the same settlement dispatch to HTTP and
// WebSocket Responses usage. Compact requests keep their separate repricing.
func ConsumeResponsesQuota(c *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage) {
	if c.GetBool("responses_warmup") {
		if info.Billing != nil {
			info.Billing.Refund(c)
		}
		return
	}
	if strings.HasPrefix(info.OriginModelName, "gpt-4o-audio") {
		service.PostAudioConsumeQuota(c, info, usage, nil)
		return
	}
	service.PostTextConsumeQuota(c, info, usage, nil)
}
