package relay

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	appdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relay/requestplugin"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type decisionOpenAIAdaptor struct {
	openai.Adaptor
	upstreamPath    string
	enableJevImages bool
}

func (a *decisionOpenAIAdaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	base := strings.TrimSuffix(info.ChannelBaseUrl, "/")
	if strings.HasSuffix(base, "/v1") {
		base = strings.TrimSuffix(base, "/v1")
	}
	return relaycommon.GetFullRequestURL(base, a.upstreamPath, info.ChannelType), nil
}

func (a *decisionOpenAIAdaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	if err := a.Adaptor.SetupRequestHeader(c, header, info); err != nil {
		return err
	}
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	if info.ChannelType == constant.ChannelTypeTypeSafe {
		if a.enableJevImages || strings.EqualFold(c.GetHeader("TypeSafe-Danger-Zone-Images"), "true") {
			header.Set("TypeSafe-Danger-Zone-Images", "true")
		}
	}
	return nil
}

func DecisionHelper(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	info.InitChannelMeta(c)
	request, ok := info.Request.(*appdto.DecisionRequest)
	if !ok {
		return types.NewError(errors.New("invalid decision request"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	if err := helper.ModelMappedHelper(c, info, request); err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	path := c.Request.URL.Path
	upstreamPath := common.DecisionUpstreamPath(info.ChannelType, path)
	var adaptor channel.Adaptor
	if constant.IsAdvancedCustomChannel(info.ChannelType) {
		route, matched := info.ChannelOtherSettings.AdvancedCustom.MatchPathForModel(path, info.OriginModelName)
		if matched && route.IsNative() {
			upstreamPath = path
			switch {
			case strings.HasSuffix(route.UpstreamPath, common.SystemOnePath):
				upstreamPath = common.SystemOnePath
			case strings.HasSuffix(route.UpstreamPath, common.DecisionsPath):
				upstreamPath = common.DecisionsPath
			}
			adaptor = GetAdaptor(info.ApiType)
		}
	}
	if upstreamPath == "" {
		return types.NewErrorWithStatusCode(fmt.Errorf("channel does not support %s", path), types.ErrorCodeInvalidRequest, http.StatusBadRequest)
	}
	var direct *decisionOpenAIAdaptor
	if adaptor == nil {
		direct = &decisionOpenAIAdaptor{upstreamPath: upstreamPath}
		adaptor = direct
	}
	adaptor.Init(info)
	body := maps.Clone(request.Body)
	model, err := common.Marshal(info.UpstreamModelName)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	body["model"] = model

	var outbound any = body
	var mappings []decisionMapping
	if upstreamPath != path {
		var converted map[string]any
		converted, mappings, err = convertDecisionRequest(body, upstreamPath == common.SystemOnePath, info.ChannelType == constant.ChannelTypeOllama)
		if err != nil {
			// Another channel may serve the request natively.
			return types.NewErrorWithStatusCode(err, types.ErrorCodeConvertRequestFailed, http.StatusBadRequest)
		}
		if direct != nil {
			_, direct.enableJevImages = converted["assets"]
		}
		outbound = converted
	}
	if upstreamPath == common.SystemOnePath {
		info.AppendRequestConversion(types.RelayFormatSystemOne)
	} else {
		info.AppendRequestConversion(types.RelayFormatOpenAIDecisions)
	}
	jsonData, err := common.Marshal(outbound)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	jsonData, apiErr := requestplugin.Outbound(c, info, jsonData)
	if apiErr != nil {
		return apiErr
	}
	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return newAPIErrorFromParamOverride(err)
		}
	}
	if apiErr = requestplugin.Egress(c, info, jsonData); apiErr != nil {
		return apiErr
	}
	reader, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()

	response, err := channel.DoApiRequest(adaptor, c, info, reader)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusBadGateway)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		apiErr = service.RelayErrorHandler(c.Request.Context(), response, false)
		service.ResetStatusCode(apiErr, c.GetString("status_code_mapping"))
		return apiErr
	}
	const maxDecisionReply = 32 << 20
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxDecisionReply+1))
	if err != nil || len(responseBody) > maxDecisionReply {
		return types.NewOpenAIError(errors.New("invalid or oversized decision response"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	var reply decisionReply
	if err := common.Unmarshal(responseBody, &reply); err != nil || reply.Model == "" ||
		(upstreamPath == common.SystemOnePath && common.GetJsonType(reply.Answers) != "object") ||
		(upstreamPath == common.DecisionsPath && common.GetJsonType(reply.Answers) != "array") {
		return types.NewOpenAIError(errors.New("invalid upstream decision response"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	usage, err := reply.Usage.billing()
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	info.ObserveResponseModel(reply.Model)
	if upstreamPath != path {
		responseBody, err = convertDecisionReply(reply, mappings, upstreamPath == common.SystemOnePath)
		if err != nil {
			return types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusBadGateway)
		}
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", responseBody)
	service.PostTextConsumeQuota(c, info, usage, nil)
	return nil
}
