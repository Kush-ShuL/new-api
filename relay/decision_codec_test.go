package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecisionProtocolConversion(t *testing.T) {
	for _, tc := range []struct {
		name, request, upstreamReply, wantReply string
		toSystemOne, ollama                     bool
	}{
		{
			name: "Ollama retains question order and boolean choices",
			request: `{"model":"clef","input":"Request received","questions":[
				{"type":"choice","name":"route","instructions":"Route this?","choices":[{"value":true},{"value":"manual"}]},
				{"type":"predicate","instructions":"Is there an incident?"}]}`,
			upstreamReply: `{"model":"clef","answers":{
				"q0001":{"type":"noul","noul":0.2},
				"q0000":{"type":"choice","choice":"option_000","confidence":0.8,"probabilities":{"option_000":0.9,"option_001":0.1}}},
				"usage":{"input_tokens":40,"output_tokens":2}}`,
			wantReply: `{"model":"clef","answers":[
				{"type":"choice","name":"route","choice":true,"confidence":0.8,"probabilities":[{"value":true,"probability":0.9},{"value":"manual","probability":0.1}]},
				{"type":"predicate","name":null,"probability":0.2}],
				"usage":{"input_tokens":40,"output_tokens":2,"total_tokens":42,"input_tokens_details":{"cached_tokens":0,"cache_write_tokens":0},"output_tokens_details":{"reasoning_tokens":0}}}`,
			toSystemOne: true, ollama: true,
		},
		{
			name: "Jev calls OpenAI Decisions and restores named results",
			request: `{"model":"gpt-6-luna","state":{"status":"degraded"},"questions":{
				"urgent":{"type":"noul","instructions":"Is this urgent?"},
				"severity":{"type":"score","instructions":"How severe?","criteria":["low","high"]}}}`,
			upstreamReply: `{"model":"gpt-6-luna","answers":[
				{"type":"score","name":"severity","score":0.8,"confidence":0.9,"probabilities":[{"value":0,"label":"low","probability":0.2},{"value":1,"label":"high","probability":0.8}]},
				{"type":"predicate","name":"urgent","probability":0.95}],"usage":{"input_tokens":40,"output_tokens":2,"total_tokens":42}}`,
			wantReply: `{"model":"gpt-6-luna","answers":{
				"severity":{"type":"score","score":0.8,"legend":{"0":"low","1":"high"},"confidence":0.9,"probabilities":{"0":0.2,"1":0.8}},
				"urgent":{"type":"noul","noul":0.95}},"usage":{"input_tokens":40,"output_tokens":2}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var request map[string]common.RawMessage
			require.NoError(t, common.UnmarshalJsonStr(tc.request, &request))
			outbound, mappings, err := convertDecisionRequest(request, tc.toSystemOne, tc.ollama)
			require.NoError(t, err)
			require.Len(t, mappings, 2)
			if tc.toSystemOne {
				questions := outbound["questions"].(map[string]any)
				assert.Equal(t, "noul", questions["q0001"].(map[string]any)["type"])
			} else {
				questions := outbound["questions"].([]decisionQuestion)
				assert.Equal(t, "predicate", questions[1].Type)
			}
			var reply decisionReply
			require.NoError(t, common.UnmarshalJsonStr(tc.upstreamReply, &reply))
			usage, err := reply.Usage.billing()
			require.NoError(t, err)
			assert.Equal(t, 42, usage.TotalTokens)
			converted, err := convertDecisionReply(reply, mappings, tc.toSystemOne)
			require.NoError(t, err)
			assert.JSONEq(t, tc.wantReply, string(converted))
		})
	}

	var nativeOnly map[string]common.RawMessage
	require.NoError(t, common.UnmarshalJsonStr(`{"model":"jev","state":"input","questions":{"box":{"type":"box","instructions":"Locate the logo"}}}`, &nativeOnly))
	_, _, err := convertDecisionRequest(nativeOnly, false, false)
	require.ErrorContains(t, err, "requires a native System One channel")
}
