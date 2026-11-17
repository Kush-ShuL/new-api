package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetPricingEndpointTestTables(t *testing.T) {
	t.Helper()
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}, &Model{}, &Vendor{}))
	for _, table := range []string{"abilities", "channels", "models", "vendors"} {
		require.NoError(t, DB.Exec("DELETE FROM "+table).Error)
	}
	InitChannelCache()
	InvalidatePricingCache()
	t.Cleanup(func() {
		for _, table := range []string{"abilities", "channels", "models", "vendors"} {
			require.NoError(t, DB.Exec("DELETE FROM "+table).Error)
		}
		InitChannelCache()
		InvalidatePricingCache()
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})
}

func insertPricingEndpointChannel(t *testing.T, channelID int, channelType int, settings dto.ChannelOtherSettings) {
	t.Helper()
	channel := &Channel{
		Id:     channelID,
		Type:   channelType,
		Key:    fmt.Sprintf("key-%d", channelID),
		Status: common.ChannelStatusEnabled,
		Name:   fmt.Sprintf("channel-%d", channelID),
	}
	if settings.AdvancedCustom != nil {
		channel.SetOtherSettings(settings)
	}
	require.NoError(t, DB.Create(channel).Error)
}

func insertPricingEndpointAbility(t *testing.T, channelID int, modelName string) {
	t.Helper()
	require.NoError(t, DB.Create(&Ability{
		Group:     "default",
		Model:     modelName,
		ChannelId: channelID,
		Enabled:   true,
	}).Error)
}

func pricingEndpointAdvancedCustomConfig(routes ...dto.AdvancedCustomRoute) dto.ChannelOtherSettings {
	return dto.ChannelOtherSettings{
		AdvancedCustom: &dto.AdvancedCustomConfig{
			Routes: routes,
		},
	}
}

func pricingEndpointTypesByModel(t *testing.T) map[string][]constant.EndpointType {
	t.Helper()
	InitChannelCache()
	return pricingEndpointTypesFromPricing(GetPricing())
}

func pricingEndpointTypesFromPricing(pricings []Pricing) map[string][]constant.EndpointType {
	byModel := make(map[string][]constant.EndpointType)
	for _, pricing := range pricings {
		byModel[pricing.ModelName] = pricing.SupportedEndpointTypes
	}
	return byModel
}

func TestPricingAdvancedCustomUsesConfiguredEndpointTypes(t *testing.T) {
	resetPricingEndpointTestTables(t)

	insertPricingEndpointChannel(t, 101, constant.ChannelTypeAdvancedCustom, pricingEndpointAdvancedCustomConfig(
		dto.AdvancedCustomRoute{
			IncomingPath: "/v1/chat/completions",
			UpstreamPath: "/v1/chat/completions",
		},
		dto.AdvancedCustomRoute{
			IncomingPath: "/v1/responses",
			UpstreamPath: "/v1beta/models/{model}:generateContent",
			Converter:    "openai_responses_to_gemini_generate_content",
			Models:       []string{"re:^gemini-"},
		},
	))
	insertPricingEndpointAbility(t, 101, "gemini-2.5-flash")
	insertPricingEndpointAbility(t, 101, "gpt-4o")

	byModel := pricingEndpointTypesByModel(t)

	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
	}, byModel["gemini-2.5-flash"])
	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAI,
	}, byModel["gpt-4o"])
}

func TestPricingModelMetadataEndpointsReplaceChannelInference(t *testing.T) {
	resetPricingEndpointTestTables(t)

	insertPricingEndpointChannel(t, 103, constant.ChannelTypeAdvancedCustom, pricingEndpointAdvancedCustomConfig(
		dto.AdvancedCustomRoute{
			IncomingPath: "/v1/responses",
			UpstreamPath: "/v1beta/models/{model}:generateContent",
			Converter:    "openai_responses_to_gemini_generate_content",
			Models:       []string{"re:^gemini-"},
		},
	))
	insertPricingEndpointAbility(t, 103, "gemini-2.5-flash")
	require.NoError(t, DB.Create(&Model{
		ModelName: "gemini-2.5-flash",
		Endpoints: `{
			"openai": "/v1/chat/completions"
		}`,
		Status:   1,
		NameRule: NameRuleExact,
	}).Error)

	byModel := pricingEndpointTypesByModel(t)

	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAI,
	}, byModel["gemini-2.5-flash"])
}

func TestModelEndpointPolicy(t *testing.T) {
	resetPricingEndpointTestTables(t)
	insertPricingEndpointChannel(t, 105, constant.ChannelTypeOpenAI, dto.ChannelOtherSettings{})
	for _, name := range []string{"messages-only", "responses-only", "both", "empty", "array", "family-chat", "family-response", "custom", "deny-messages", "deny-none", "allow-none"} {
		insertPricingEndpointAbility(t, 105, name)
	}
	require.NoError(t, DB.Create(&[]Model{
		{ModelName: "messages-only", Endpoints: `{"anthropic":{"path":"/v1/messages","method":"POST"}}`, Status: 1},
		{ModelName: "responses-only", Endpoints: `{"openai-response":"/v1/responses"}`, Status: 1},
		{ModelName: "both", Endpoints: `["anthropic","openai-response"]`, Status: 1},
		{ModelName: "empty", Endpoints: `{}`, Status: 1},
		{ModelName: "array", Endpoints: `["openai-response"]`, Status: 1},
		{ModelName: "family-", NameRule: NameRulePrefix, Endpoints: `["anthropic"]`, Status: 1},
		{ModelName: "family-response", Endpoints: `["openai-response"]`, Status: 1},
		{ModelName: "custom", Endpoints: `{"custom":{"path":"/custom/infer","method":"PUT"}}`, Status: 1},
		{ModelName: "deny-messages", Endpoints: `{"mode":"blacklist","endpoints":["anthropic"]}`, Status: 1},
		{ModelName: "deny-none", Endpoints: `{"mode":"blacklist","endpoints":[]}`, Status: 1},
		{ModelName: "allow-none", Endpoints: `{"mode":"whitelist","endpoints":[]}`, Status: 1},
	}).Error)

	GetPricing()
	for _, tc := range []struct {
		model   string
		method  string
		path    string
		allowed bool
	}{
		{"messages-only", "POST", "/v1/messages", true},
		{"messages-only", "POST", "/v1/chat/completions", false},
		{"messages-only", "POST", "/v1/responses", false},
		{"messages-only", "GET", "/v1/responses", false},
		{"responses-only", "POST", "/v1/responses", true},
		{"responses-only", "GET", "/v1/responses", true},
		{"responses-only", "POST", "/v1/responses/compact", false},
		{"responses-only", "POST", "/v1/messages", false},
		{"responses-only", "POST", "/v1/chat/completions", false},
		{"responses-only", "POST", "/v1/completions", false},
		{"responses-only", "POST", "/pg/chat/completions", false},
		{"responses-only@thinking:on", "POST", "/v1/chat/completions", false},
		{"responses-only", "POST", "/v1beta/models/responses-only:generateContent", false},
		{"both", "POST", "/v1/messages", true},
		{"both", "POST", "/v1/responses", true},
		{"both", "POST", "/v1/chat/completions", false},
		{"empty", "POST", "/v1/chat/completions", true},
		{"empty", "POST", "/v1/messages", true},
		{"array", "POST", "/v1/responses", true},
		{"array", "POST", "/v1/chat/completions", false},
		{"family-chat", "POST", "/v1/messages", true},
		{"family-chat", "POST", "/v1/responses", false},
		{"family-response", "POST", "/v1/messages", false},
		{"family-response", "POST", "/v1/responses", true},
		{"custom", "PUT", "/custom/infer", true},
		{"custom", "POST", "/custom/infer", false},
		{"deny-messages", "POST", "/v1/messages", false},
		{"deny-messages", "POST", "/v1/messages/count_tokens", false},
		{"deny-messages", "GET", "/v1/responses", true},
		{"deny-messages", "POST", "/v1/chat/completions", true},
		{"deny-none", "POST", "/v1/messages", true},
		{"allow-none", "POST", "/v1/responses", false},
		{"unconfigured", "POST", "/v1/chat/completions", true},
	} {
		t.Run(tc.model+tc.method+tc.path, func(t *testing.T) {
			assert.Equal(t, tc.allowed, ModelAllowsEndpoint(tc.model, tc.method, tc.path))
		})
	}

	require.NoError(t, DB.Model(&Model{}).Where("model_name = ?", "responses-only").Update("endpoints", `["anthropic"]`).Error)
	RefreshPricing()
	assert.False(t, ModelAllowsEndpoint("responses-only", "POST", "/v1/responses"))
	assert.True(t, ModelAllowsEndpoint("responses-only", "POST", "/v1/messages"))
}

func TestParseModelEndpointPolicy(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		mode    string
		names   []string
		invalid bool
	}{
		{raw: ""},
		{raw: `{}`},
		{raw: `["openai-response"]`, mode: "whitelist", names: []string{"openai-response"}},
		{raw: `{"mode":"blacklist","endpoints":{"anthropic":"/v1/messages"}}`, mode: "blacklist", names: []string{"anthropic"}},
		{raw: `{"mode":"whitelist","endpoints":[]}`, mode: "whitelist", names: []string{}},
		{raw: `{"mode":"other","endpoints":[]}`, invalid: true},
		{raw: `{"mode":"blacklist"}`, invalid: true},
		{raw: `{"mode":"whitelist","endpoints":{"x":{"path":"bad"}}}`, invalid: true},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			policy, err := ParseModelEndpointPolicy(tc.raw)
			if tc.invalid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.mode == "" {
				assert.Nil(t, policy)
				return
			}
			require.NotNil(t, policy)
			assert.Equal(t, tc.mode, policy.Mode)
			assert.Equal(t, tc.names, policy.Endpoints)
		})
	}
}

func TestPricingModelMetadataEndpointsCanProvideEndpointWithoutChannelInference(t *testing.T) {
	resetPricingEndpointTestTables(t)

	insertPricingEndpointChannel(t, 104, constant.ChannelTypeAdvancedCustom, pricingEndpointAdvancedCustomConfig(
		dto.AdvancedCustomRoute{
			IncomingPath: "/v1/responses",
			UpstreamPath: "/v1beta/models/{model}:generateContent",
			Converter:    "openai_responses_to_gemini_generate_content",
			Models:       []string{"re:^gemini-"},
		},
	))
	insertPricingEndpointAbility(t, 104, "metadata-only-model")
	require.NoError(t, DB.Create(&Model{
		ModelName: "metadata-only-model",
		Endpoints: `{
			"openai": "/v1/chat/completions"
		}`,
		Status:   1,
		NameRule: NameRuleExact,
	}).Error)

	byModel := pricingEndpointTypesByModel(t)

	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAI}, byModel["metadata-only-model"])
}

func TestPricingAdvancedCustomMissingConfigFallsBackToChannelType(t *testing.T) {
	resetPricingEndpointTestTables(t)

	insertPricingEndpointChannel(t, 102, constant.ChannelTypeAdvancedCustom, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, 102, "gpt-4o")

	byModel := pricingEndpointTypesByModel(t)

	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAI}, byModel["gpt-4o"])
}

func TestPricingKeepsNativeProtocolFirstAndIncludesConversions(t *testing.T) {
	resetPricingEndpointTestTables(t)

	insertPricingEndpointChannel(t, 201, constant.ChannelTypeOpenAI, dto.ChannelOtherSettings{})
	insertPricingEndpointChannel(t, 202, constant.ChannelTypeGemini, dto.ChannelOtherSettings{})
	insertPricingEndpointChannel(t, 203, constant.ChannelTypeAnthropic, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, 201, "gpt-4o")
	insertPricingEndpointAbility(t, 202, "gemini-2.5-flash")
	insertPricingEndpointAbility(t, 203, "claude-3-5-sonnet")

	byModel := pricingEndpointTypesByModel(t)

	all := []constant.EndpointType{constant.EndpointTypeOpenAI, constant.EndpointTypeAnthropic, constant.EndpointTypeGemini, constant.EndpointTypeOpenAIResponse}
	for name, preferred := range map[string]constant.EndpointType{"gpt-4o": constant.EndpointTypeOpenAI, "gemini-2.5-flash": constant.EndpointTypeGemini, "claude-3-5-sonnet": constant.EndpointTypeAnthropic} {
		assert.ElementsMatch(t, all, byModel[name])
		assert.Equal(t, preferred, byModel[name][0])
	}
}

func TestInitChannelCacheInvalidatesPricingCache(t *testing.T) {
	resetPricingEndpointTestTables(t)

	insertPricingEndpointChannel(t, 301, constant.ChannelTypeAdvancedCustom, pricingEndpointAdvancedCustomConfig(
		dto.AdvancedCustomRoute{
			IncomingPath: "/v1/chat/completions",
			UpstreamPath: "/v1/chat/completions",
		},
	))
	insertPricingEndpointAbility(t, 301, "gemini-3.5-flash")
	InitChannelCache()

	initial := pricingEndpointTypesByModel(t)
	require.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAI}, initial["gemini-3.5-flash"])

	var channel Channel
	require.NoError(t, DB.First(&channel, "id = ?", 301).Error)
	channel.SetOtherSettings(pricingEndpointAdvancedCustomConfig(
		dto.AdvancedCustomRoute{
			IncomingPath: "/v1/chat/completions",
			UpstreamPath: "/v1/chat/completions",
		},
		dto.AdvancedCustomRoute{
			IncomingPath: "/v1/responses",
			UpstreamPath: "/v1beta/models/{model}:generateContent",
			Converter:    "openai_responses_to_gemini_generate_content",
			Models:       []string{"re:^gemini-"},
		},
	))
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", 301).Update("settings", channel.OtherSettings).Error)
	InitChannelCache()

	updated := pricingEndpointTypesByModel(t)
	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
	}, updated["gemini-3.5-flash"])
}

func TestInitChannelCacheInvalidatesStartupPricingBuiltBeforeChannelCache(t *testing.T) {
	resetPricingEndpointTestTables(t)

	insertPricingEndpointChannel(t, 302, constant.ChannelTypeAdvancedCustom, pricingEndpointAdvancedCustomConfig(
		dto.AdvancedCustomRoute{
			IncomingPath: "/v1/chat/completions",
			UpstreamPath: "/v1/chat/completions",
		},
		dto.AdvancedCustomRoute{
			IncomingPath: "/v1/responses",
			UpstreamPath: "/v1beta/models/{model}:generateContent",
			Converter:    "openai_responses_to_gemini_generate_content",
			Models:       []string{"re:^gemini-"},
		},
	))
	insertPricingEndpointAbility(t, 302, "gemini-3.5-flash")

	staleByModel := pricingEndpointTypesFromPricing(GetPricing())
	require.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAI}, staleByModel["gemini-3.5-flash"])

	InitChannelCache()

	rebuiltByModel := pricingEndpointTypesFromPricing(GetPricing())
	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
	}, rebuiltByModel["gemini-3.5-flash"])
}

func TestCacheUpdateChannelSyncsAdvancedCustomConfig(t *testing.T) {
	resetPricingEndpointTestTables(t)

	channel := &Channel{
		Id:     401,
		Type:   constant.ChannelTypeAdvancedCustom,
		Key:    "key-401",
		Status: common.ChannelStatusEnabled,
		Name:   "channel-401",
	}
	channel.SetOtherSettings(pricingEndpointAdvancedCustomConfig(dto.AdvancedCustomRoute{
		IncomingPath: "/v1/responses",
		UpstreamPath: "/v1beta/models/{model}:generateContent",
		Converter:    "openai_responses_to_gemini_generate_content",
	}))
	CacheUpdateChannel(channel)

	require.NotNil(t, channel2advancedCustomConfig[401])
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIResponse}, channel2advancedCustomConfig[401].SupportedEndpointTypesForModel("gemini-3.5-flash"))

	channel.SetOtherSettings(pricingEndpointAdvancedCustomConfig(dto.AdvancedCustomRoute{
		IncomingPath: "/v1/chat/completions",
		UpstreamPath: "/v1/chat/completions",
	}))
	CacheUpdateChannel(channel)

	require.NotNil(t, channel2advancedCustomConfig[401])
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAI}, channel2advancedCustomConfig[401].SupportedEndpointTypesForModel("gemini-3.5-flash"))

	channel.Type = constant.ChannelTypeOpenAI
	CacheUpdateChannel(channel)

	assert.Nil(t, channel2advancedCustomConfig[401])
}
