package service

import (
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupResponsesOutputTest(t *testing.T, encrypt bool) *ResponsesOutputProcessor {
	t.Helper()
	previousDB := model.DB
	settings := model_setting.GetGlobalSettings()
	previousEncryption := settings.ResponsesReasoningEncryption
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.DB = db
	settings.ResponsesReasoningEncryption = encrypt
	t.Cleanup(func() {
		model.DB = previousDB
		settings.ResponsesReasoningEncryption = previousEncryption
		require.NoError(t, sqlDB.Close())
	})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	common.SetContextKey(c, constant.ContextKeyUserId, 1)
	return &ResponsesOutputProcessor{
		Context: c,
		Info:    &relaycommon.RelayInfo{FinalRequestRelayFormat: types.RelayFormatOpenAI},
		Request: &dto.OpenAIResponsesRequest{Input: common.RawMessage(`"hello"`), Store: common.RawMessage(`false`)},
	}
}

func TestResponsesConvertedStreamClosesIndependentSummariesBeforeMessage(t *testing.T) {
	p := setupResponsesOutputTest(t, true)
	state := relayconvert.NewChatToResponsesStreamState("chatcmpl_test", "deepseek-flash")
	state.EmitSequenceNumber = true
	var wire []map[string]any
	for _, delta := range []dto.ChatCompletionsStreamResponseChoiceDelta{
		{ReasoningContent: common.GetPointer("**First ")},
		{ReasoningContent: common.GetPointer("heading**\nprivate body one\n")},
		{ReasoningContent: common.GetPointer("## Second heading\nprivate body two")},
		{Content: common.GetPointer("answer")},
	} {
		events, err := relayconvert.ChatCompletionsStreamChunkToResponsesEvents(&dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{{Delta: delta}},
		}, state)
		require.NoError(t, err)
		for _, event := range events {
			body, err := common.Marshal(event.Payload)
			require.NoError(t, err)
			encoded, err := p.ProcessEvent(string(body))
			require.NoError(t, err)
			for _, value := range encoded {
				var parsed map[string]any
				require.NoError(t, common.UnmarshalJsonStr(value, &parsed))
				wire = append(wire, parsed)
			}
		}
	}
	for _, event := range relayconvert.FinalizeChatCompletionsStreamToResponses(state) {
		body, err := common.Marshal(event.Payload)
		require.NoError(t, err)
		encoded, err := p.ProcessEvent(string(body))
		require.NoError(t, err)
		for _, value := range encoded {
			var parsed map[string]any
			require.NoError(t, common.UnmarshalJsonStr(value, &parsed))
			wire = append(wire, parsed)
		}
	}
	var eventTypes []string
	var doneItems []any
	for i, event := range wire {
		assert.Equal(t, float64(i), event["sequence_number"])
		eventTypes = append(eventTypes, event["type"].(string))
		if event["type"] == "response.output_text.delta" || event["type"] == "response.output_text.done" {
			require.IsType(t, []any{}, event["logprobs"])
		}
		if event["type"] == "response.output_item.done" {
			assert.Equal(t, float64(len(doneItems)), event["output_index"])
			doneItems = append(doneItems, event["item"])
		}
		if item, ok := event["item"].(map[string]any); ok && item["type"] == "message" {
			assert.Equal(t, "final_answer", item["phase"])
		}
	}
	assert.Equal(t, []string{
		"response.created", "response.in_progress", "response.output_item.added",
		"response.reasoning_summary_part.added", "response.reasoning_summary_text.delta",
		"response.reasoning_summary_text.done", "response.reasoning_summary_part.done", "response.output_item.done",
		"response.output_item.added", "response.reasoning_summary_part.added", "response.reasoning_summary_text.delta",
		"response.reasoning_summary_text.done", "response.reasoning_summary_part.done", "response.output_item.done",
		"response.output_item.added", "response.content_part.added", "response.output_text.delta",
		"response.output_text.done", "response.content_part.done", "response.output_item.done", "response.completed",
	}, eventTypes)
	require.Len(t, doneItems, 3)
	for i, title := range []string{"**First heading**", "**Second heading**"} {
		item := doneItems[i].(map[string]any)
		assert.Equal(t, []any{map[string]any{"type": "summary_text", "text": title}}, item["summary"])
		plaintext, err := OpenResponsesReasoning(1, item["encrypted_content"].(string))
		require.NoError(t, err)
		assert.Contains(t, plaintext, []string{"private body one", "private body two"}[i])
	}
	final := wire[len(wire)-1]["response"].(map[string]any)
	assert.Equal(t, "resp_test", final["id"])
	assert.Equal(t, doneItems, final["output"])
	body, err := common.Marshal(wire)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "private body")
}

func TestResponsesReasoningCiphertextOwnership(t *testing.T) {
	p := setupResponsesOutputTest(t, true)
	sealed, err := SealResponsesReasoning(1, "private reasoning")
	require.NoError(t, err)
	payload, marked := strings.CutPrefix(sealed, "nv1.")
	require.True(t, marked)
	_, err = base64.RawURLEncoding.DecodeString(payload)
	require.NoError(t, err)
	_, err = OpenResponsesReasoning(2, sealed)
	require.Error(t, err)
	_, err = OpenResponsesReasoning(1, sealed+"x")
	require.Error(t, err)

	p.Request.Input, err = common.Marshal([]map[string]any{
		{"type": "reasoning", "encrypted_content": sealed, "summary": []any{}},
	})
	require.NoError(t, err)
	require.NoError(t, PrepareStatefulResponsesRequest(p.Context, p.Request))
	var items []map[string]any
	require.NoError(t, common.Unmarshal(p.Request.Input, &items))
	assert.NotContains(t, items[0], "encrypted_content")
	assert.Equal(t, []any{map[string]any{"type": "summary_text", "text": "private reasoning"}}, items[0]["summary"])
	foreign := base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("foreign", 20)))
	for _, upstream := range []string{foreign, "native-opaque-ciphertext", payload} {
		p.Request.Input, err = common.Marshal([]map[string]any{{"type": "reasoning", "encrypted_content": upstream}})
		require.NoError(t, err)
		require.NoError(t, PrepareStatefulResponsesRequest(p.Context, p.Request))
		items = nil
		require.NoError(t, common.Unmarshal(p.Request.Input, &items))
		assert.Equal(t, upstream, items[0]["encrypted_content"])
		assert.NotContains(t, items[0], "summary")
	}
	for _, invalid := range []string{"nv1.", "nv1.invalid!", sealed + "x"} {
		p.Request.Input, err = common.Marshal([]map[string]any{{"type": "reasoning", "encrypted_content": invalid}})
		require.NoError(t, err)
		require.Error(t, PrepareStatefulResponsesRequest(p.Context, p.Request))
	}

	legacy := "newapi:chacha20:v1:" + sealed
	_, err = OpenResponsesReasoning(1, legacy)
	require.Error(t, err)
}

func TestResponsesOutputMessagePhase(t *testing.T) {
	p := setupResponsesOutputTest(t, true)
	body, err := p.ProcessResponse([]byte(`{"id":"resp_phase","output":[{"type":"message","phase":"commentary"},{"type":"message","role":"assistant","content":[]}]}`))
	require.NoError(t, err)
	var response struct {
		Output []dto.ResponsesOutput `json:"output"`
	}
	require.NoError(t, common.Unmarshal(body, &response))
	require.Len(t, response.Output, 2)
	assert.Equal(t, "commentary", response.Output[0].Phase)
	assert.Equal(t, "final_answer", response.Output[1].Phase)
	encoded, err := common.Marshal(response.Output[0])
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"phase":"commentary"`)
}

func TestResponsesGeneratedSummariesFollowDetail(t *testing.T) {
	for _, tc := range []struct {
		level string
		count int
	}{{"concise", 2}, {"auto", 3}, {"detailed", 7}} {
		t.Run(tc.level, func(t *testing.T) {
			p := setupResponsesOutputTest(t, true)
			p.Request.Reasoning = &dto.Reasoning{Summary: tc.level}
			var excerpts []string
			p.SummarizeReasoning = func(text string) (string, error) {
				excerpts = append(excerpts, text)
				return fmt.Sprintf("Progress %d", len(excerpts)), nil
			}
			_, err := p.ProcessEvent(`{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_generated","type":"reasoning","summary":[]}}`)
			require.NoError(t, err)
			var source strings.Builder
			var summaries []string
			for i := range 7 {
				text := fmt.Sprintf("paragraph %d without a heading", i)
				if i < 6 {
					text += "\r\n\r\n"
				}
				source.WriteString(text)
				body, err := common.Marshal(map[string]any{"type": "response.reasoning_summary_text.delta", "output_index": 0, "delta": text})
				require.NoError(t, err)
				events, err := p.ProcessEvent(string(body))
				require.NoError(t, err)
				for _, event := range events {
					if strings.Contains(event, `"type":"response.reasoning_summary_text.done"`) {
						summaries = append(summaries, event)
					}
				}
			}
			// Completed paragraph batches are visible before the upstream ends.
			assert.Len(t, summaries, tc.count-1)
			_, err = p.ProcessEvent(`{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_generated","type":"reasoning"}}`)
			require.NoError(t, err)
			require.Len(t, p.output, tc.count)
			var decrypted strings.Builder
			for i, raw := range p.output {
				item := raw.(map[string]any)
				assert.Equal(t, []any{map[string]any{"type": "summary_text", "text": fmt.Sprintf("Progress %d", i+1)}}, item["summary"])
				text, err := OpenResponsesReasoning(1, item["encrypted_content"].(string))
				require.NoError(t, err)
				decrypted.WriteString(text)
			}
			assert.Equal(t, source.String(), decrypted.String())
		})
	}
}

func TestResponsesOutputJSONAndStreamKeepReasoningPrivate(t *testing.T) {
	for _, encrypt := range []bool{true, false} {
		t.Run(map[bool]string{true: "encrypted", false: "plaintext"}[encrypt], func(t *testing.T) {
			p := setupResponsesOutputTest(t, encrypt)
			p.Request.Temperature = common.GetPointer(0.0)
			body, err := p.ProcessResponse([]byte(`{"id":"chatcmpl_json","output":[{"id":"rs_json","type":"reasoning","summary":[{"type":"summary_text","text":"plain private reasoning"}]}],"usage":{"input_tokens":37,"output_tokens":175,"total_tokens":212,"prompt_tokens":37,"completion_tokens":175,"billing_usage":{"source":"oai_chat"},"completion_tokens_details":{"reasoning_tokens":74}}}`))
			require.NoError(t, err)
			var response map[string]any
			require.NoError(t, common.Unmarshal(body, &response))
			usage := response["usage"].(map[string]any)
			assert.Equal(t, map[string]any{
				"input_tokens": float64(37), "output_tokens": float64(175), "total_tokens": float64(212),
				"input_tokens_details":  map[string]any{"cached_tokens": float64(0)},
				"output_tokens_details": map[string]any{"reasoning_tokens": float64(74)},
			}, usage)
			assert.Equal(t, float64(0), response["temperature"])
			assert.Equal(t, float64(1), response["top_p"])
			item := response["output"].([]any)[0].(map[string]any)
			if encrypt {
				assert.Empty(t, item["summary"])
				assert.NotContains(t, string(body), "plain private reasoning")
				plaintext, err := OpenResponsesReasoning(1, item["encrypted_content"].(string))
				require.NoError(t, err)
				assert.Equal(t, "plain private reasoning", plaintext)
			} else {
				assert.NotContains(t, item, "encrypted_content")
				assert.Contains(t, string(body), "plain private reasoning")
			}
		})
	}
}

func TestResponsesNativeSummaryLifecycleIsPreserved(t *testing.T) {
	p := setupResponsesOutputTest(t, true)
	p.Info.FinalRequestRelayFormat = types.RelayFormatOpenAIResponses
	for _, raw := range []string{
		`{"type":"response.output_item.added","sequence_number":4,"output_index":0,"item":{"id":"rs_native","type":"reasoning","summary":[]}}`,
		`{"type":"response.reasoning_summary_text.delta","sequence_number":5,"output_index":0,"item_id":"rs_native","summary_index":0,"delta":"native summary"}`,
		`{"type":"response.output_item.done","sequence_number":6,"output_index":0,"item":{"id":"rs_native","type":"reasoning","summary":[{"type":"summary_text","text":"native summary"}],"encrypted_content":"native-opaque-ciphertext"}}`,
		`{"type":"response.output_item.added","sequence_number":7,"output_index":1,"item":{"id":"msg_native","type":"message","phase":"commentary","content":[]}}`,
	} {
		result, err := p.ProcessEvent(raw)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.JSONEq(t, raw, result[0])
	}
}

func TestResponsesLegacyStreamClosesReasoningBeforeBody(t *testing.T) {
	p := setupResponsesOutputTest(t, true)
	p.Info.FinalRequestRelayFormat = types.RelayFormatOpenAIResponses
	var kinds []string
	for _, raw := range []string{
		`{"type":"response.created","response":{"id":"chatcmpl_legacy","output":[]}}`,
		`{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_legacy","type":"reasoning","summary":[]}}`,
		`{"type":"response.reasoning_summary_text.delta","output_index":0,"item_id":"rs_legacy","delta":"**Title**\nprivate reasoning"}`,
		`{"type":"response.output_item.added","output_index":1,"item":{"id":"msg_legacy","type":"message","content":[]}}`,
		`{"type":"response.output_text.delta","output_index":1,"item_id":"msg_legacy","content_index":0,"delta":"answer"}`,
		`{"type":"response.output_text.done","output_index":1,"item_id":"msg_legacy","content_index":0,"text":"answer"}`,
		`{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_legacy","type":"reasoning","summary":[{"type":"summary_text","text":"**Title**\nprivate reasoning"}]}}`,
	} {
		result, err := p.ProcessEvent(raw)
		require.NoError(t, err)
		for _, value := range result {
			var event map[string]any
			require.NoError(t, common.UnmarshalJsonStr(value, &event))
			kinds = append(kinds, event["type"].(string))
			assert.NotContains(t, value, "private reasoning")
		}
	}
	assert.Equal(t, 1, strings.Count(strings.Join(kinds, ","), "response.output_item.done"))
	assert.Less(t, strings.Index(strings.Join(kinds, ","), "response.output_item.done"), strings.Index(strings.Join(kinds, ","), "response.output_text.delta"))
	assert.Contains(t, kinds, "response.content_part.added")
	assert.Contains(t, kinds, "response.content_part.done")
}
