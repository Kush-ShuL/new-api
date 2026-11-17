package model

import (
	"errors"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

type ModelEndpointPolicy struct {
	Mode          string                         `json:"mode"`
	Endpoints     []string                       `json:"endpoints"`
	Paths         map[string]common.EndpointInfo `json:"-"`
	WebSocketMode string                         `json:"websocket_mode,omitempty"`
}

// ParseModelEndpointPolicy keeps legacy declarations as whitelists. An empty
// legacy declaration has no restriction; an explicit empty whitelist denies all.
func ParseModelEndpointPolicy(raw string) (*ModelEndpointPolicy, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var value any
	if err := common.UnmarshalJsonStr(raw, &value); err != nil {
		return nil, err
	}
	policy := &ModelEndpointPolicy{Mode: "whitelist", Endpoints: []string{}, Paths: make(map[string]common.EndpointInfo)}
	explicit := false
	if object, ok := value.(map[string]any); ok {
		if mode, exists := object["mode"]; exists {
			if mode != "whitelist" && mode != "blacklist" {
				return nil, errors.New("endpoint mode must be whitelist or blacklist")
			}
			policy.Mode = mode.(string)
			if wsMode, exists := object["websocket_mode"]; exists {
				text, ok := wsMode.(string)
				if !ok {
					return nil, errors.New("websocket_mode must be a string")
				}
				if err := dto.ValidateResponsesWebSocketMode(text); err != nil {
					return nil, err
				}
				policy.WebSocketMode = text
			}
			value = object["endpoints"]
			explicit = true
		}
	}
	switch endpoints := value.(type) {
	case []any:
		for _, endpoint := range endpoints {
			name, ok := endpoint.(string)
			if !ok || strings.TrimSpace(name) == "" {
				return nil, errors.New("endpoint types must be non-empty strings")
			}
			policy.Endpoints = append(policy.Endpoints, strings.TrimSpace(name))
		}
	case map[string]any:
		for name, value := range endpoints {
			name = strings.TrimSpace(name)
			if name == "" {
				return nil, errors.New("endpoint type is required")
			}
			info := common.EndpointInfo{Method: "POST"}
			switch details := value.(type) {
			case string:
				info.Path = details
			case map[string]any:
				info.Path, _ = details["path"].(string)
				if method, exists := details["method"]; exists {
					info.Method, _ = method.(string)
				}
			default:
				return nil, errors.New("endpoint configuration must be a path or object")
			}
			if !strings.HasPrefix(info.Path, "/") {
				return nil, errors.New("endpoint paths must start with /")
			}
			switch info.Method {
			case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
			default:
				return nil, errors.New("invalid endpoint HTTP method")
			}
			policy.Endpoints = append(policy.Endpoints, name)
			policy.Paths[name] = info
		}
	default:
		return nil, errors.New("endpoints must be a JSON object or array")
	}
	if !explicit && len(policy.Endpoints) == 0 {
		return nil, nil
	}
	slices.Sort(policy.Endpoints)
	policy.Endpoints = slices.Compact(policy.Endpoints)
	return policy, nil
}

func modelEndpointMetadataForName(name string) *Model {
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
	return metadata
}

// ModelAllowsEndpoint applies the same effective metadata rule as the catalog.
func ModelAllowsEndpoint(name, method, path string) bool {
	metadata := modelEndpointMetadataForName(name)
	if metadata == nil {
		return true
	}
	policy, err := ParseModelEndpointPolicy(metadata.Endpoints)
	if err != nil || policy == nil {
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
		case relayconstant.RelayModeDecisions:
			endpoint = constant.EndpointTypeOpenAIDecisions
			if path == common.SystemOnePath {
				endpoint = constant.EndpointTypeSystemOne
			}
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
	matched := endpoint != "" && slices.Contains(policy.Endpoints, string(endpoint))
	for _, info := range policy.Paths {
		if path == strings.TrimSuffix(info.Path, "/") && method == info.Method {
			matched = true
			break
		}
	}
	if policy.Mode == "blacklist" {
		return !matched
	}
	return matched
}

func (ch *Channel) ResponsesWebSocketMode(modelName string) string {
	if metadata := modelEndpointMetadataForName(modelName); metadata != nil {
		if policy, err := ParseModelEndpointPolicy(metadata.Endpoints); err == nil && policy != nil && policy.WebSocketMode != "" {
			return policy.WebSocketMode
		}
	}
	settings := ch.GetSetting()
	if mode := settings.ResponsesWebSocketMode; mode != "" {
		return mode
	}
	if ch.Type == constant.ChannelTypeTaskPlugin && settings.ResponsesWebSocketEnabled {
		return dto.ResponsesWSPassthrough
	}
	return dto.ResponsesWSHTTPBridge
}
