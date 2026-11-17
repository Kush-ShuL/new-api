package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
)

type ResponsesOutputProcessor struct {
	Context            *gin.Context
	Info               *relaycommon.RelayInfo
	Request            *dto.OpenAIResponsesRequest
	SummarizeReasoning func(string) (string, error)
	sequence           int
	emulated           bool
	reasoning          map[int]*responsesReasoningStream
	active             []*responsesReasoningStream
	indexes            map[int]int
	output             []any
	parts              map[string]bool
	partDone           map[string]bool
	settings           map[string]any
}

type responsesReasoningStream struct {
	id            string
	index         int
	segment       int
	pending       string
	text          strings.Builder
	total         int
	summary       []any
	closed        bool
	paragraphs    int
	paragraphOpen bool
	sectionClosed bool
}

func (p *ResponsesOutputProcessor) converted() bool {
	return p.emulated || p.Info.GetFinalRequestRelayFormat() != types.RelayFormatOpenAIResponses
}

func responsesReasoningTitle(line string) string {
	line = strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(line, "#"):
		line = strings.TrimSpace(strings.TrimLeft(line, "#"))
		line = strings.TrimSpace(strings.TrimRight(line, "#"))
	case strings.HasPrefix(line, "**") && strings.HasSuffix(line, "**") && len(line) > 4:
		line = strings.TrimSpace(line[2 : len(line)-2])
	default:
		return ""
	}
	if line == "" {
		return ""
	}
	return "**" + line + "**"
}

func responsesReasoningText(item map[string]any) string {
	var text strings.Builder
	for _, key := range []string{"summary", "content"} {
		if parts, ok := item[key].([]any); ok {
			for _, raw := range parts {
				if part, ok := raw.(map[string]any); ok {
					if value, ok := part["text"].(string); ok {
						text.WriteString(value)
					}
				}
			}
		}
	}
	return text.String()
}

func (p *ResponsesOutputProcessor) startReasoning(s *responsesReasoningStream) map[string]any {
	s.index = len(p.output)
	id := s.id
	if s.segment > 0 {
		id = fmt.Sprintf("%s_%d", id, s.segment)
	}
	item := map[string]any{"id": id, "type": "reasoning", "status": "in_progress", "summary": []any{}}
	p.output = append(p.output, item)
	return map[string]any{"type": "response.output_item.added", "output_index": s.index, "item": item}
}

func (p *ResponsesOutputProcessor) reasoningEvent(s *responsesReasoningStream, kind string) map[string]any {
	item := p.output[s.index].(map[string]any)
	return map[string]any{"type": kind, "output_index": s.index, "item_id": item["id"], "summary_index": 0}
}

// Each heading is a complete public summary. Section bodies remain private
// until sealed at the next heading or before the next output item.
func (p *ResponsesOutputProcessor) reasoningLine(s *responsesReasoningStream, line string) ([]map[string]any, error) {
	var events []map[string]any
	if p.SummarizeReasoning != nil {
		if s.sectionClosed {
			if strings.TrimSpace(line) == "" {
				return nil, nil
			}
			s.segment++
			s.text.Reset()
			s.summary = nil
			s.paragraphs = 0
			s.sectionClosed = false
			events = append(events, p.startReasoning(s))
		}
		s.text.WriteString(line)
		if strings.TrimSpace(line) != "" {
			s.paragraphOpen = true
		} else if s.paragraphOpen {
			s.paragraphs++
			s.paragraphOpen = false
		}
		paragraphs := 3
		if p.Request.Reasoning != nil {
			switch p.Request.Reasoning.Summary {
			case "concise":
				paragraphs = 6
			case "detailed":
				paragraphs = 1
			}
		}
		if s.paragraphs >= paragraphs || s.text.Len() >= 8192 {
			generated, err := p.generateReasoningSummary(s)
			return append(events, generated...), err
		}
		return events, nil
	}
	if title := responsesReasoningTitle(line); title != "" {
		if len(s.summary) > 0 {
			done, err := p.finishReasoningSection(s)
			if err != nil {
				return nil, err
			}
			events = append(events, done)
			s.segment++
			s.text.Reset()
			events = append(events, p.startReasoning(s))
		}
		events = append(events, p.reasoningSummaryEvents(s, title)...)
	}
	s.text.WriteString(line)
	return events, nil
}

func (p *ResponsesOutputProcessor) reasoningSummaryEvents(s *responsesReasoningStream, text string) []map[string]any {
	part := map[string]any{"type": "summary_text", "text": text}
	s.summary = []any{part}
	added := p.reasoningEvent(s, "response.reasoning_summary_part.added")
	added["part"] = map[string]any{"type": "summary_text", "text": ""}
	delta := p.reasoningEvent(s, "response.reasoning_summary_text.delta")
	delta["delta"] = text
	textDone := p.reasoningEvent(s, "response.reasoning_summary_text.done")
	textDone["text"] = text
	partDone := p.reasoningEvent(s, "response.reasoning_summary_part.done")
	partDone["part"] = part
	return []map[string]any{added, delta, textDone, partDone}
}

func (p *ResponsesOutputProcessor) generateReasoningSummary(s *responsesReasoningStream) ([]map[string]any, error) {
	text, err := p.SummarizeReasoning(s.text.String())
	if err != nil {
		return nil, fmt.Errorf("generate reasoning summary: %w", err)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("reasoning summary model returned empty text")
	}
	events := p.reasoningSummaryEvents(s, text)
	done, err := p.finishReasoningSection(s)
	if err != nil {
		return nil, err
	}
	s.sectionClosed = true
	return append(events, done), nil
}

func (p *ResponsesOutputProcessor) appendReasoning(s *responsesReasoningStream, delta string) ([]map[string]any, error) {
	if s.closed {
		return nil, fmt.Errorf("reasoning item %q received text after completion", s.id)
	}
	if delta == "" {
		return nil, nil
	}
	s.total += len(delta)
	if !model_setting.GetGlobalSettings().ResponsesReasoningEncryption {
		var events []map[string]any
		if s.text.Len() == 0 {
			added := p.reasoningEvent(s, "response.reasoning_summary_part.added")
			added["part"] = map[string]any{"type": "summary_text", "text": ""}
			events = append(events, added)
		}
		s.text.WriteString(delta)
		event := p.reasoningEvent(s, "response.reasoning_summary_text.delta")
		event["delta"] = delta
		return append(events, event), nil
	}
	s.pending += delta
	var events []map[string]any
	for {
		line, rest, found := strings.Cut(s.pending, "\n")
		if !found {
			break
		}
		s.pending = rest
		lineEvents, err := p.reasoningLine(s, line+"\n")
		if err != nil {
			return nil, err
		}
		events = append(events, lineEvents...)
	}
	return events, nil
}

func (p *ResponsesOutputProcessor) finishReasoningSection(s *responsesReasoningStream) (map[string]any, error) {
	// Keep already-emitted snapshots immutable.
	previous := p.output[s.index].(map[string]any)
	item := map[string]any{"id": previous["id"], "type": "reasoning", "status": "completed", "summary": s.summary}
	if s.summary == nil {
		item["summary"] = []any{}
	}
	if model_setting.GetGlobalSettings().ResponsesReasoningEncryption && s.text.Len() > 0 {
		encrypted, err := SealResponsesReasoning(common.GetContextKeyInt(p.Context, constant.ContextKeyUserId), s.text.String())
		if err != nil {
			return nil, err
		}
		item["encrypted_content"] = encrypted
	}
	p.output[s.index] = item
	return map[string]any{"type": "response.output_item.done", "output_index": s.index, "item": item}, nil
}

func (p *ResponsesOutputProcessor) finishReasoning(s *responsesReasoningStream) ([]map[string]any, error) {
	if s.closed {
		return nil, nil
	}
	var events []map[string]any
	if s.pending != "" {
		pending := s.pending
		s.pending = ""
		lineEvents, err := p.reasoningLine(s, pending)
		if err != nil {
			return nil, err
		}
		events = append(events, lineEvents...)
	}
	if !model_setting.GetGlobalSettings().ResponsesReasoningEncryption && s.text.Len() > 0 {
		part := map[string]any{"type": "summary_text", "text": s.text.String()}
		s.summary = []any{part}
		textDone := p.reasoningEvent(s, "response.reasoning_summary_text.done")
		textDone["text"] = s.text.String()
		partDone := p.reasoningEvent(s, "response.reasoning_summary_part.done")
		partDone["part"] = part
		events = append(events, textDone, partDone)
	}
	if model_setting.GetGlobalSettings().ResponsesReasoningEncryption && p.SummarizeReasoning != nil {
		if !s.sectionClosed && s.text.Len() > 0 {
			generated, err := p.generateReasoningSummary(s)
			if err != nil {
				return nil, err
			}
			events = append(events, generated...)
		} else if !s.sectionClosed {
			done, err := p.finishReasoningSection(s)
			if err != nil {
				return nil, err
			}
			events = append(events, done)
		}
		s.closed = true
		return events, nil
	}
	done, err := p.finishReasoningSection(s)
	if err != nil {
		return nil, err
	}
	s.closed = true
	return append(events, done), nil
}

func (p *ResponsesOutputProcessor) closeReasoning() ([]map[string]any, error) {
	var events []map[string]any
	for _, s := range p.active {
		closed, err := p.finishReasoning(s)
		if err != nil {
			return nil, err
		}
		events = append(events, closed...)
	}
	return events, nil
}

// Billing provenance stays in the host; only the public Responses usage
// contract crosses the client boundary. The adaptor has already settled usage.
func (p *ResponsesOutputProcessor) normalizeEnvelope(response map[string]any) error {
	if usage, ok := response["usage"].(map[string]any); ok {
		input, output := usage["input_tokens"], usage["output_tokens"]
		if input == nil {
			input = usage["prompt_tokens"]
		}
		if output == nil {
			output = usage["completion_tokens"]
		}
		if input == nil {
			input = 0
		}
		if output == nil {
			output = 0
		}
		cached, reasoning := any(0), any(0)
		for _, key := range []string{"prompt_tokens_details", "input_tokens_details"} {
			if details, ok := usage[key].(map[string]any); ok && details["cached_tokens"] != nil {
				cached = details["cached_tokens"]
			}
		}
		for _, key := range []string{"completion_tokens_details", "output_tokens_details"} {
			if details, ok := usage[key].(map[string]any); ok && details["reasoning_tokens"] != nil {
				reasoning = details["reasoning_tokens"]
			}
		}
		response["usage"] = map[string]any{
			"input_tokens": input, "output_tokens": output, "total_tokens": usage["total_tokens"],
			"input_tokens_details":  map[string]any{"cached_tokens": cached},
			"output_tokens_details": map[string]any{"reasoning_tokens": reasoning},
		}
	}
	if !p.converted() {
		return nil
	}
	if id, ok := response["id"].(string); ok && strings.HasPrefix(id, "chatcmpl") {
		response["id"] = "resp_" + strings.TrimLeft(strings.TrimPrefix(id, "chatcmpl"), "-_")
	}
	if !p.emulated {
		if p.settings == nil {
			p.settings = map[string]any{
				"temperature": 1, "top_p": 1, "tools": []any{}, "metadata": map[string]any{},
				"tool_choice": "auto", "truncation": "disabled", "store": true,
				"parallel_tool_calls": true, "text": map[string]any{"format": map[string]any{"type": "text"}},
				"max_output_tokens": p.Request.MaxOutputTokens, "reasoning": p.Request.Reasoning,
			}
			if p.Request.Temperature != nil {
				p.settings["temperature"] = *p.Request.Temperature
			}
			if p.Request.TopP != nil {
				p.settings["top_p"] = *p.Request.TopP
			}
			for key, raw := range map[string]common.RawMessage{
				"tools": p.Request.Tools, "metadata": p.Request.Metadata, "tool_choice": p.Request.ToolChoice,
				"truncation": p.Request.Truncation, "store": p.Request.Store, "parallel_tool_calls": p.Request.ParallelToolCalls,
				"instructions": p.Request.Instructions, "text": p.Request.Text,
			} {
				if len(raw) > 0 && string(raw) != "null" {
					var value any
					if err := common.Unmarshal(raw, &value); err != nil {
						return err
					}
					p.settings[key] = value
				}
			}
		}
		for key, value := range p.settings {
			response[key] = value
		}
	}
	return nil
}

func (p *ResponsesOutputProcessor) transformResponse(response map[string]any) error {
	if !p.converted() {
		return nil
	}
	if p.output != nil {
		response["output"] = p.output
		return nil
	}
	output, _ := response["output"].([]any)
	for _, raw := range output {
		item, ok := raw.(map[string]any)
		if ok && item["type"] != "image_generation_call" {
			delete(item, "quality")
			delete(item, "size")
		}
		if ok && item["type"] == "message" && (item["phase"] == nil || item["phase"] == "") {
			item["phase"] = "final_answer"
		}
		if !ok || item["type"] != "reasoning" {
			p.output = append(p.output, raw)
			continue
		}
		id, _ := item["id"].(string)
		s := &responsesReasoningStream{id: id}
		p.startReasoning(s)
		if _, err := p.appendReasoning(s, responsesReasoningText(item)); err != nil {
			return err
		}
		if _, err := p.finishReasoning(s); err != nil {
			return err
		}
	}
	if p.output == nil {
		p.output = []any{}
	}
	response["output"] = p.output
	return nil
}

func (p *ResponsesOutputProcessor) saveResponse(response map[string]any) error {
	body, err := common.Marshal(response)
	if err != nil {
		return err
	}
	if id, _ := response["id"].(string); id != "" {
		if err := SaveResponsesContext(p.Context, id, p.Request, body); err != nil {
			return fmt.Errorf("save Responses context: %w", err)
		}
	}
	return nil
}

func (p *ResponsesOutputProcessor) ProcessResponse(body []byte) ([]byte, error) {
	var response map[string]any
	if err := common.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if id, _ := response["id"].(string); strings.HasPrefix(id, "chatcmpl") && !p.converted() {
		p.emulated = true
	}
	if err := p.normalizeEnvelope(response); err != nil {
		return nil, err
	}
	if err := p.transformResponse(response); err != nil {
		return nil, err
	}
	if err := p.saveResponse(response); err != nil {
		return nil, err
	}
	return common.Marshal(response)
}

func (p *ResponsesOutputProcessor) ProcessEvent(raw string) ([]string, error) {
	var event map[string]any
	if err := common.UnmarshalJsonStr(raw, &event); err != nil {
		return nil, err
	}
	kind, _ := event["type"].(string)
	if response, ok := event["response"].(map[string]any); ok && kind == "response.created" {
		if id, _ := response["id"].(string); strings.HasPrefix(id, "chatcmpl") && !p.converted() {
			p.emulated = true
		}
	}
	var events []map[string]any
	if p.converted() {
		var err error
		events, err = p.convertEvent(event)
		if err != nil {
			return nil, err
		}
		if p.emulated && kind == "response.created" {
			progress := map[string]any{"type": "response.in_progress", "response": event["response"]}
			events = append(events, progress)
		}
	} else {
		events = []map[string]any{event}
	}
	result := make([]string, 0, len(events))
	for _, event := range events {
		if response, ok := event["response"].(map[string]any); ok {
			if err := p.normalizeEnvelope(response); err != nil {
				return nil, err
			}
			switch event["type"] {
			case "response.completed", "response.done", "response.incomplete", "response.failed":
				if err := p.transformResponse(response); err != nil {
					return nil, err
				}
				if err := p.saveResponse(response); err != nil {
					return nil, err
				}
			}
		}
		if p.converted() {
			event["sequence_number"] = p.sequence
			p.sequence++
		}
		body, err := common.Marshal(event)
		if err != nil {
			return nil, err
		}
		result = append(result, string(body))
	}
	return result, nil
}

func (p *ResponsesOutputProcessor) convertEvent(event map[string]any) ([]map[string]any, error) {
	if p.reasoning == nil {
		p.reasoning = make(map[int]*responsesReasoningStream)
		p.indexes = make(map[int]int)
		p.parts = make(map[string]bool)
		p.partDone = make(map[string]bool)
	}
	kind, _ := event["type"].(string)
	if p.emulated && kind == "response.in_progress" {
		return nil, nil
	}
	indexValue, hasIndex := event["output_index"].(float64)
	index := int(indexValue)
	item, _ := event["item"].(map[string]any)
	if item != nil && item["type"] != "image_generation_call" {
		delete(item, "quality")
		delete(item, "size")
	}
	if item != nil && item["type"] == "message" && (item["phase"] == nil || item["phase"] == "") {
		item["phase"] = "final_answer"
	}
	if kind == "response.output_item.added" && item["type"] == "reasoning" {
		events, err := p.closeReasoning()
		if err != nil {
			return nil, err
		}
		id, _ := item["id"].(string)
		s := &responsesReasoningStream{id: id}
		p.reasoning[index] = s
		p.active = append(p.active, s)
		return append(events, p.startReasoning(s)), nil
	}
	if s := p.reasoning[index]; hasIndex && s != nil {
		switch kind {
		case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
			delta, _ := event["delta"].(string)
			return p.appendReasoning(s, delta)
		case "response.reasoning_summary_text.done", "response.reasoning_text.done":
			if s.total == 0 && !s.closed {
				text, _ := event["text"].(string)
				return p.appendReasoning(s, text)
			}
			return nil, nil
		case "response.reasoning_summary_part.added", "response.reasoning_summary_part.done":
			return nil, nil
		case "response.output_item.done":
			var events []map[string]any
			if s.total == 0 && !s.closed {
				var err error
				events, err = p.appendReasoning(s, responsesReasoningText(item))
				if err != nil {
					return nil, err
				}
			}
			closed, err := p.finishReasoning(s)
			return append(events, closed...), err
		}
	}
	var events []map[string]any
	if kind == "response.output_item.added" || kind == "response.completed" || kind == "response.incomplete" || kind == "response.failed" {
		var err error
		events, err = p.closeReasoning()
		if err != nil {
			return nil, err
		}
	}
	if kind == "response.output_item.added" {
		p.indexes[index] = len(p.output)
		p.output = append(p.output, item)
	}
	if mapped, ok := p.indexes[index]; hasIndex && ok {
		event["output_index"] = mapped
		if kind == "response.output_item.done" {
			p.output[mapped] = item
		}
	}
	if kind == "response.output_item.added" || kind == "response.output_item.done" {
		delete(event, "item_id")
	}
	// Tolerate older Responses-shaped upstreams that omit content-part events.
	// Current relaykit converters already produce these events; avoid duplicates.
	contentIndex, _ := event["content_index"].(float64)
	partKey := fmt.Sprintf("%d:%d", index, int(contentIndex))
	switch kind {
	case "response.content_part.added":
		if p.parts[partKey] {
			return events, nil
		}
		p.parts[partKey] = true
	case "response.content_part.done":
		if p.partDone[partKey] {
			return events, nil
		}
		p.partDone[partKey] = true
	case "response.output_text.delta", "response.output_text.done":
		if event["logprobs"] == nil {
			event["logprobs"] = []any{}
		}
		if !p.parts[partKey] {
			added := map[string]any{
				"type": "response.content_part.added", "output_index": event["output_index"],
				"item_id": event["item_id"], "content_index": int(contentIndex),
				"part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}},
			}
			events = append(events, added)
			p.parts[partKey] = true
		}
		if p.emulated && kind == "response.output_text.done" {
			done := map[string]any{
				"type": "response.content_part.done", "output_index": event["output_index"],
				"item_id": event["item_id"], "content_index": int(contentIndex),
				"part": map[string]any{"type": "output_text", "text": event["text"], "annotations": []any{}},
			}
			p.partDone[partKey] = true
			return append(events, event, done), nil
		}
	}
	return append(events, event), nil
}
