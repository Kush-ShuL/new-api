package dto

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// DecisionRequest keeps native request extensions, while exposing the evidence
// and questions to request size estimation and model capability checks.
type DecisionRequest struct {
	dto.BaseRequest
	Model string
	Body  map[string]common.RawMessage
}

func (r *DecisionRequest) SetModelName(model string) {
	r.Model = model
}

func (r *DecisionRequest) GetTokenCountMeta() *types.TokenCountMeta {
	meta := &types.TokenCountMeta{TokenType: types.TokenTypeTokenizer}
	var text strings.Builder
	for _, key := range []string{"input", "state", "questions", "assets", "images"} {
		var value any
		if len(r.Body[key]) == 0 || common.Unmarshal(r.Body[key], &value) != nil {
			continue
		}
		if key == "images" {
			if images, ok := value.([]any); ok {
				for _, item := range images {
					if data, ok := item.(string); ok && data != "" {
						meta.Files = append(meta.Files, types.NewImageFileMeta(types.NewBase64FileSource(data, ""), ""))
					}
				}
			}
			continue
		}
		collectDecisionEvidence(value, &text, meta)
	}
	meta.CombineText = text.String()
	return meta
}

func collectDecisionEvidence(value any, text *strings.Builder, meta *types.TokenCountMeta) {
	switch item := value.(type) {
	case nil:
	case string:
		text.WriteString(item)
		text.WriteByte('\n')
	case []any:
		for _, child := range item {
			collectDecisionEvidence(child, text, meta)
		}
	case map[string]any:
		if url, ok := item["image_url"].(string); item["type"] == "input_image" && ok && url != "" {
			detail, _ := item["detail"].(string)
			meta.Files = append(meta.Files, types.NewImageFileMeta(types.NewFileSourceFromData(url, ""), detail))
			return
		}
		if data, ok := item["data_base64"].(string); ok && data != "" {
			meta.Files = append(meta.Files, types.NewImageFileMeta(types.NewBase64FileSource(data, ""), ""))
			return
		}
		for key, child := range item {
			text.WriteString(key)
			text.WriteByte('\n')
			collectDecisionEvidence(child, text, meta)
		}
	default:
		fmt.Fprintln(text, item)
	}
}
