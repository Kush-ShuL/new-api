package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

func TestResponsesSummaryUsesModelForRequestedDetail(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	previousSettings := *settings
	previousDB, previousCache := model.DB, common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Option{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.DB, common.MemoryCacheEnabled = db, false
	t.Cleanup(func() {
		*settings = previousSettings
		model.DB, common.MemoryCacheEnabled = previousDB, previousCache
		require.NoError(t, sqlDB.Close())
	})
	var models []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request dto.OpenAIResponsesRequest
		require.NoError(t, common.DecodeJson(r.Body, &request))
		models = append(models, request.Model)
		assert.Equal(t, "summary-key", strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		assert.False(t, request.IsStream(r))
		assert.Equal(t, uint(256), *request.MaxOutputTokens)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"resp_summary","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Checking the mathematical domain.","annotations":[]}]}],"usage":{"input_tokens":12,"output_tokens":5,"total_tokens":17}}`)
	}))
	defer server.Close()
	channel := &model.Channel{Id: 101, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "summary-key", BaseURL: common.GetPointer(server.URL), Models: "summary-concise,summary-auto,summary-detailed"}
	require.NoError(t, db.Create(channel).Error)
	settings.ResponsesReasoningEncryption = true
	settings.ResponsesReasoningSummaryMode = "model"
	settings.ResponsesReasoningSummaryModels = make(map[string]model_setting.ResponsesSummaryModel)
	for _, level := range []string{"concise", "auto", "detailed"} {
		settings.ResponsesReasoningSummaryModels[level] = model_setting.ResponsesSummaryModel{ChannelID: 101, Model: "summary-" + level}
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	common.SetContextKey(c, constant.ContextKeyUserId, 1)
	for _, level := range []string{"concise", "auto", "detailed"} {
		summarize := responsesReasoningSummarizer(c, &dto.OpenAIResponsesRequest{Reasoning: &dto.Reasoning{Summary: level}})
		require.NotNil(t, summarize)
		text, err := summarize("Interesting! The function values are tiny.\n\nReconsider its domain.")
		require.NoError(t, err)
		assert.Equal(t, "Checking the mathematical domain.", text)
	}
	assert.Equal(t, []string{"summary-concise", "summary-auto", "summary-detailed"}, models)
	c.Set("responses_summary_request", true)
	assert.Nil(t, responsesReasoningSummarizer(c, &dto.OpenAIResponsesRequest{}))
}

func TestPrepareResponsesRequestDeepSeekEffectiveReasoning(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	previous := settings.PassThroughRequestEnabled
	t.Cleanup(func() { settings.PassThroughRequestEnabled = previous })
	settings.PassThroughRequestEnabled = false
	const input = `[{"type":"function_call","call_id":"c1","name":"lookup","arguments":"{}"}]`
	for _, tc := range []struct {
		name, effort, override, endpoint         string
		passthrough, replaceInput, wantReasoning bool
		globalPassthrough                        bool
	}{
		{name: "override enables thinking", effort: "none", override: "high", wantReasoning: true},
		{name: "override disables thinking", effort: "high", override: "none"},
		{name: "override replaces history", effort: "high", override: "high", replaceInput: true, wantReasoning: true},
		{name: "no override", effort: "high", wantReasoning: true},
		{name: "thinking disabled", effort: "none"},
		{name: "third party endpoint", effort: "none", override: "high", endpoint: "https://other.example"},
		{name: "passthrough bypasses overrides and normalization", effort: "none", override: "high", passthrough: true},
		{name: "global passthrough bypasses normalization", effort: "high", globalPassthrough: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings.PassThroughRequestEnabled = tc.globalPassthrough
			endpoint := tc.endpoint
			if endpoint == "" {
				endpoint = "https://API.DEEPSEEK.COM:443/v1"
			}
			raw := `{"model":"deepseek-flash","reasoning":{"effort":"` + tc.effort + `"},"input":` + input + `}`
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(raw))
			c.Request.Header.Set("Content-Type", "application/json")
			t.Cleanup(func() { common.CleanupBodyStorage(c) })
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "deepseek-flash")
			common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeDeepSeek)
			common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, endpoint)
			common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: tc.passthrough})
			if tc.override != "" {
				ops := []any{map[string]any{"path": "reasoning.effort", "mode": "set", "value": tc.override}, map[string]any{"path": "vendor_extension", "mode": "set", "value": "preserved"}}
				if tc.replaceInput {
					var replacement any
					require.NoError(t, common.Unmarshal([]byte(input), &replacement))
					ops = append(ops, map[string]any{"path": "input", "mode": "set", "value": replacement})
				}
				common.SetContextKey(c, constant.ContextKeyChannelParamOverride, map[string]any{"operations": ops})
			}
			var req dto.OpenAIResponsesRequest
			require.NoError(t, common.Unmarshal([]byte(raw), &req))
			info := relaycommon.GenRelayInfoResponses(c, &req)
			_, body, closer, apiErr := PrepareResponsesRequest(c, info, &req)
			require.Nil(t, apiErr)
			t.Cleanup(func() { require.NoError(t, closer.Close()) })
			wire, err := io.ReadAll(body)
			require.NoError(t, err)
			if tc.passthrough || tc.globalPassthrough {
				assert.JSONEq(t, raw, string(wire))
				return
			}
			wantEffort := tc.effort
			if tc.override != "" {
				wantEffort = tc.override
				assert.Equal(t, "preserved", gjson.GetBytes(wire, "vendor_extension").String())
			}
			assert.Equal(t, wantEffort, gjson.GetBytes(wire, "reasoning.effort").String())
			if tc.wantReasoning {
				assert.Equal(t, "reasoning", gjson.GetBytes(wire, "input.0.type").String())
				assert.Equal(t, " ", gjson.GetBytes(wire, "input.0.content.0.text").String())
				assert.Equal(t, "function_call", gjson.GetBytes(wire, "input.1.type").String())
			} else {
				assert.JSONEq(t, input, gjson.GetBytes(wire, "input").Raw)
			}
		})
	}
}

func TestResponsesNonStreamingRequestCollectsUpstreamSSE(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	previousTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = previousTimeout })
	previousPassthrough, previousEncryption := settings.PassThroughRequestEnabled, settings.ResponsesReasoningEncryption
	settings.PassThroughRequestEnabled, settings.ResponsesReasoningEncryption = false, false
	t.Cleanup(func() {
		settings.PassThroughRequestEnabled, settings.ResponsesReasoningEncryption = previousPassthrough, previousEncryption
	})
	for _, protocol := range []string{"responses", "chat", "responses-json", "chat-json"} {
		t.Run(protocol, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(protocol, "-json") {
					w.Header().Set("Content-Type", "application/json")
					if protocol == "responses-json" {
						_, _ = io.WriteString(w, `{"id":"resp_fixture","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"answer","annotations":[]}]}],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`)
					} else {
						_, _ = io.WriteString(w, `{"id":"chatcmpl_fixture","model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`)
					}
					return
				}
				w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
				if protocol == "responses" {
					_, _ = io.WriteString(w, "event: response.created\ndata: {\"type\":\"response.created\",\"sequence_number\":0,\"response\":{\"id\":\"resp_fixture\",\"status\":\"in_progress\",\"output\":[]}}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"sequence_number\":1,\"response\":{\"id\":\"resp_fixture\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"answer\",\"annotations\":[]}]}],\"usage\":{\"input_tokens\":3,\"output_tokens\":2,\"total_tokens\":5}}}\n\n")
				} else {
					_, _ = io.WriteString(w, "data: {\"id\":\"chatcmpl_fixture\",\"model\":\"test-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"answer\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n\ndata: [DONE]\n\n")
				}
			}))
			defer server.Close()
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"test-model","input":"hello","stream":false,"store":false}`))
			c.Request.Header.Set("Content-Type", "application/json")
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "test-model")
			common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeAdvancedCustom)
			common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, server.URL)
			common.SetContextKey(c, constant.ContextKeyChannelKey, "fixture-key")
			route := dto.AdvancedCustomRoute{IncomingPath: "/v1/responses", UpstreamPath: "/v1/responses", Converter: relayconvert.ConverterNone}
			if strings.HasPrefix(protocol, "chat") {
				route.UpstreamPath, route.Converter = "/v1/chat/completions", relayconvert.ConverterOpenAIResponsesToOpenAIChat
			}
			common.SetContextKey(c, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{
				AdvancedCustom: &dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{route}},
			})
			request := &dto.OpenAIResponsesRequest{Model: "test-model", Input: []byte(`"hello"`), Stream: common.GetPointer(false), Store: []byte(`false`)}
			info := relaycommon.GenRelayInfoResponses(c, request)
			usage, apiErr := relayResponsesHTTP(c, info, request)
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			assert.Equal(t, 5, usage.TotalTokens)
			assert.False(t, info.IsStream)
			assert.Contains(t, recorder.Header().Get("Content-Type"), "application/json")
			var response map[string]any
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.Equal(t, "completed", response["status"])
			assert.Contains(t, recorder.Body.String(), "answer")
			assert.NotContains(t, recorder.Body.String(), "billing_usage")
		})
	}
}

func TestCollectGeminiSSEPreservesThoughtsToolsAndFinalUsage(t *testing.T) {
	body, err := collectGeminiSSE([]byte(strings.Join([]string{
		`data: {"candidates":[{"index":0,"content":{"role":"model","parts":[{"thought":true,"text":"Think "}]}}]}`,
		`data: {"candidates":[{"index":0,"content":{"parts":[{"thought":true,"text":"carefully"},{"text":"Hello "}]}}]}`,
		`data: {"candidates":[{"index":0,"content":{"parts":[{"text":"world"},{"functionCall":{"name":"lookup","args":{"q":"x"}}}]}}]}`,
		`data: {"candidates":[{"index":0,"content":{"parts":[]},"finishReason":"STOP"}],"modelVersion":"fixture","usageMetadata":{"promptTokenCount":37,"candidatesTokenCount":126,"thoughtsTokenCount":47,"totalTokenCount":210}}`,
	}, "\n\n")))
	require.NoError(t, err)
	var response map[string]any
	require.NoError(t, common.Unmarshal(body, &response))
	assert.Equal(t, "fixture", response["modelVersion"])
	output := response["candidates"].([]any)[0].(map[string]any)
	assert.Equal(t, "STOP", output["finishReason"])
	parts := output["content"].(map[string]any)["parts"].([]any)
	require.Len(t, parts, 3)
	assert.Equal(t, map[string]any{"text": "Think carefully", "thought": true}, parts[0])
	assert.Equal(t, map[string]any{"text": "Hello world"}, parts[1])
	assert.Contains(t, parts[2], "functionCall")
	assert.Equal(t, float64(47), response["usageMetadata"].(map[string]any)["thoughtsTokenCount"])
	_, err = collectGeminiSSE([]byte(`data: {"candidates":[{"index":0,"content":{"parts":[{"text":"unfinished"}]}}]}`))
	require.ErrorContains(t, err, "before completion")
}
