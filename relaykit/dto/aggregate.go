package dto

import "fmt"

// AggregateConfig deliberately has no per-endpoint credentials or model rules:
// all protocols belong to the same supplier and expose the channel's model list.
type AggregateConfig struct {
	ChatPath      string `json:"chat_path,omitempty"`
	MessagesPath  string `json:"messages_path,omitempty"`
	ResponsesPath string `json:"responses_path,omitempty"`
	ModelsPath    string `json:"models_path,omitempty"`
	WebSocket     bool   `json:"websocket,omitempty"`
}

func (a *AggregateConfig) Routes() *AdvancedCustomConfig {
	config := &AdvancedCustomConfig{}
	if a == nil {
		return config
	}
	// A matching native protocol always wins. Missing protocols use the existing
	// converter pipeline, retaining its usage, reasoning and transport policies.
	for _, incoming := range []string{"/v1/chat/completions", "/v1/messages", "/v1/responses", "/v1beta/models/{model}:generateContent"} {
		path, converter := "", advancedCustomConverterNone
		switch {
		case incoming == "/v1/chat/completions" && a.ChatPath != "":
			path = a.ChatPath
		case incoming == "/v1/messages" && a.MessagesPath != "":
			path = a.MessagesPath
		case incoming == "/v1/responses" && a.ResponsesPath != "":
			path = a.ResponsesPath
		case a.ResponsesPath != "":
			path = a.ResponsesPath
			switch incoming {
			case "/v1/chat/completions":
				converter = advancedCustomConverterOpenAIChatToOpenAIResponses
			case "/v1/messages":
				converter = advancedCustomConverterClaudeMessagesToOpenAIResponses
			default:
				converter = advancedCustomConverterGeminiContentToOpenAIResponses
			}
		case a.ChatPath != "":
			path = a.ChatPath
			switch incoming {
			case "/v1/messages":
				converter = advancedCustomConverterClaudeMessagesToOpenAIChat
			case "/v1/responses":
				converter = advancedCustomConverterOpenAIResponsesToOpenAIChat
			default:
				converter = advancedCustomConverterGeminiContentToOpenAIChat
			}
		case a.MessagesPath != "":
			path = a.MessagesPath
			switch incoming {
			case "/v1/chat/completions":
				converter = advancedCustomConverterOpenAIChatToClaudeMessages
			case "/v1/responses":
				converter = advancedCustomConverterOpenAIResponsesToClaude
			default:
				converter = advancedCustomConverterGeminiToClaude
			}
		}
		if path == "" {
			continue
		}
		route := AdvancedCustomRoute{IncomingPath: incoming, UpstreamPath: path, Converter: converter}
		if (incoming == "/v1/messages" && converter == advancedCustomConverterNone) ||
			converter == advancedCustomConverterOpenAIChatToClaudeMessages || converter == advancedCustomConverterOpenAIResponsesToClaude || converter == advancedCustomConverterGeminiToClaude {
			route.Auth = &AdvancedCustomRouteAuth{Type: AdvancedCustomAuthTypeHeader, Name: "x-api-key", Value: "{api_key}"}
		}
		config.Routes = append(config.Routes, route)
	}
	if a.ModelsPath != "" {
		config.Routes = append(config.Routes, AdvancedCustomRoute{IncomingPath: AdvancedCustomModelListPath, UpstreamPath: a.ModelsPath, Converter: advancedCustomConverterNone})
	}
	return config
}

func (a *AggregateConfig) Validate() error {
	if a == nil || (a.ChatPath == "" && a.MessagesPath == "" && a.ResponsesPath == "") {
		return fmt.Errorf("aggregate requires at least one upstream protocol")
	}
	if a.WebSocket && a.ResponsesPath == "" {
		return fmt.Errorf("aggregate WebSocket requires a native Responses endpoint")
	}
	return a.Routes().Validate()
}
