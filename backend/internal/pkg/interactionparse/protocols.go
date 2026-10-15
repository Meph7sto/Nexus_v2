package interactionparse

import (
	"fmt"
	"strings"
)

func (p *parser) responseItem(value any, source, key string) {
	role := "assistant"
	if strings.HasPrefix(key, "input/") {
		role = "user"
	}
	if text, ok := value.(string); ok {
		p.parts(text, role, source, key, "")
		return
	}
	m := obj(value)
	if m == nil {
		p.unknown(value, source, "unknown", "", key)
		return
	}
	if id := str(m["id"]); id != "" {
		p.aliases[id] = key
	}
	if explicit := str(m["role"]); explicit != "" {
		role = explicit
	} else {
		role = "assistant"
	}
	switch str(m["type"]) {
	case "message", "":
		if m["content"] != nil {
			p.parts(m["content"], role, source+".content", key, "")
		} else {
			p.unknown(m, source, role, "", key)
		}
	case "reasoning":
		for i, summary := range arr(m["summary"]) {
			p.block(fmt.Sprintf("%s/summary/%d", key, i), role, "reasoning", source+".summary", "").Text = str(obj(summary)["text"])
		}
		if m["encrypted_content"] != nil {
			p.block(key+"/opaque", role, "opaque", source+".encrypted_content", "").Name = "encrypted_content"
		}
	case "function_call", "custom_tool_call":
		b := p.block(key, role, "tool_call", source, "")
		b.Name = str(m["name"])
		b.CallID = str(m["call_id"])
		if a, ok := m["arguments"]; ok {
			b.Text = str(a)
			if _, ok := a.(string); !ok {
				b.Data = a
			}
		}
		if a, ok := m["input"]; ok {
			b.Kind = "custom_tool_call"
			b.Text = str(a)
		}
		b.Status = str(m["status"])
	case "function_call_output", "custom_tool_call_output":
		b := p.block(key, "tool", "tool_result", source, "")
		b.CallID = str(m["call_id"])
		b.Data = m["output"]
		b.dataSource = source + ".output"
	case "additional_tools":
		if arr(m["tools"]) == nil {
			p.unknown(m, source, role, "", key)
			return
		}
		if str(m["role"]) == "" {
			role = "system"
		}
		p.block(key, role, "tool_definitions", source+".tools", "").Data = m["tools"]
	case "agent_message":
		b := p.block(key, "agent", "agent_message", source, "")
		b.Sender, b.Recipient = str(m["author"]), str(m["recipient"])
		b.Children = p.contentParts(m["content"], "agent", source+".content", key+"/content", "")
		b.Data = map[string]any{"id": m["id"], "author": m["author"], "recipient": m["recipient"]}
	case "compaction":
		b := p.block(key, "system", "compaction", source, "")
		b.Data = compactResultMedia(m)
	case "item_reference":
		p.block(key, role, "reference", source, "").Data = m
	default:
		if strings.HasSuffix(str(m["type"]), "_call") {
			b := p.block(key, role, "tool_call", source, "")
			b.Name = str(m["type"])
			b.CallID = str(m["id"])
			b.Data = m
			b.Status = str(m["status"])
		} else {
			p.unknown(m, source, role, "", key)
		}
	}
}

func (p *parser) responsesSnapshot(m map[string]any, source string) {
	for i, item := range p.collection(m["output"], source+".output", "assistant") {
		p.responseItem(item, fmt.Sprintf("%s.output[%d]", source, i), fmt.Sprintf("output/%d", i))
	}
	if status := str(m["status"]); status == "completed" || status == "failed" || status == "incomplete" || status == "cancelled" {
		p.doc.Ended = true
		if status != "completed" {
			p.warn(status)
		}
	}
	if m["error"] != nil {
		p.block(source+"/error", "system", "error", source+".error", "").Data = m["error"]
	}
	if m["incomplete_details"] != nil {
		p.block(source+"/incomplete", "system", "metadata", source+".incomplete_details", "").Data = m["incomplete_details"]
	}
}

func (p *parser) responsesEvent(m map[string]any, source string) {
	t := str(m["type"])
	key := "output/" + index(m["output_index"])
	if id := str(m["item_id"]); id != "" {
		if alias := p.aliases[id]; alias != "" {
			key = alias
		} else {
			p.aliases[id] = key
		}
	}
	part := key + "/part/" + index(m["content_index"])
	switch t {
	case "response.created", "response.in_progress", "response.queued":
		return
	case "response.completed", "response.done", "response.failed", "response.incomplete", "response.cancelled":
		p.responsesSnapshot(obj(m["response"]), source+".response")
		p.doc.Ended = true
		if t != "response.completed" && t != "response.done" {
			p.warn(strings.TrimPrefix(t, "response."))
		}
	case "response.output_item.added", "response.output_item.done":
		p.responseItem(m["item"], source+".item", key)
	case "response.content_part.added", "response.content_part.done":
		partData := obj(m["part"])
		if partData["type"] != "output_text" && partData["type"] != "refusal" {
			p.unknown(partData, source+".part", "assistant", "")
			return
		}
		kind, text := "text", str(partData["text"])
		if partData["type"] == "refusal" {
			kind, text = "refusal", str(partData["refusal"])
		}
		b := p.block(part, "assistant", kind, source, "")
		b.Text = text
		if partData["annotations"] != nil {
			p.block(part+"/annotations", "assistant", "metadata", source, "").Data = partData["annotations"]
		}
	case "response.output_text.delta", "response.output_text.done", "response.refusal.delta", "response.refusal.done":
		kind := "text"
		if strings.Contains(t, "refusal") {
			kind = "refusal"
		}
		b := p.block(part, "assistant", kind, source, "")
		if strings.HasSuffix(t, ".delta") {
			b.Text += str(m["delta"])
		} else if kind == "refusal" {
			b.Text = str(m["refusal"])
		} else {
			b.Text = str(m["text"])
		}
	case "response.reasoning_summary_text.delta", "response.reasoning_summary_text.done", "response.reasoning_text.delta", "response.reasoning_text.done":
		k := key + "/summary/" + index(m["summary_index"])
		if strings.Contains(t, "reasoning_text") {
			k = key + "/reasoning/" + index(m["content_index"])
		}
		b := p.block(k, "assistant", "reasoning", source, "")
		if strings.HasSuffix(t, ".delta") {
			b.Text += str(m["delta"])
		} else {
			b.Text = str(m["text"])
		}
	case "response.reasoning_summary_part.added", "response.reasoning_summary_part.done":
		p.block(key+"/summary/"+index(m["summary_index"]), "assistant", "reasoning", source, "").Text = str(obj(m["part"])["text"])
	case "response.function_call_arguments.delta", "response.function_call_arguments.done", "response.custom_tool_call_input.delta", "response.custom_tool_call_input.done":
		kind := "tool_call"
		field := "arguments"
		if strings.Contains(t, "custom_tool") {
			kind, field = "custom_tool_call", "input"
		}
		b := p.block(key, "assistant", kind, source, "")
		b.Kind = kind
		if name := str(m["name"]); name != "" {
			b.Name = name
		}
		if id := str(m["call_id"]); id != "" {
			b.CallID = id
		}
		if strings.HasSuffix(t, ".delta") {
			b.Text += str(m["delta"])
		} else {
			b.Text = str(m[field])
		}
	case "response.output_text.annotation.added":
		b := p.block(part+"/annotations", "assistant", "metadata", source, "")
		annotations := obj(b.Data)
		if annotations == nil {
			annotations = map[string]any{}
		}
		annotations[index(m["annotation_index"])] = m["annotation"]
		b.Data = annotations
	default:
		p.unknown(m, source, "system", "")
	}
}

func (p *parser) chatReasoning(m map[string]any, role, source, key, group string, delta bool) {
	value := m["reasoning_content"]
	if value == nil {
		value = m["reasoning"]
	}
	if s := str(value); s != "" {
		b := p.block(key+"/reasoning", role, "reasoning", source, group)
		if delta {
			b.Text += s
		} else {
			b.Text = s
		}
	}
}

func (p *parser) chatTool(m map[string]any, source, key, group string, delta bool) {
	b := p.block(key, "assistant", "tool_call", source, group)
	if id := str(m["id"]); id != "" {
		b.CallID = id
	}
	f := obj(m["function"])
	if name := str(f["name"]); name != "" {
		if delta {
			b.Name += name
		} else {
			b.Name = name
		}
	}
	if a, ok := f["arguments"]; ok {
		if delta {
			b.Text += str(a)
		} else {
			b.Text = str(a)
		}
	}
	if custom := obj(m["custom"]); custom != nil {
		b.Kind = "custom_tool_call"
		b.Name = str(custom["name"])
		if delta {
			b.Text += str(custom["input"])
		} else {
			b.Text = str(custom["input"])
		}
	}
}

func (p *parser) chat(m map[string]any, source string) {
	for i, choice := range p.collection(m["choices"], source+".choices", "assistant") {
		c := obj(choice)
		if c == nil {
			p.unknown(choice, fmt.Sprintf("%s.choices[%d]", source, i), "assistant", "")
			continue
		}
		idx := index(c["index"])
		if c["index"] == nil {
			idx = fmt.Sprint(i)
		}
		key := "choice/" + idx
		path := fmt.Sprintf("%s.choices[%d]", source, i)
		message := obj(c["message"])
		delta := false
		if d := obj(c["delta"]); d != nil {
			message, delta = d, true
		}
		if text, ok := message["content"].(string); ok {
			b := p.block(key+"/text", "assistant", "text", path, key)
			if delta {
				b.Text += text
			} else {
				b.Text = text
			}
		} else if message["content"] != nil {
			p.parts(message["content"], "assistant", path, key, key)
		}
		p.chatReasoning(message, "assistant", path, key, key, delta)
		if s := str(message["refusal"]); s != "" {
			b := p.block(key+"/refusal", "assistant", "refusal", path, key)
			if delta {
				b.Text += s
			} else {
				b.Text = s
			}
		}
		for j, call := range arr(message["tool_calls"]) {
			tool := obj(call)
			ti := index(tool["index"])
			if tool["index"] == nil {
				ti = fmt.Sprint(j)
			}
			p.chatTool(tool, path, key+"/tool/"+ti, key, delta)
		}
		if message["function_call"] != nil {
			p.chatTool(map[string]any{"function": message["function_call"]}, path, key+"/function", key, delta)
		}
		if c["finish_reason"] != nil {
			p.block(key+"/finish", "system", "metadata", path+".finish_reason", key).Data = map[string]any{"finish_reason": c["finish_reason"]}
		}
		p.finishGroup(key, c["finish_reason"] != nil)
	}
}

func (p *parser) anthropicEvent(m map[string]any, source string) {
	key := "anthropic/part/" + index(m["index"])
	switch str(m["type"]) {
	case "ping":
		return
	case "message_start":
		p.parts(obj(m["message"])["content"], "assistant", source+".message.content", "anthropic", "")
	case "content_block_start":
		part := obj(m["content_block"])
		// Parse using the real block index rather than the singleton array index.
		tmp := parser{doc: Document{Blocks: []*Block{}}, byKey: map[string]*Block{}}
		tmp.parts([]any{part}, "assistant", source, "block", "")
		for i, b := range tmp.doc.Blocks {
			k := key
			if i > 0 {
				k += fmt.Sprintf("/extra/%d", i)
			}
			target := p.block(k, b.Role, b.Kind, b.Source, "")
			*target = *b
		}
	case "content_block_delta":
		d := obj(m["delta"])
		kind, field := "text", "text"
		switch str(d["type"]) {
		case "thinking_delta":
			kind, field = "reasoning", "thinking"
		case "input_json_delta":
			kind, field = "tool_call", "partial_json"
		case "signature_delta":
			p.block(key+"/signature", "assistant", "opaque", source, "").Name = "signature"
			return
		case "text_delta":
		default:
			p.unknown(m, source, "assistant", "")
			return
		}
		b := p.block(key, "assistant", kind, source, "")
		b.Text += str(d[field])
		if kind == "tool_call" {
			b.Data = nil
		}
	case "content_block_stop":
		if b := p.byKey[key]; b != nil {
			b.Status = "complete"
		}
	case "message_delta":
		if reason := obj(m["delta"])["stop_reason"]; reason != nil {
			p.block("anthropic/finish", "system", "metadata", source, "").Data = map[string]any{"stop_reason": reason}
		}
	case "message_stop":
		p.doc.Ended = true
	default:
		p.unknown(m, source, "system", "")
	}
}

func (p *parser) gemini(m map[string]any, source string) {
	if system := obj(m["systemInstruction"]); system != nil {
		p.geminiParts(arr(system["parts"]), "system", source+".systemInstruction", "system", false)
	}
	for i, content := range p.collection(m["contents"], source+".contents", "unknown") {
		c := obj(content)
		if c == nil {
			p.unknown(content, fmt.Sprintf("%s.contents[%d]", source, i), "unknown", "")
			continue
		}
		p.geminiParts(arr(c["parts"]), str(c["role"]), fmt.Sprintf("%s.contents[%d]", source, i), fmt.Sprintf("input/%d", i), false)
	}
	for i, candidate := range p.collection(m["candidates"], source+".candidates", "assistant") {
		c := obj(candidate)
		if c == nil {
			p.unknown(candidate, fmt.Sprintf("%s.candidates[%d]", source, i), "assistant", "")
			continue
		}
		idx := index(c["index"])
		if c["index"] == nil {
			idx = fmt.Sprint(i)
		}
		key := "candidate/" + idx
		p.geminiParts(arr(obj(c["content"])["parts"]), "assistant", fmt.Sprintf("%s.candidates[%d]", source, i), key, p.doc.Format != "json")
		if c["finishReason"] != nil {
			p.block(key+"/finish", "system", "metadata", source, key).Data = map[string]any{"finishReason": c["finishReason"], "safetyRatings": c["safetyRatings"]}
		}
		p.finishGroup(key, c["finishReason"] != nil)
		if c["groundingMetadata"] != nil {
			p.block(key+"/grounding", "assistant", "metadata", source, key).Data = c["groundingMetadata"]
		}
	}
	if m["promptFeedback"] != nil {
		p.block(source+"/feedback", "system", "metadata", source+".promptFeedback", "").Data = m["promptFeedback"]
		if obj(m["promptFeedback"])["blockReason"] != nil {
			p.doc.Ended = true
			p.warn("blocked")
		}
	}
	p.tools(m, source)
}

func (p *parser) geminiParts(parts []any, role, source, key string, delta bool) {
	if role == "model" {
		role = "assistant"
	}
	for i, value := range parts {
		m := obj(value)
		path := fmt.Sprintf("%s.parts[%d]", source, i)
		k := fmt.Sprintf("%s/part/%d", key, i)
		switch {
		case m["text"] != nil:
			kind := "text"
			if m["thought"] == true {
				kind = "reasoning"
			}
			if delta {
				k = key + "/" + kind
				if len(p.doc.Blocks) > 0 {
					last := p.doc.Blocks[len(p.doc.Blocks)-1]
					if last.Kind != kind || last.Group != key {
						k += "/" + source + fmt.Sprint(i)
					} else {
						last.Text += str(m["text"])
						if m["thoughtSignature"] != nil {
							p.block(k+"/signature", role, "opaque", path, key).Name = "thoughtSignature"
						}
						continue
					}
				}
			}
			b := p.block(k, role, kind, path, key)
			if delta {
				b.Text += str(m["text"])
			} else {
				b.Text = str(m["text"])
			}
		case m["functionCall"] != nil:
			f := obj(m["functionCall"])
			if delta {
				k = path
			}
			b := p.block(k, role, "tool_call", path, key)
			b.Name = str(f["name"])
			b.CallID = str(f["id"])
			b.Data = f["args"]
		case m["functionResponse"] != nil:
			f := obj(m["functionResponse"])
			b := p.block(k, "tool", "tool_result", path, key)
			b.Name = str(f["name"])
			b.CallID = str(f["id"])
			b.Data = f["response"]
			b.dataSource = path + ".functionResponse.response"
		case m["inlineData"] != nil || m["fileData"] != nil:
			if delta {
				k = path
			}
			p.block(k, role, "attachment", path, key).Data = attachmentMetadata(m)
		case m["executableCode"] != nil || m["codeExecutionResult"] != nil:
			p.block(k, role, "metadata", path, key).Data = m
		default:
			p.unknown(value, path, role, key)
		}
		if m["thoughtSignature"] != nil {
			p.block(k+"/signature", role, "opaque", path, key).Name = "thoughtSignature"
		}
	}
}
