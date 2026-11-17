package service

import (
	"fmt"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
)

// ValidateModelCapabilities runs before token estimation and charging, and also
// before every WebSocket turn. It uses the original routing ID and metadata rules.
func ValidateModelCapabilities(c *gin.Context, request dto.Request) error {
	name := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	if req, ok := request.(*dto.OpenAIResponsesRequest); ok {
		if err := ValidateResponsesContextManagement(name, req); err != nil {
			return err
		}
	}
	kind, allowedInput, allowedOutput := model.ModelCapabilities(name)
	if kind == "" && len(allowedInput) == 0 && len(allowedOutput) == 0 {
		return nil
	}
	mode := relayconstant.Path2RelayMode(c.Request.URL.Path)
	operation, output := "chat", []string{"text"}
	switch mode {
	case relayconstant.RelayModeEmbeddings:
		operation, output = "embedding", []string{"embedding"}
	case relayconstant.RelayModeRerank:
		operation, output = "rerank", nil
	case relayconstant.RelayModeDecisions:
		operation, output = "decision", []string{"decision"}
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		operation, output = "image", []string{"image"}
	case relayconstant.RelayModeAudioSpeech:
		operation, output = "audio", []string{"audio"}
	case relayconstant.RelayModeAudioTranscription, relayconstant.RelayModeAudioTranslation:
		operation = "audio"
	}
	if strings.Contains(c.Request.URL.Path, ":embedContent") || strings.Contains(c.Request.URL.Path, ":batchEmbedContents") {
		operation, output = "embedding", []string{"embedding"}
	}
	if kind != "" && kind != operation && !(kind == "chat" && operation == "decision") {
		return fmt.Errorf("model %q is a %s model and cannot be used for %s requests", name, kind, operation)
	}
	input := make(map[string]bool)
	meta := request.GetTokenCountMeta()
	if meta != nil {
		if meta.CombineText != "" {
			input["text"] = true
		}
		for _, file := range meta.Files {
			if file != nil {
				input[string(file.FileType)] = true
			}
		}
	}
	switch req := request.(type) {
	case *dto.ImageRequest:
		if req.Prompt != "" {
			input["text"] = true
		}
		if mode == relayconstant.RelayModeImagesEdits || len(req.Image) > 0 || len(req.Images) > 0 {
			input["image"] = true
		}
	case *dto.OpenAIResponsesRequest:
		var tools []struct {
			Type string `json:"type"`
		}
		if len(req.Tools) > 0 {
			if err := common.Unmarshal(req.Tools, &tools); err != nil {
				return fmt.Errorf("invalid Responses tools")
			}
		}
		for _, tool := range tools {
			if tool.Type == "image_generation" {
				output = append(output, "image")
			}
		}
	case *dto.AudioRequest:
		if mode == relayconstant.RelayModeAudioSpeech {
			input["text"] = true
		} else {
			input["audio"] = true
		}
	case *dto.GeneralOpenAIRequest:
		if len(req.Modalities) > 0 {
			if err := common.Unmarshal(req.Modalities, &output); err != nil {
				return fmt.Errorf("invalid output modalities")
			}
		}
	case *dto.GeminiChatRequest:
		if len(req.GenerationConfig.ResponseModalities) > 0 {
			output = req.GenerationConfig.ResponseModalities
		}
	}
	for modality := range input {
		if len(allowedInput) > 0 && !slices.Contains(allowedInput, modality) {
			return fmt.Errorf("model %q does not accept %s input; allowed input modalities: %s", name, modality, strings.Join(allowedInput, ", "))
		}
	}
	for _, modality := range output {
		modality = strings.ToLower(modality)
		if len(allowedOutput) > 0 && !slices.Contains(allowedOutput, modality) {
			return fmt.Errorf("model %q does not support %s output; allowed output modalities: %s", name, modality, strings.Join(allowedOutput, ", "))
		}
	}
	return nil
}

// Automatic compaction must use an explicitly configured model window. An
// absent threshold defaults to 80% of that window; a client cannot exceed it.
func ValidateResponsesContextManagement(name string, request *dto.OpenAIResponsesRequest) error {
	if len(request.ContextManagement) == 0 || strings.TrimSpace(string(request.ContextManagement)) == "null" {
		return nil
	}
	var policies []struct {
		Type             string `json:"type"`
		CompactThreshold *int   `json:"compact_threshold,omitempty"`
	}
	if err := common.Unmarshal(request.ContextManagement, &policies); err != nil {
		return fmt.Errorf("context_management must be an array of compaction policies")
	}
	for i := range policies {
		policy := &policies[i]
		if policy.Type != "compaction" {
			return fmt.Errorf("unsupported context management type %q", policy.Type)
		}
		window := model.ModelContextLength(name)
		if window <= 0 {
			return fmt.Errorf("automatic Responses compaction requires an explicit context length for model %q", name)
		}
		if policy.CompactThreshold == nil {
			policy.CompactThreshold = common.GetPointer(max(1, int(int64(window)*4/5)))
		}
		if *policy.CompactThreshold < 1 || *policy.CompactThreshold >= window {
			return fmt.Errorf("compact_threshold must be greater than 0 and below model %q context length (%d tokens)", name, window)
		}
	}
	encoded, err := common.Marshal(policies)
	if err != nil {
		return err
	}
	request.ContextManagement = encoded
	return nil
}
