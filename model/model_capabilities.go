package model

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

var modelTypes = []string{"", "chat", "embedding", "rerank", "image", "audio", "video", "decision"}
var modelModalities = []string{"text", "image", "audio", "video", "file", "embedding", "decision"}

func (m *Model) ValidateCapabilities() error {
	if m.ContextLength < 0 || m.ContextLength > math.MaxInt32 {
		return fmt.Errorf("context length must be between 0 and %d tokens", math.MaxInt32)
	}
	if !slices.Contains(modelTypes, m.ModelType) {
		return fmt.Errorf("invalid model type: %s", m.ModelType)
	}
	for direction, values := range [][]string{m.InputModalities, m.OutputModalities} {
		for _, modality := range values {
			if !slices.Contains(modelModalities, modality) || (direction == 0 && (modality == "embedding" || modality == "decision")) {
				return fmt.Errorf("invalid model modality: %s", modality)
			}
		}
	}
	return nil
}

// Explicit metadata wins over conservative inference for existing non-chat models.
// An unspecified modality list remains unrestricted until an administrator sets it.
func (m *Model) EffectiveCapabilities(name string) (kind string, input, output []string) {
	if m != nil {
		kind, input, output = m.ModelType, m.InputModalities, m.OutputModalities
	}
	if kind == "" {
		normalized := strings.ToLower(name)
		switch {
		case common.IsJevDecisionModel(normalized):
			kind = "decision"
		case strings.Contains(normalized, "embed"):
			kind = "embedding"
		case strings.Contains(normalized, "rerank"):
			kind = "rerank"
		case common.IsImageGenerationModel(name):
			kind = "image"
		}
	}
	if kind == "decision" && len(output) == 0 {
		output = []string{"decision"}
	}
	return
}

func ModelCapabilities(name string) (string, []string, []string) {
	return modelEndpointMetadataForName(name).EffectiveCapabilities(name)
}

func ModelContextLength(name string) int {
	if metadata := modelEndpointMetadataForName(name); metadata != nil {
		return metadata.ContextLength
	}
	return 0
}
