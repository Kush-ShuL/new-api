package relay

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/MoeForge/moego/ojson"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaychannel "github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relay/requestplugin"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
)

// PrepareResponsesRequest applies the same model, conversion and channel rules
// for HTTP and WebSocket requests. The caller closes closer after the attempt;
// passthrough bodies remain owned by the incoming request's BodyStorage.
// The returned adaptor retains route/conversion state for DoRequest/DoResponse.
func PrepareResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.OpenAIResponsesRequest) (relaychannel.Adaptor, common.ReplayableBody, io.Closer, *types.NewAPIError) {
	info.InitChannelMeta(c)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact &&
		!common.SupportsResponsesCompact(info.ChannelType, info.ApiType) {
		return nil, nil, nil, types.NewErrorWithStatusCode(
			fmt.Errorf("unsupported endpoint %q for api type %d", "/v1/responses/compact", info.ApiType),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	request, err := common.DeepCopy(req)
	if err != nil {
		return nil, nil, nil, types.NewError(fmt.Errorf("failed to copy responses request: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	if err := helper.ModelMappedHelper(c, info, request); err != nil {
		return nil, nil, nil, types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}
	if err := helper.ApplyReasoningModelSuffix(c, info, request); err != nil {
		return nil, nil, nil, newConvertRequestFailedError(c, info, err)
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return nil, nil, nil, types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)
	if (model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled) && !c.GetBool("responses_http_bridge") && !c.GetBool("responses_ws_native") && !c.GetBool("responses_input_rewritten") && len(request.ContextManagement) == 0 {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return nil, nil, nil, types.NewError(err, types.ErrorCodeReadRequestBodyFailed, types.ErrOptionWithSkipRetry())
		}
		if apiErr := requestplugin.PassThrough(c, info, storage); apiErr != nil {
			return nil, nil, nil, apiErr
		}
		body := common.NewReplayableBodyReader(storage)
		return adaptor, body, io.NopCloser(body), nil
	}

	convertedRequest, err := adaptor.ConvertOpenAIResponsesRequest(c, info, *request)
	if err != nil {
		return nil, nil, nil, newConvertRequestFailedError(c, info, err)
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return nil, nil, nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	if raw, ok := c.Get("responses_ws_body"); ok && c.GetBool("responses_ws_native") {
		doc := ojson.NewDoc(raw.([]byte))
		if err := doc.Set("model", request.Model); err != nil {
			return nil, nil, nil, newConvertRequestFailedError(c, info, err)
		}
		if request.Reasoning != nil {
			if err := doc.Set("reasoning", request.Reasoning); err != nil {
				return nil, nil, nil, newConvertRequestFailedError(c, info, err)
			}
		}
		for key, value := range map[string]common.RawMessage{"context_management": request.ContextManagement} {
			if len(value) > 0 {
				if err := doc.SetRawBytes(key, value); err != nil {
					return nil, nil, nil, newConvertRequestFailedError(c, info, err)
				}
			}
		}
		if c.GetBool("responses_pool_expand_context") {
			if err := doc.SetRawBytes("input", request.Input); err != nil {
				return nil, nil, nil, newConvertRequestFailedError(c, info, err)
			}
			if err := relaycommon.DeleteJSONMember(doc, "previous_response_id"); err != nil {
				return nil, nil, nil, newConvertRequestFailedError(c, info, err)
			}
		}
		jsonData = doc.Bytes()
	}
	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, nil, nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	jsonData, apiErr := requestplugin.Outbound(c, info, jsonData)
	if apiErr != nil {
		return nil, nil, nil, apiErr
	}
	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return nil, nil, nil, newAPIErrorFromParamOverride(err)
		}
	}
	if finalizer, ok := adaptor.(relaychannel.ResponsesRequestFinalizer); ok {
		jsonData, err = finalizer.FinalizeResponsesRequest(info, jsonData)
		if err != nil {
			return nil, nil, nil, newConvertRequestFailedError(c, info, err)
		}
	}
	if info.GetFinalRequestRelayFormat() == types.RelayFormatOpenAIResponses {
		var final dto.OpenAIResponsesRequest
		if err := common.Unmarshal(jsonData, &final); err != nil {
			return nil, nil, nil, newConvertRequestFailedError(c, info, err)
		}
		if helper.ExceedsMaxTokensLimit(final.MaxOutputTokens) {
			return nil, nil, nil, newConvertRequestFailedError(c, info, fmt.Errorf("max_output_tokens is invalid"))
		}
		if err := service.ValidateResponsesContextManagement(info.OriginModelName, &final); err != nil {
			return nil, nil, nil, newConvertRequestFailedError(c, info, err)
		}
		if len(final.ContextManagement) > 0 {
			jsonData, err = ojson.SetRawBytes(jsonData, "context_management", final.ContextManagement)
			if err != nil {
				return nil, nil, nil, newConvertRequestFailedError(c, info, err)
			}
		}
	}
	// egress reads the body as it is sent, after the channel finalized it.
	if apiErr = requestplugin.Egress(c, info, jsonData); apiErr != nil {
		return nil, nil, nil, apiErr
	}

	logger.LogDebug(c, "requestBody: %s", jsonData)
	body, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return nil, nil, nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	return adaptor, body, closer, nil
}

// Resolve missing call IDs against the tool calls in the expanded HTTP input.
// Never bind an unlabelled result to one of several outstanding calls.
// A nil result means the original input can be forwarded unchanged.
func resolveHTTPResponsesToolOutputs(input common.RawMessage) (common.RawMessage, error) {
	if common.GetJsonType(input) != "array" {
		return nil, nil
	}
	var items []common.RawMessage
	if err := common.Unmarshal(input, &items); err != nil {
		return nil, err
	}
	type pendingCall struct {
		kind   string
		itemID string
	}
	pending := make(map[string]pendingCall)
	changed := false
	for i, item := range items {
		kind := ojson.GetBytesNoCopy(item, "type").String()
		callID := ojson.GetBytesNoCopy(item, "call_id").String()
		switch kind {
		case "function_call", "custom_tool_call":
			if callID != "" {
				pending[callID] = pendingCall{kind: kind, itemID: ojson.GetBytesNoCopy(item, "id").String()}
			}
		case "message":
			clear(pending)
		case "function_call_output", "custom_tool_call_output":
			if callID != "" {
				delete(pending, callID)
				continue
			}
			callKind := strings.TrimSuffix(kind, "_output")
			itemID := ojson.GetBytesNoCopy(item, "id").String()
			matches := 0
			for id, call := range pending {
				if call.kind != callKind {
					continue
				}
				callID = id
				matches++
				if itemID != "" && (itemID == id || itemID == call.itemID) {
					matches = 1
					break
				}
			}
			if matches != 1 {
				return nil, fmt.Errorf("input[%d].call_id is required: no unique matching call for %s", i, kind)
			}
			var err error
			items[i], err = ojson.SetBytes(item, "call_id", callID)
			if err != nil {
				return nil, err
			}
			delete(pending, callID)
			changed = true
		case "":
			if ojson.GetBytesNoCopy(item, "role").String() != "" {
				clear(pending)
			}
		}
	}
	if !changed {
		return nil, nil
	}
	return common.Marshal(items)
}
