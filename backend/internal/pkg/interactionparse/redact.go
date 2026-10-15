package interactionparse

import (
	"encoding/json"
	"reflect"
	"strings"
)

// Redact also inspects embedded JSON. Stream fragments cannot be independently
// redacted reliably, so their source representation is withheld; reconstructed
// tool arguments are redacted separately after parsing.
func Redact(value any, secret func(string) bool) any { return redact(value, secret, 0) }

func redact(value any, secret func(string) bool, depth int) any {
	if depth > 64 {
		return "[REDACTED: nesting limit]"
	}
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if secret(key) {
				out[key] = "[REDACTED]"
				continue
			}
			if key == "raw_text" {
				frames, format, warnings, ended := unpack(map[string]any{"raw_text": child})
				if format == "json_prefix" {
					out["parse_warnings"] = warnings
					out[key] = "[REDACTED: invalid JSON]"
					if len(frames) > 0 {
						data, _ := json.Marshal(redact(frames[0].value, secret, depth+1))
						out[key] = string(data)
					}
					continue
				}
				if format == "sse" || format == "ndjson" {
					var lines []string
					for _, f := range frames {
						data, _ := json.Marshal(redact(f.value, secret, depth+1))
						if format == "sse" {
							lines = append(lines, "data: "+string(data)+"\n")
						} else {
							lines = append(lines, string(data))
						}
					}
					if ended && format == "sse" {
						lines = append(lines, "data: [DONE]\n")
					}
					if len(warnings) > 0 {
						out["parse_warnings"] = warnings
					}
					out[key] = strings.Join(lines, "\n")
					continue
				}
			}
			if key == "partial_json" || (key == "delta" && (strings.Contains(str(v["type"]), "arguments.delta") || strings.Contains(str(v["type"]), "tool_call_input.delta"))) {
				out[key] = "[REDACTED: argument fragment]"
				continue
			}
			if key == "arguments" {
				if s, ok := child.(string); ok && s != "" && !json.Valid([]byte(s)) {
					out[key] = "[REDACTED: incomplete arguments]"
					continue
				}
			}
			out[key] = redact(child, secret, depth+1)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = redact(child, secret, depth+1)
		}
		return out
	case string:
		var decoded any
		if json.Unmarshal([]byte(v), &decoded) == nil {
			switch decoded.(type) {
			case map[string]any, []any:
				cleaned := redact(decoded, secret, depth+1)
				if reflect.DeepEqual(decoded, cleaned) {
					return v
				}
				data, _ := json.Marshal(cleaned)
				return string(data)
			}
		}
		return v
	default:
		return value
	}
}

func RedactDocument(doc *Document, secret func(string) bool) {
	redactBlocks(doc.Blocks, secret)
}

func redactBlocks(blocks []*Block, secret func(string) bool) {
	for _, b := range blocks {
		b.Data = Redact(b.Data, secret)
		b.Text = str(Redact(b.Text, secret))
		if b.Kind == "tool_call" && b.Text != "" {
			b.Text = "[REDACTED: incomplete arguments]"
		}
		if b.Kind == "custom_tool_call" {
			b.Text = str(Redact(b.Text, secret))
		}
		redactBlocks(b.Children, secret)
	}
}
