package model

import (
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// ModelAllowsEndpoint applies the same effective metadata rule as the catalog.
// Empty endpoint configuration keeps the channel's inferred capabilities.
func ModelAllowsEndpoint(name, method, path string) bool {
	modelSupportEndpointsLock.RLock()
	metadata := modelEndpointPolicies[name]
	if metadata == nil {
		metadata = resolveModelMetadata(modelEndpointMetadata, []string{name})[name]
	}
	if normalized := ratio_setting.RoutingMatchModelName(name); metadata == nil && normalized != name {
		metadata = modelEndpointPolicies[normalized]
		if metadata == nil {
			metadata = resolveModelMetadata(modelEndpointMetadata, []string{normalized})[normalized]
		}
	}
	modelSupportEndpointsLock.RUnlock()
	if metadata == nil {
		return true
	}
	endpoints, err := declaredEndpointNames(metadata.Endpoints)
	if err != nil || len(endpoints) == 0 {
		return true
	}

	path = strings.TrimSuffix(path, "/")
	var endpoint constant.EndpointType
	if path == "/v1/messages" || path == "/v1/messages/count_tokens" {
		endpoint = constant.EndpointTypeAnthropic
	} else if path == "/v1/videos" || strings.HasPrefix(path, "/v1/videos/") {
		endpoint = constant.EndpointTypeOpenAIVideo
	} else {
		switch relayconstant.Path2RelayMode(path) {
		case relayconstant.RelayModeChatCompletions, relayconstant.RelayModeCompletions:
			endpoint = constant.EndpointTypeOpenAI
		case relayconstant.RelayModeResponses:
			endpoint = constant.EndpointTypeOpenAIResponse
		case relayconstant.RelayModeResponsesCompact:
			endpoint = constant.EndpointTypeOpenAIResponseCompact
		case relayconstant.RelayModeAlphaSearch:
			endpoint = constant.EndpointTypeOpenAIAlphaSearch
		case relayconstant.RelayModeToolsSearch:
			endpoint = constant.EndpointTypeDeepSeekToolsSearch
		case relayconstant.RelayModeToolsFetch:
			endpoint = constant.EndpointTypeDeepSeekToolsFetch
		case relayconstant.RelayModeGemini:
			endpoint = constant.EndpointTypeGemini
		case relayconstant.RelayModeEmbeddings:
			endpoint = constant.EndpointTypeEmbeddings
		case relayconstant.RelayModeRerank:
			endpoint = constant.EndpointTypeJinaRerank
		case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
			endpoint = constant.EndpointTypeImageGeneration
		}
	}
	if endpoint != "" {
		return slices.Contains(endpoints, string(endpoint))
	}

	// Custom protocols have no built-in type mapping, so their configured
	// request path and method identify the accepted endpoint.
	var configured map[string]any
	if common.UnmarshalJsonStr(metadata.Endpoints, &configured) != nil {
		return false
	}
	for _, value := range configured {
		info := common.EndpointInfo{Method: "POST"}
		switch value := value.(type) {
		case string:
			info.Path = value
		case map[string]any:
			info.Path, _ = value["path"].(string)
			if configuredMethod, ok := value["method"].(string); ok {
				info.Method = configuredMethod
			}
		}
		if path == strings.TrimSuffix(info.Path, "/") && method == info.Method {
			return true
		}
	}
	return false
}
