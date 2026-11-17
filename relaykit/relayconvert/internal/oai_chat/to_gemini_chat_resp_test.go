package oaichat

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"
	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseOpenAI2GeminiMapsTextToolFinishReasonAndUsage(t *testing.T) {
	msg := dto.Message{
		Role:    "assistant",
		Content: "hello",
	}
	msg.SetToolCalls([]dto.ToolCallRequest{
		{
			ID:   "call_1",
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      "lookup",
				Arguments: `{"q":"x"}`,
			},
		},
	})

	resp := ResponseOpenAI2Gemini(&dto.OpenAITextResponse{
		Model: "gpt-test",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index:        2,
				Message:      msg,
				FinishReason: "length",
			},
		},
		Usage: dto.Usage{
			PromptTokens:     11,
			CompletionTokens: 5,
			TotalTokens:      16,
		},
	}, nil)

	assert.Equal(t, 11, resp.UsageMetadata.PromptTokenCount)
	assert.Equal(t, 5, resp.UsageMetadata.CandidatesTokenCount)
	assert.Equal(t, 16, resp.UsageMetadata.TotalTokenCount)
	require.NotNil(t, resp.UsageMetadata.BillingUsage)
	require.NotNil(t, resp.UsageMetadata.BillingUsage.OpenAIUsage)
	assert.Equal(t, dto.BillingUsageSourceOAIChat, resp.UsageMetadata.BillingUsage.Source)
	assert.Equal(t, dto.BillingUsageSemanticOpenAI, resp.UsageMetadata.BillingUsage.Semantic)
	assert.Equal(t, 11, resp.UsageMetadata.BillingUsage.OpenAIUsage.PromptTokens)
	assert.Equal(t, 5, resp.UsageMetadata.BillingUsage.OpenAIUsage.CompletionTokens)
	assert.Equal(t, 16, resp.UsageMetadata.BillingUsage.OpenAIUsage.TotalTokens)
	assert.Nil(t, resp.UsageMetadata.BillingUsage.OpenAIUsage.BillingUsage)
	require.Len(t, resp.Candidates, 1)
	assert.Equal(t, int64(2), resp.Candidates[0].Index)
	require.NotNil(t, resp.Candidates[0].FinishReason)
	assert.Equal(t, "MAX_TOKENS", *resp.Candidates[0].FinishReason)
	require.Len(t, resp.Candidates[0].Content.Parts, 2)
	assert.Equal(t, "hello", resp.Candidates[0].Content.Parts[0].Text)
	require.NotNil(t, resp.Candidates[0].Content.Parts[1].FunctionCall)
	assert.Equal(t, "lookup", resp.Candidates[0].Content.Parts[1].FunctionCall.FunctionName)
	assert.Equal(t, map[string]any{"q": "x"}, resp.Candidates[0].Content.Parts[1].FunctionCall.Arguments)
}

func TestGeminiConversionMapsReasoningAndOmitsUnreportedStreamUsage(t *testing.T) {
	usage := &dto.Usage{PromptTokens: 37, CompletionTokens: 173, TotalTokens: 210}
	usage.CompletionTokenDetails.ReasoningTokens = 47
	usage.PromptTokensDetails.CachedTokens = 12
	state := NewChatToGeminiStreamState()
	chunks, err := state.ConvertChunk(&dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{{Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: geminiRespPtr("hello")}}},
	}, &convmeta.Values{})
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	body, err := kitutil.Marshal(chunks[0])
	require.NoError(t, err)
	assert.NotContains(t, string(body), "usageMetadata")
	stop := "stop"
	final, err := state.ConvertChunk(&dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{{FinishReason: &stop}}, Usage: usage,
	}, nil)
	require.NoError(t, err)
	require.Len(t, final, 1)
	for _, response := range []*dto.GeminiChatResponse{
		ResponseOpenAI2Gemini(&dto.OpenAITextResponse{Usage: *usage}, nil),
		StreamResponseOpenAI2Gemini(&dto.ChatCompletionsStreamResponse{Usage: usage, Choices: []dto.ChatCompletionsStreamResponseChoice{{FinishReason: &stop}}}, nil),
		final[0],
	} {
		metadata := response.UsageMetadata
		assert.Equal(t, 37, metadata.PromptTokenCount)
		assert.Equal(t, 126, metadata.CandidatesTokenCount)
		assert.Equal(t, 47, metadata.ThoughtsTokenCount)
		assert.Equal(t, 210, metadata.TotalTokenCount)
		assert.Equal(t, 12, metadata.CachedContentTokenCount)
		require.NotNil(t, metadata.BillingUsage)
		assert.Equal(t, 173, metadata.BillingUsage.OpenAIUsage.CompletionTokens)
		body, err := kitutil.Marshal(response)
		require.NoError(t, err)
		assert.NotContains(t, string(body), "billing_usage")
		assert.NotContains(t, string(body), `"promptTokensDetails":null`)
	}
	// Multi-hop conversions may retain details only in the accounting snapshot.
	carried := *usage
	carried.BillingUsage = dto.NewOpenAIChatBillingUsage(usage)
	carried.CompletionTokenDetails = dto.OutputTokenDetails{}
	response := ResponseOpenAI2Gemini(&dto.OpenAITextResponse{Usage: carried}, nil)
	assert.Equal(t, 47, response.UsageMetadata.ThoughtsTokenCount)
	assert.Equal(t, 126, response.UsageMetadata.CandidatesTokenCount)
}

func TestStreamResponseOpenAI2GeminiMapsToolCallFinishReasonAndUsage(t *testing.T) {
	resp := StreamResponseOpenAI2Gemini(&dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index:        1,
				FinishReason: geminiRespPtr("tool_calls"),
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ToolCalls: []dto.ToolCallResponse{
						{
							Type: "function",
							Function: dto.FunctionResponse{
								Name:      "lookup",
								Arguments: `{"q":"x"}`,
							},
						},
					},
				},
			},
		},
		Usage: &dto.Usage{
			PromptTokens:     13,
			CompletionTokens: 8,
			TotalTokens:      21,
		},
	}, &convmeta.Values{})

	require.NotNil(t, resp)
	assert.Equal(t, 13, resp.UsageMetadata.PromptTokenCount)
	assert.Equal(t, 8, resp.UsageMetadata.CandidatesTokenCount)
	assert.Equal(t, 21, resp.UsageMetadata.TotalTokenCount)
	require.NotNil(t, resp.UsageMetadata.BillingUsage)
	require.NotNil(t, resp.UsageMetadata.BillingUsage.OpenAIUsage)
	assert.Equal(t, 13, resp.UsageMetadata.BillingUsage.OpenAIUsage.PromptTokens)
	assert.Equal(t, 8, resp.UsageMetadata.BillingUsage.OpenAIUsage.CompletionTokens)
	require.Len(t, resp.Candidates, 1)
	assert.Equal(t, int64(1), resp.Candidates[0].Index)
	require.NotNil(t, resp.Candidates[0].FinishReason)
	assert.Equal(t, "STOP", *resp.Candidates[0].FinishReason)
	require.Len(t, resp.Candidates[0].Content.Parts, 1)
	require.NotNil(t, resp.Candidates[0].Content.Parts[0].FunctionCall)
	assert.Equal(t, "lookup", resp.Candidates[0].Content.Parts[0].FunctionCall.FunctionName)
	assert.Equal(t, map[string]any{"q": "x"}, resp.Candidates[0].Content.Parts[0].FunctionCall.Arguments)
}

func geminiRespPtr[T any](value T) *T {
	return &value
}
