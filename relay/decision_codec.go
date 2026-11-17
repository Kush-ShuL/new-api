package relay

import (
	"encoding/base64"
	"errors"
	"fmt"
	"maps"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
)

type decisionOption struct {
	Value       any    `json:"value"`
	Description string `json:"description,omitempty"`
}

type decisionLevel struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type decisionQuestion struct {
	Type         string            `json:"type"`
	Name         *string           `json:"name,omitempty"`
	Instructions common.RawMessage `json:"instructions"`
	Choices      []decisionOption  `json:"choices,omitempty"`
	Levels       []decisionLevel   `json:"levels,omitempty"`
}

type systemOneQuestion struct {
	Type         string            `json:"type"`
	Instructions common.RawMessage `json:"instructions"`
	Criteria     common.RawMessage `json:"criteria,omitempty"`
}

type decisionMapping struct {
	Key      string
	Question decisionQuestion
	Options  []string
	Levels   []string
}

type decisionAsset struct {
	Type   string `json:"type"`
	Format string `json:"format"`
	Data   string `json:"data_base64"`
}

// Cross-protocol conversion only accepts fields it can represent faithfully.
// Same-protocol relays keep the original request body and provider extensions.
func convertDecisionRequest(body map[string]common.RawMessage, toSystemOne, ollama bool) (map[string]any, []decisionMapping, error) {
	if toSystemOne {
		return decisionsToSystemOne(body, ollama)
	}
	return systemOneToDecisions(body)
}

func decisionsToSystemOne(body map[string]common.RawMessage, ollama bool) (map[string]any, []decisionMapping, error) {
	for _, key := range []string{"prompt_cache_key", "prompt_cache_options", "safety_identifier"} {
		if value := body[key]; len(value) != 0 && string(value) != "null" {
			return nil, nil, fmt.Errorf("%s is not supported by System One", key)
		}
	}
	var questions []decisionQuestion
	if err := common.Unmarshal(body["questions"], &questions); err != nil {
		return nil, nil, err
	}
	if ollama && len(questions) > 64 {
		return nil, nil, errors.New("Ollama accepts up to 64 questions per request")
	}
	assets := make(map[string]decisionAsset)
	var images []string
	renderParts := func(raw common.RawMessage, shared bool) (any, error) {
		if common.GetJsonType(raw) == "string" {
			var text string
			err := common.Unmarshal(raw, &text)
			return text, err
		}
		var parts []struct {
			Type       string            `json:"type"`
			Text       string            `json:"text"`
			ImageURL   string            `json:"image_url"`
			Refusal    string            `json:"refusal"`
			Data       common.RawMessage `json:"data"`
			Breakpoint common.RawMessage `json:"prompt_cache_breakpoint"`
		}
		if err := common.Unmarshal(raw, &parts); err != nil || parts == nil {
			return nil, errors.New("content must be text or a content-part array")
		}
		out := make([]any, 0, len(parts))
		for _, part := range parts {
			if len(part.Breakpoint) > 0 && string(part.Breakpoint) != "null" {
				return nil, errors.New("cache breakpoints cannot be converted to System One")
			}
			switch part.Type {
			case "input_text", "output_text", "":
				out = append(out, part.Text)
			case "refusal":
				out = append(out, part.Refusal)
			case "input_json":
				out = append(out, part.Data)
			case "input_image":
				header, data, ok := strings.Cut(part.ImageURL, ";base64,")
				format, inline := strings.CutPrefix(header, "data:image/")
				if !ok || !inline || data == "" || !slices.Contains([]string{"png", "jpg", "jpeg", "webp"}, format) {
					return nil, errors.New("decision conversion requires inline base64 PNG, JPEG, or WebP images")
				}
				if _, err := base64.StdEncoding.DecodeString(data); err != nil {
					return nil, errors.New("invalid base64 image")
				}
				if ollama {
					if !shared {
						return nil, errors.New("Ollama supports shared images only, not images in question instructions")
					}
					images = append(images, data)
					out = append(out, fmt.Sprintf("[Image %d]", len(images)))
				} else {
					if format == "webp" {
						return nil, errors.New("TypeSafe accepts PNG and JPEG images")
					}
					key := "image_" + common.GetUUID()
					assets[key] = decisionAsset{"image", format, data}
					out = append(out, key)
				}
			default:
				return nil, fmt.Errorf("unsupported decision input part %q", part.Type)
			}
		}
		if len(out) == 1 {
			return out[0], nil
		}
		return out, nil
	}

	var state any
	if common.GetJsonType(body["input"]) == "string" {
		state = body["input"]
	} else {
		var items []map[string]common.RawMessage
		if err := common.Unmarshal(body["input"], &items); err != nil || items == nil {
			return nil, nil, errors.New("input must be text or an input-item array")
		}
		rendered := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if len(item["prompt_cache_breakpoint"]) != 0 && string(item["prompt_cache_breakpoint"]) != "null" {
				return nil, nil, errors.New("cache breakpoints cannot be converted to System One")
			}
			entry := make(map[string]any, len(item))
			for k, v := range item {
				entry[k] = v
			}
			for _, k := range []string{"content", "output"} {
				if raw := item[k]; len(raw) > 0 {
					value, err := renderParts(raw, true)
					if err != nil {
						return nil, nil, err
					}
					entry[k] = value
				}
			}
			rendered = append(rendered, entry)
		}
		state = rendered
	}

	named := make(map[string]any, len(questions))
	transforms := make([]decisionMapping, 0, len(questions))
	for i, q := range questions {
		m := decisionMapping{Key: fmt.Sprintf("q%04d", i), Question: q}
		instructions, err := renderParts(q.Instructions, false)
		if err != nil {
			return nil, nil, fmt.Errorf("questions[%d]: %w", i, err)
		}
		if ollama {
			if _, ok := instructions.(string); !ok {
				data, err := common.Marshal(instructions)
				if err != nil {
					return nil, nil, err
				}
				instructions = string(data)
			}
		}
		item := map[string]any{"type": q.Type, "instructions": instructions}
		switch q.Type {
		case "predicate":
			item["type"] = "noul"
		case "choice":
			if len(q.Choices) < 2 || len(q.Choices) > 255 || (ollama && len(q.Choices) > 26) {
				return nil, nil, fmt.Errorf("questions[%d] has an unsupported number of choices", i)
			}
			criteria := make(map[string]string, len(q.Choices))
			seen := make(map[any]bool, len(q.Choices))
			for j, option := range q.Choices {
				switch option.Value.(type) {
				case bool, string:
				default:
					return nil, nil, fmt.Errorf("questions[%d] choice values must be strings or booleans", i)
				}
				if seen[option.Value] {
					return nil, nil, fmt.Errorf("questions[%d] has duplicate choices", i)
				}
				seen[option.Value] = true
				key := fmt.Sprintf("option_%03d", j)
				description := fmt.Sprint(option.Value)
				if option.Description != "" {
					description += ": " + option.Description
				}
				criteria[key] = description
				m.Options = append(m.Options, key)
			}
			item["criteria"] = criteria
		case "score":
			if len(q.Levels) < 2 || len(q.Levels) > 10 {
				return nil, nil, fmt.Errorf("questions[%d] needs 2–10 levels", i)
			}
			for _, level := range q.Levels {
				description := level.Label
				if level.Description != "" {
					description += ": " + level.Description
				}
				m.Levels = append(m.Levels, description)
			}
			item["criteria"] = m.Levels
		default:
			return nil, nil, fmt.Errorf("questions[%d] has unsupported type %q", i, q.Type)
		}
		named[m.Key] = item
		transforms = append(transforms, m)
	}
	out := map[string]any{"model": body["model"], "state": state, "questions": named}
	if len(assets) > 0 {
		out["assets"] = assets
	}
	if len(images) > 0 {
		out["images"] = images
	}
	return out, transforms, nil
}

func systemOneToDecisions(body map[string]common.RawMessage) (map[string]any, []decisionMapping, error) {
	if raw := body["assets"]; len(raw) > 0 && string(raw) != "null" && common.GetJsonType(raw) == "object" {
		var assets map[string]common.RawMessage
		if err := common.Unmarshal(raw, &assets); err != nil || len(assets) > 0 {
			return nil, nil, errors.New("TypeSafe asset references require a native System One channel")
		}
	}
	if raw := body["keep_alive"]; len(raw) > 0 && string(raw) != "null" {
		return nil, nil, errors.New("keep_alive requires a native Ollama channel")
	}
	parts := make([]map[string]any, 0, 1)
	state := body["state"]
	switch common.GetJsonType(state) {
	case "string":
		parts = append(parts, map[string]any{"type": "input_text", "text": state})
	case "object":
		parts = append(parts, map[string]any{"type": "input_json", "data": state})
	default:
		parts = append(parts, map[string]any{"type": "input_text", "text": string(state)})
	}
	var images []string
	if raw := body["images"]; len(raw) > 0 {
		if err := common.Unmarshal(raw, &images); err != nil {
			return nil, nil, errors.New("images must be a base64 array")
		}
	}
	for _, image := range images {
		data, err := base64.StdEncoding.DecodeString(image)
		if err != nil {
			return nil, nil, errors.New("invalid base64 image")
		}
		mime := http.DetectContentType(data)
		if !slices.Contains([]string{"image/png", "image/jpeg", "image/webp"}, mime) {
			return nil, nil, errors.New("unsupported decision image type")
		}
		parts = append(parts, map[string]any{"type": "input_image", "image_url": "data:" + mime + ";base64," + image})
	}

	var named map[string]systemOneQuestion
	if err := common.Unmarshal(body["questions"], &named); err != nil {
		return nil, nil, err
	}
	if len(named) > 400 {
		return nil, nil, errors.New("OpenAI Decisions accepts up to 400 questions")
	}
	out := make([]decisionQuestion, 0, len(named))
	transforms := make([]decisionMapping, 0, len(named))
	for _, name := range slices.Sorted(maps.Keys(named)) {
		q := named[name]
		m := decisionMapping{Key: name, Question: decisionQuestion{Type: q.Type, Name: &name, Instructions: q.Instructions}}
		switch common.GetJsonType(q.Instructions) {
		case "string":
		case "object", "array":
			m.Question.Instructions, _ = common.Marshal(string(q.Instructions))
		default:
			return nil, nil, fmt.Errorf("question %q has invalid instructions", name)
		}
		switch q.Type {
		case "noul":
			m.Question.Type = "predicate"
			if len(q.Criteria) > 0 && string(q.Criteria) != "null" {
				var instructions string
				_ = common.Unmarshal(m.Question.Instructions, &instructions)
				m.Question.Instructions, _ = common.Marshal(instructions + "\nInterpret false and true according to these criteria: " + string(q.Criteria))
			}
		case "choice":
			var criteria map[string]common.RawMessage
			if err := common.Unmarshal(q.Criteria, &criteria); err != nil || len(criteria) < 2 || len(criteria) > 255 {
				return nil, nil, fmt.Errorf("question %q needs 2–255 choices", name)
			}
			for _, label := range slices.Sorted(maps.Keys(criteria)) {
				m.Question.Choices = append(m.Question.Choices, decisionOption{label, decisionDescription(criteria[label])})
				m.Options = append(m.Options, label)
			}
		case "score":
			var criteria []common.RawMessage
			if err := common.Unmarshal(q.Criteria, &criteria); err != nil || len(criteria) < 2 || len(criteria) > 10 {
				return nil, nil, fmt.Errorf("question %q needs 2–10 levels", name)
			}
			for _, raw := range criteria {
				level := decisionDescription(raw)
				m.Question.Levels = append(m.Question.Levels, decisionLevel{Label: level})
				m.Levels = append(m.Levels, level)
			}
		default:
			return nil, nil, fmt.Errorf("question %q type %q requires a native System One channel", name, q.Type)
		}
		out = append(out, m.Question)
		transforms = append(transforms, m)
	}
	return map[string]any{
		"model": body["model"], "input": []any{map[string]any{"role": "user", "content": parts}}, "questions": out,
	}, transforms, nil
}

func decisionDescription(raw common.RawMessage) string {
	var s string
	if common.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

type decisionUsage struct {
	Input        *int `json:"input_tokens"`
	Output       *int `json:"output_tokens"`
	Total        int  `json:"total_tokens"`
	InputDetails struct {
		Cached int `json:"cached_tokens"`
		Write  int `json:"cache_write_tokens"`
	} `json:"input_tokens_details"`
	OutputDetails struct {
		Reasoning int `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
}

func (u decisionUsage) billing() (*kitdto.Usage, error) {
	if u.Input == nil || u.Output == nil || *u.Input < 0 || *u.Output < 0 ||
		int64(*u.Input) > math.MaxInt32 || int64(*u.Output) > math.MaxInt32-int64(*u.Input) ||
		u.InputDetails.Cached < 0 || u.InputDetails.Cached > *u.Input ||
		u.InputDetails.Write < 0 || u.InputDetails.Write > *u.Input-u.InputDetails.Cached ||
		u.OutputDetails.Reasoning < 0 || u.OutputDetails.Reasoning > *u.Output {
		return nil, errors.New("invalid upstream decision token usage")
	}
	return &kitdto.Usage{
		PromptTokens: *u.Input, CompletionTokens: *u.Output, TotalTokens: *u.Input + *u.Output,
		PromptTokensDetails:    kitdto.InputTokenDetails{CachedTokens: u.InputDetails.Cached, CacheWriteTokens: u.InputDetails.Write},
		CompletionTokenDetails: kitdto.OutputTokenDetails{ReasoningTokens: u.OutputDetails.Reasoning},
	}, nil
}

type decisionReply struct {
	Model   string            `json:"model"`
	Answers common.RawMessage `json:"answers"`
	Usage   decisionUsage     `json:"usage"`
}

func convertDecisionReply(reply decisionReply, mappings []decisionMapping, fromSystemOne bool) ([]byte, error) {
	if fromSystemOne {
		var named map[string]struct {
			Type          string             `json:"type"`
			Noul          *float64           `json:"noul"`
			Choice        string             `json:"choice"`
			Score         *float64           `json:"score"`
			Confidence    *float64           `json:"confidence"`
			Probabilities map[string]float64 `json:"probabilities"`
		}
		if err := common.Unmarshal(reply.Answers, &named); err != nil || len(named) != len(mappings) {
			return nil, errors.New("upstream System One answers do not match the questions")
		}
		answers := make([]map[string]any, 0, len(named))
		for _, m := range mappings {
			got := named[m.Key]
			q := m.Question
			out := map[string]any{"type": q.Type, "name": q.Name}
			switch q.Type {
			case "predicate":
				if got.Type != "noul" || got.Noul == nil || !probability(*got.Noul) {
					return nil, fmt.Errorf("invalid System One predicate answer for %s", m.Key)
				}
				out["probability"] = *got.Noul
			case "choice", "score":
				if got.Type != q.Type || got.Confidence == nil || !probability(*got.Confidence) {
					return nil, fmt.Errorf("invalid System One %s answer for %s", q.Type, m.Key)
				}
				out["confidence"] = *got.Confidence
				probs := make([]map[string]any, 0, len(got.Probabilities))
				if q.Type == "choice" {
					selected := slices.Index(m.Options, got.Choice)
					if selected < 0 {
						return nil, errors.New("System One selected an unknown choice")
					}
					out["choice"] = q.Choices[selected].Value
					for i, key := range m.Options {
						p, ok := got.Probabilities[key]
						if !ok || !probability(p) {
							return nil, errors.New("invalid System One choice probabilities")
						}
						probs = append(probs, map[string]any{"value": q.Choices[i].Value, "probability": p})
					}
				} else {
					if got.Score == nil || *got.Score < 0 || *got.Score > float64(len(m.Levels)-1) {
						return nil, errors.New("System One score is out of range")
					}
					out["score"] = *got.Score
					for i, level := range q.Levels {
						p, ok := got.Probabilities[strconv.Itoa(i)]
						if !ok {
							p, ok = got.Probabilities[m.Levels[i]]
						}
						if !ok || !probability(p) {
							return nil, errors.New("invalid System One score probabilities")
						}
						probs = append(probs, map[string]any{"value": i, "label": level.Label, "probability": p})
					}
				}
				out["probabilities"] = probs
			}
			answers = append(answers, out)
		}
		reply.Usage.Total = *reply.Usage.Input + *reply.Usage.Output
		return common.Marshal(map[string]any{"model": reply.Model, "answers": answers, "usage": reply.Usage})
	}

	var ordered []struct {
		Type          string   `json:"type"`
		Name          *string  `json:"name"`
		Probability   *float64 `json:"probability"`
		Choice        *string  `json:"choice"`
		Score         *float64 `json:"score"`
		Confidence    *float64 `json:"confidence"`
		Probabilities []struct {
			Value       any     `json:"value"`
			Probability float64 `json:"probability"`
		} `json:"probabilities"`
	}
	if err := common.Unmarshal(reply.Answers, &ordered); err != nil || len(ordered) != len(mappings) {
		return nil, errors.New("upstream Decisions answers do not match the questions")
	}
	answers := make(map[string]any, len(ordered))
	for i, m := range mappings {
		got := ordered[i]
		if got.Name == nil || *got.Name != m.Key || got.Type != m.Question.Type {
			return nil, fmt.Errorf("Decision %q was refused or returned an incompatible answer", m.Key)
		}
		out := map[string]any{"type": got.Type}
		switch got.Type {
		case "predicate":
			if got.Probability == nil || !probability(*got.Probability) {
				return nil, errors.New("invalid Decisions predicate probability")
			}
			out["type"], out["noul"] = "noul", *got.Probability
		case "choice", "score":
			if got.Confidence == nil || !probability(*got.Confidence) {
				return nil, errors.New("invalid Decisions confidence")
			}
			out["confidence"] = *got.Confidence
			probs := make(map[string]float64, len(got.Probabilities))
			for _, p := range got.Probabilities {
				if !probability(p.Probability) {
					return nil, errors.New("invalid Decisions probability")
				}
				if got.Type == "choice" {
					label, ok := p.Value.(string)
					if !ok || !slices.Contains(m.Options, label) {
						return nil, errors.New("Decisions returned an unknown choice")
					}
					probs[label] = p.Probability
				} else {
					idx, ok := p.Value.(float64)
					if !ok || idx < 0 || idx >= float64(len(m.Levels)) || idx != math.Trunc(idx) {
						return nil, errors.New("Decisions returned an unknown score level")
					}
					probs[strconv.Itoa(int(idx))] = p.Probability
				}
			}
			if got.Type == "choice" {
				if got.Choice == nil || !slices.Contains(m.Options, *got.Choice) || len(probs) != len(m.Options) {
					return nil, errors.New("Decisions choice or distribution does not match the criteria")
				}
				out["choice"] = *got.Choice
			} else {
				if got.Score == nil || *got.Score < 0 || *got.Score > float64(len(m.Levels)-1) || len(probs) != len(m.Levels) {
					return nil, errors.New("Decisions score or distribution does not match the criteria")
				}
				legend := make(map[string]string, len(m.Levels))
				for j, label := range m.Levels {
					legend[strconv.Itoa(j)] = label
				}
				out["score"], out["legend"] = *got.Score, legend
			}
			out["probabilities"] = probs
		}
		answers[m.Key] = out
	}
	return common.Marshal(map[string]any{
		"model": reply.Model, "answers": answers,
		"usage": map[string]int{"input_tokens": *reply.Usage.Input, "output_tokens": *reply.Usage.Output},
	})
}

func probability(value float64) bool {
	return value >= 0 && value <= 1
}
