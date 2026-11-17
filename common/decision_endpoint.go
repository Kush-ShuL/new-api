package common

import (
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

const (
	DecisionsPath = "/v1/decisions"
	SystemOnePath = "/v1/systemone"
)

func IsJevDecisionModel(name string) bool {
	return strings.HasPrefix(name, "jev-") || name == "autotrust-jev" || strings.HasPrefix(name, "autotrust-jev:")
}

// DecisionUpstreamPath identifies the native wire format of a built-in channel.
func DecisionUpstreamPath(channelType int, path string) string {
	switch channelType {
	case constant.ChannelTypeOpenAI:
		return DecisionsPath
	case constant.ChannelTypeTypeSafe, constant.ChannelTypeOllama:
		return SystemOnePath
	case constant.ChannelTypeNewAPI:
		return path
	default:
		return ""
	}
}
