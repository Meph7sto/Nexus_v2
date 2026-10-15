package interactionparse

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Recover closed fields and array items only. An unfinished message or tool
// argument is not made valid by appending quotes, braces, or missing bytes.
func recoverJSONPrefix(raw string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	value, _, err := prefixValue(decoder, raw, 0)
	return value, err
}

func prefixValue(decoder *json.Decoder, raw string, depth int) (any, bool, error) {
	if depth > 64 {
		return nil, false, fmt.Errorf("JSON prefix nesting limit")
	}
	token, err := prefixToken(decoder, raw)
	if err != nil {
		return nil, false, err
	}
	end := int64(len(strings.TrimRight(raw, " \r\n\t")))
	switch token {
	case json.Delim('{'):
		out := map[string]any{}
		for decoder.InputOffset() < end && decoder.More() {
			key, err := prefixToken(decoder, raw)
			if err != nil {
				return out, false, err
			}
			name, ok := key.(string)
			if !ok {
				return out, false, fmt.Errorf("invalid JSON object key")
			}
			value, complete, err := prefixValue(decoder, raw, depth+1)
			items, isArray := value.([]any)
			if complete || (isArray && len(items) > 0) {
				out[name] = value
			}
			if err != nil {
				return out, false, err
			}
		}
		_, err := prefixToken(decoder, raw)
		return out, err == nil, err
	case json.Delim('['):
		items := []any{}
		for decoder.InputOffset() < end && decoder.More() {
			var item json.RawMessage
			if err := decoder.Decode(&item); err != nil {
				return items, false, fmt.Errorf("array item decode at %d after %d items: %w", decoder.InputOffset(), len(items), err)
			}
			var value any
			if err := json.Unmarshal(item, &value); err != nil {
				return items, false, fmt.Errorf("array item unmarshal: %w", err)
			}
			if _, number := value.(float64); number && decoder.InputOffset() == int64(len(raw)) {
				return items, false, io.ErrUnexpectedEOF
			}
			items = append(items, value)
		}
		_, err := prefixToken(decoder, raw)
		return items, err == nil, err
	default:
		if _, number := token.(float64); number && decoder.InputOffset() == int64(len(raw)) {
			return nil, false, io.ErrUnexpectedEOF
		}
		return token, true, nil
	}
}

func prefixToken(decoder *json.Decoder, raw string) (json.Token, error) {
	token, err := decoder.Token()
	if err != nil && strings.TrimSpace(raw[decoder.InputOffset():]) == "" {
		return nil, io.ErrUnexpectedEOF
	}
	return token, err
}
