// Package interactionparse reconstructs captured conversations without changing
// gateway forwarding or assuming that a capture contains an entire session.
package interactionparse

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

type Block struct {
	Role       string   `json:"role"`
	Kind       string   `json:"kind"`
	Text       string   `json:"text,omitempty"`
	Name       string   `json:"name,omitempty"`
	CallID     string   `json:"call_id,omitempty"`
	Data       any      `json:"data,omitempty"`
	Source     string   `json:"source"`
	Group      string   `json:"group,omitempty"`
	Status     string   `json:"status,omitempty"`
	Sender     string   `json:"sender,omitempty"`
	Recipient  string   `json:"recipient,omitempty"`
	Children   []*Block `json:"children,omitempty"`
	dataSource string
}

type Document struct {
	Protocol string   `json:"protocol"`
	Format   string   `json:"format"`
	Blocks   []*Block `json:"blocks"`
	Warnings []string `json:"warnings"`
	Ended    bool     `json:"ended"`
}

type frame struct {
	value  any
	source string
}
type parser struct {
	doc      Document
	response bool
	byKey    map[string]*Block
	aliases  map[string]string
	keys     map[*Block]string
	finished map[string]bool
	depth    int
}

func newParser(response bool) parser {
	return parser{doc: Document{Protocol: "unknown", Format: "json", Blocks: []*Block{}, Warnings: []string{}}, response: response, byKey: map[string]*Block{}, aliases: map[string]string{}, keys: map[*Block]string{}, finished: map[string]bool{}}
}

func Parse(input map[string]any, response bool) Document {
	p := newParser(response)
	if len(input) == 0 {
		return p.doc
	}
	frames, format, warnings, ended := unpack(input)
	p.doc.Format, p.doc.Warnings, p.doc.Ended = format, warnings, ended
	if input["truncated"] == true {
		p.warn("truncated")
	}
	for _, f := range frames {
		p.read(f.value, f.source)
	}
	if ended {
		p.doc.Ended = true
	}
	if format == "json_prefix" {
		p.doc.Ended = false
	}
	if response && p.doc.Format != "json" && p.doc.Format != "text" && !p.doc.Ended {
		p.warn("missing_end")
	}
	p.finalizeBlocks(p.doc.Blocks, 0)
	for _, b := range p.doc.Blocks {
		if format == "json_prefix" {
			markPartial(b)
		}
	}
	// Output item/block indices are authoritative even when deltas interleave.
	if response && (p.doc.Protocol == "responses" || p.doc.Protocol == "anthropic") {
		prefix := "output/"
		if p.doc.Protocol == "anthropic" {
			prefix = "anthropic/part/"
		}
		rank := func(b *Block) int {
			k := strings.TrimPrefix(p.keys[b], prefix)
			if k == p.keys[b] {
				return int(^uint(0) >> 1)
			}
			n, err := strconv.Atoi(strings.Split(k, "/")[0])
			if err != nil {
				return int(^uint(0) >> 1)
			}
			return n
		}
		partRank := func(b *Block) int {
			key := p.keys[b]
			for _, separator := range []string{"/part/", "/summary/"} {
				if _, tail, ok := strings.Cut(key, separator); ok {
					n, _ := strconv.Atoi(strings.Split(tail, "/")[0])
					return n
				}
			}
			return 0
		}
		sort.SliceStable(p.doc.Blocks, func(i, j int) bool {
			left, right := rank(p.doc.Blocks[i]), rank(p.doc.Blocks[j])
			if left != right {
				return left < right
			}
			return partRank(p.doc.Blocks[i]) < partRank(p.doc.Blocks[j])
		})
	}
	return p.doc
}

func (p *parser) finalizeBlocks(items []*Block, depth int) {
	if depth > 64 {
		p.warn("nesting_limit")
		return
	}
	for _, b := range items {
		if b.Kind == "tool_call" && b.Text != "" {
			var value any
			if json.Unmarshal([]byte(b.Text), &value) == nil {
				b.Data, b.Text = value, ""
			} else {
				b.Status = "invalid_arguments"
			}
		}
		if b.Kind == "tool_result" {
			source := b.dataSource
			if source == "" {
				source = b.Source
			}
			b.Children = p.resultParts(b.Data, source, source, b.Group)
			if text, ok := b.Data.(string); ok && len(b.Children) > 0 {
				var decoded any
				if json.Unmarshal([]byte(text), &decoded) == nil {
					encoded, _ := json.Marshal(compactResultMedia(decoded))
					b.Data = string(encoded)
				}
			} else {
				b.Data = compactResultMedia(b.Data)
			}
		}
		p.finalizeBlocks(b.Children, depth+1)
	}
}

func (p *parser) finishGroup(group string, finished bool) {
	p.finished[group] = p.finished[group] || finished
	p.doc.Ended = true
	for _, done := range p.finished {
		if !done {
			p.doc.Ended = false
			break
		}
	}
}

func compactResultMedia(v any) any {
	switch value := v.(type) {
	case []any:
		out := make([]any, len(value))
		for i, child := range value {
			out[i] = compactResultMedia(child)
		}
		return out
	case map[string]any:
		switch str(value["type"]) {
		case "image", "image_url", "input_image", "input_audio", "audio", "file", "input_file", "document", "video":
			return attachmentMetadata(value)
		}
		out := make(map[string]any, len(value))
		for k, child := range value {
			if isOpaqueField(k) {
				out[k] = fmt.Sprintf("[opaque: %d characters]", len(str(child)))
			} else {
				out[k] = compactResultMedia(child)
			}
		}
		return out
	case string:
		if strings.HasPrefix(value, "data:") {
			return attachmentMetadata(value)
		}
		return value
	default:
		return v
	}
}

func unpack(input map[string]any) ([]frame, string, []string, bool) {
	if value, ok := input["raw_json"]; ok {
		return []frame{{value, "$.raw_json"}}, "json", nil, false
	}
	raw, ok := input["raw_text"].(string)
	if !ok {
		return []frame{{input, "$"}}, "json", nil, false
	}
	if strings.TrimSpace(raw) == "" {
		return nil, "text", nil, false
	}
	var decoded any
	if json.Unmarshal([]byte(raw), &decoded) == nil {
		return []frame{{decoded, "$.raw_text"}}, "json", nil, false
	}
	raw = strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")
	isSSE := false
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "data:") || strings.HasPrefix(line, "event:") {
			isSSE = true
			break
		}
	}
	if !isSSE && (strings.HasPrefix(strings.TrimSpace(raw), "{") || strings.HasPrefix(strings.TrimSpace(raw), "[")) {
		value, err := recoverJSONPrefix(raw)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return prefixCapture(value, "incomplete_json")
		}
		if err != nil {
			for _, field := range []string{"input", "output", "messages", "contents", "choices", "candidates"} {
				if obj(value)[field] != nil {
					return prefixCapture(value, "invalid_json")
				}
			}
			if len(arr(value)) > 0 {
				return prefixCapture(value, "invalid_json")
			}
		}
	}
	format := "ndjson"
	if isSSE {
		format = "sse"
	}
	var frames []frame
	var warnings []string
	ended := false
	decode := func(data, event string, index int) {
		if strings.TrimSpace(data) == "[DONE]" {
			ended = true
			return
		}
		if strings.TrimSpace(data) == "" {
			return
		}
		var v any
		if json.Unmarshal([]byte(data), &v) != nil {
			warnings = append(warnings, fmt.Sprintf("invalid_event:%d", index))
			return
		}
		if m, ok := v.(map[string]any); ok && m["type"] == nil && event != "" {
			m["type"] = event
		}
		frames = append(frames, frame{v, fmt.Sprintf("$.raw_text#%d", index)})
	}
	if isSSE {
		for i, event := range strings.Split(raw, "\n\n") {
			var data []string
			name := ""
			for _, line := range strings.Split(event, "\n") {
				if strings.HasPrefix(line, "data:") {
					data = append(data, strings.TrimPrefix(line[5:], " "))
				}
				if strings.HasPrefix(line, "event:") {
					name = strings.TrimSpace(line[6:])
				}
			}
			decode(strings.Join(data, "\n"), name, i)
		}
	} else {
		for i, line := range strings.Split(raw, "\n") {
			decode(line, "", i)
		}
	}
	if len(frames) == 0 && !isSSE && !strings.HasPrefix(strings.TrimSpace(raw), "{") && !strings.HasPrefix(strings.TrimSpace(raw), "[") {
		return []frame{{raw, "$.raw_text"}}, "text", nil, false
	}
	if !isSSE && len(frames) == 0 {
		value, err := recoverJSONPrefix(raw)
		warning := "invalid_json"
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			warning = "incomplete_json"
		}
		return prefixCapture(value, warning)
	}
	return frames, format, warnings, ended
}

func prefixCapture(value any, warning string) ([]frame, string, []string, bool) {
	if len(obj(value)) > 0 || len(arr(value)) > 0 {
		return []frame{{value, "$.raw_text"}}, "json_prefix", []string{warning, "recovered_prefix"}, false
	}
	return nil, "json_prefix", []string{warning}, false
}

func obj(v any) map[string]any { m, _ := v.(map[string]any); return m }
func arr(v any) []any          { a, _ := v.([]any); return a }
func str(v any) string         { s, _ := v.(string); return s }
func index(v any) string {
	if v == nil {
		return "0"
	}
	return fmt.Sprint(v)
}
func (p *parser) warn(w string) {
	for _, old := range p.doc.Warnings {
		if old == w {
			return
		}
	}
	p.doc.Warnings = append(p.doc.Warnings, w)
}
func (p *parser) protocol(name string) {
	if p.doc.Protocol == "unknown" {
		p.doc.Protocol = name
	} else if p.doc.Protocol != name {
		p.warn("mixed_protocol")
	}
}
func (p *parser) block(key, role, kind, source, group string) *Block {
	if b := p.byKey[key]; b != nil {
		return b
	}
	b := &Block{Role: role, Kind: kind, Source: source, Group: group}
	p.byKey[key] = b
	if p.keys != nil {
		p.keys[b] = key
	}
	p.doc.Blocks = append(p.doc.Blocks, b)
	return b
}
func (p *parser) unknown(v any, source, role, group string, stableKey ...string) {
	p.warn("unknown_content")
	key := source + "/unknown"
	if len(stableKey) > 0 {
		key = stableKey[0]
	}
	b := p.block(key, role, "unknown", source, group)
	b.Data = compactResultMedia(v)
	m := obj(v)
	if text, ok := m["text"].(string); ok && text != "" {
		b.Children = []*Block{{Role: role, Kind: "text", Text: text, Source: source + ".text", Group: group}}
	} else if content := m["content"]; content != nil {
		b.Children = p.contentParts(content, role, source+".content", source+"/content", group)
	} else if parts := m["parts"]; parts != nil {
		b.Children = p.contentParts(parts, role, source+".parts", source+"/parts", group)
	}
}

func (p *parser) read(v any, source string) {
	if list, ok := v.([]any); ok {
		if p.response && p.doc.Format == "json" {
			p.doc.Format = "json_array"
		}
		for i, child := range list {
			p.read(child, fmt.Sprintf("%s[%d]", source, i))
		}
		return
	}
	m := obj(v)
	if m == nil {
		if s, ok := v.(string); ok {
			p.block(source, "unknown", "text", source, "").Text = s
		} else {
			p.unknown(v, source, "unknown", "")
		}
		return
	}
	t := str(m["type"])
	if t == "keepalive" || t == "heartbeat" || t == "ping" {
		return
	}
	// Failed Responses snapshots may still contain readable output.
	if t == "error" || (m["error"] != nil && m["object"] != "response" && m["output"] == nil) {
		p.block(source+"/error", "system", "error", source, "").Data = m
		p.warn("failed")
		p.doc.Ended = true
		return
	}
	if strings.HasPrefix(t, "response.") {
		if p.doc.Format == "json" {
			p.doc.Format = "events"
		}
		p.protocol("responses")
		p.responsesEvent(m, source)
		return
	}
	if t == "message_start" || t == "message_delta" || t == "message_stop" || strings.HasPrefix(t, "content_block_") || t == "ping" {
		if p.doc.Format == "json" {
			p.doc.Format = "events"
		}
		p.protocol("anthropic")
		p.anthropicEvent(m, source)
		return
	}
	if m["choices"] != nil {
		p.protocol("chat_completions")
		p.chat(m, source)
		return
	}
	if m["candidates"] != nil || m["contents"] != nil || m["systemInstruction"] != nil || m["promptFeedback"] != nil {
		p.protocol("gemini")
		p.gemini(m, source)
		return
	}
	if m["output"] != nil || m["input"] != nil || m["instructions"] != nil || m["object"] == "response" {
		p.protocol("responses")
		if m["instructions"] != nil {
			p.parts(m["instructions"], "system", source+".instructions", source+"/instructions", "")
		}
		if s, ok := m["input"].(string); ok {
			p.parts(s, "user", source+".input", source+"/input", "")
		} else if item := obj(m["input"]); item != nil {
			p.responseItem(item, source+".input", "input/0")
		} else {
			for i, item := range p.collection(m["input"], source+".input", "user") {
				p.responseItem(item, fmt.Sprintf("%s.input[%d]", source, i), fmt.Sprintf("input/%d", i))
			}
		}
		p.responsesSnapshot(m, source)
		p.tools(m, source)
		if m["previous_response_id"] != nil {
			p.block(source+"/previous", "system", "reference", source+".previous_response_id", "").Data = m["previous_response_id"]
		}
		return
	}
	if m["messages"] != nil {
		messages := p.collection(m["messages"], source+".messages", "unknown")
		anthropic := m["system"] != nil || m["anthropic_version"] != nil
		for _, msg := range messages {
			for _, part := range arr(obj(msg)["content"]) {
				switch str(obj(part)["type"]) {
				case "tool_use", "tool_result", "thinking":
					anthropic = true
				}
			}
		}
		if anthropic {
			p.protocol("anthropic")
		} else {
			p.protocol("chat_completions")
		}
		if m["system"] != nil {
			p.parts(m["system"], "system", source+".system", source+"/system", "")
		}
		for i, msg := range messages {
			message := obj(msg)
			path := fmt.Sprintf("%s.messages[%d]", source, i)
			if message == nil {
				p.unknown(msg, path, "unknown", "")
				continue
			}
			role := str(message["role"])
			if role == "tool" {
				b := p.block(path, role, "tool_result", path, "")
				b.CallID = str(message["tool_call_id"])
				b.Data = message["content"]
				b.dataSource = path + ".content"
			} else {
				p.parts(message["content"], role, path+".content", path, "")
			}
			p.chatReasoning(message, role, path, path, "", false)
			for j, call := range arr(message["tool_calls"]) {
				p.chatTool(obj(call), path, fmt.Sprintf("%s/tool/%d", path, j), "", false)
			}
			if message["function_call"] != nil {
				p.chatTool(map[string]any{"function": message["function_call"]}, path, path+"/function", "", false)
			}
		}
		p.tools(m, source)
		return
	}
	if m["content"] != nil && (t == "message" || m["role"] != nil) {
		p.protocol("anthropic")
		p.parts(m["content"], str(m["role"]), source+".content", source, "")
		p.doc.Ended = m["stop_reason"] != nil
		if m["stop_reason"] != nil {
			p.block(source+"/finish", "system", "metadata", source+".stop_reason", "").Data = map[string]any{"stop_reason": m["stop_reason"]}
		}
		return
	}
	if len(m) > 0 {
		p.unknown(m, source, "unknown", "")
	}
}

func (p *parser) tools(m map[string]any, source string) {
	if len(p.collection(m["tools"], source+".tools", "system")) > 0 {
		p.block(source+"/tools", "system", "tool_definitions", source+".tools", "").Data = m["tools"]
	}
}

func (p *parser) collection(value any, source, role string) []any {
	if value == nil {
		return nil
	}
	if list, ok := value.([]any); ok {
		return list
	}
	p.unknown(value, source, role, "")
	return nil
}

func (p *parser) parts(v any, role, source, key, group string) {
	if role == "" {
		role = "assistant"
	}
	if s, ok := v.(string); ok {
		if s != "" {
			p.block(key+"/text", role, "text", source, group).Text = s
		}
		return
	}
	list := arr(v)
	singleton := obj(v) != nil
	if singleton {
		list = []any{v}
	}
	if v != nil && list == nil {
		p.unknown(v, source, role, group)
		return
	}
	for i, part := range list {
		m := obj(part)
		path := fmt.Sprintf("%s[%d]", source, i)
		k := fmt.Sprintf("%s/part/%d", key, i)
		if singleton {
			path = source
		}
		if text, ok := part.(string); ok {
			p.parts(text, role, path, k, group)
			continue
		}
		t := str(m["type"])
		switch t {
		case "text", "input_text", "output_text":
			p.block(k, role, "text", path, group).Text = str(m["text"])
		case "thinking":
			p.block(k, role, "reasoning", path, group).Text = str(m["thinking"])
		case "refusal":
			p.block(k, role, "refusal", path, group).Text = str(m["refusal"])
		case "tool_use", "server_tool_use":
			b := p.block(k, role, "tool_call", path, group)
			b.Name = str(m["name"])
			b.CallID = str(m["id"])
			b.Data = m["input"]
		case "tool_result":
			b := p.block(k, "tool", "tool_result", path, group)
			b.CallID = str(m["tool_use_id"])
			b.Data = m["content"]
			b.dataSource = path + ".content"
			if m["is_error"] == true {
				b.Status = "error"
			}
		case "image", "image_url", "input_image", "input_audio", "audio", "file", "input_file", "document", "video":
			p.block(k, role, "attachment", path, group).Data = attachmentMetadata(m)
		case "redacted_thinking", "encrypted_content":
			p.block(k, role, "opaque", path, group).Name = t
		default:
			if strings.HasSuffix(t, "_tool_result") {
				b := p.block(k, "tool", "tool_result", path, group)
				b.CallID = str(m["tool_use_id"])
				b.Data = m["content"]
				b.dataSource = path + ".content"
			} else {
				p.unknown(part, path, role, group, k)
			}
		}
		if m["annotations"] != nil {
			p.block(k+"/annotations", role, "metadata", path+".annotations", group).Data = m["annotations"]
		}
	}
}

func markPartial(b *Block) {
	if b.Status == "" {
		b.Status = "partial"
	}
	for _, child := range b.Children {
		markPartial(child)
	}
}

func (p *parser) contentParts(value any, role, source, key, group string) []*Block {
	if p.depth >= 64 {
		p.warn("nesting_limit")
		return nil
	}
	child := newParser(false)
	child.depth = p.depth + 1
	child.parts(value, role, source, key, group)
	for _, warning := range child.doc.Warnings {
		p.warn(warning)
	}
	return child.doc.Blocks
}

func (p *parser) resultParts(value any, source, key, group string) []*Block {
	if text, ok := value.(string); ok {
		var decoded any
		if json.Unmarshal([]byte(text), &decoded) != nil {
			return nil
		}
		value = decoded
	}
	if envelope := obj(value); envelope != nil {
		if envelope["content"] == nil {
			return nil
		}
		value, source = envelope["content"], source+".content"
	}
	if arr(value) == nil {
		return nil
	}
	return p.contentParts(value, "tool", source, key+"/result", group)
}

func isOpaqueField(key string) bool {
	return key == "encrypted_content" || key == "signature" || key == "thoughtSignature" || key == "thought_signature"
}

// Binary payloads and signatures are not useful text and should not inflate the DOM.
func attachmentMetadata(v any) any {
	switch value := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, child := range value {
			if k == "data" || k == "file_data" || k == "b64_json" {
				out[k] = fmt.Sprintf("[binary: %d characters]", len(str(child)))
			} else if isOpaqueField(k) {
				out[k] = fmt.Sprintf("[opaque: %d characters]", len(str(child)))
			} else {
				out[k] = attachmentMetadata(child)
			}
		}
		return out
	case []any:
		out := make([]any, len(value))
		for i, child := range value {
			out[i] = attachmentMetadata(child)
		}
		return out
	case string:
		if strings.HasPrefix(value, "data:") {
			if i := strings.Index(value, ","); i >= 0 {
				return fmt.Sprintf("%s,[%d characters]", value[:i], len(value)-i-1)
			}
		}
		return value
	default:
		return v
	}
}
