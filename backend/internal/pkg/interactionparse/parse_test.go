package interactionparse

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func payload(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	return m
}
func blocks(d Document, kind string) []*Block {
	var out []*Block
	for _, b := range d.Blocks {
		if b.Kind == kind {
			out = append(out, b)
		}
	}
	return out
}
func eventCapture(events ...string) map[string]any {
	return map[string]any{"raw_text": strings.Join(events, "\n")}
}

func TestResponsesSnapshotsReplaceDeltas(t *testing.T) {
	d := Parse(eventCapture(
		`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"item1","call_id":"call1","name":"read","arguments":""}}`,
		`{"type":"response.function_call_arguments.delta","item_id":"item1","delta":"{\"path\":"}`,
		`{"type":"response.function_call_arguments.delta","item_id":"item1","delta":"\"a\"}"}`,
		`{"type":"response.function_call_arguments.done","item_id":"item1","arguments":"{\"path\":\"a\"}"}`,
		`{"type":"response.completed","response":{"status":"completed","output":[{"type":"function_call","id":"item1","call_id":"call1","name":"read","arguments":"{\"path\":\"a\"}"}]}}`,
	), true)
	calls := blocks(d, "tool_call")
	if len(calls) != 1 || obj(calls[0].Data)["path"] != "a" || calls[0].CallID != "call1" || !d.Ended {
		t.Fatalf("%+v %+v", d, calls)
	}
}

func TestResponsesTextAndReasoning(t *testing.T) {
	d := Parse(eventCapture(
		`{"type":"response.output_text.delta","output_index":1,"content_index":0,"delta":"hello"}`,
		`{"type":"response.output_text.delta","output_index":1,"content_index":0,"delta":"hello"}`,
		`{"type":"response.output_text.done","output_index":1,"content_index":0,"text":"hellohello"}`,
		`{"type":"response.reasoning_summary_text.delta","output_index":0,"summary_index":0,"delta":"summary"}`,
		`{"type":"response.completed","response":{"output":[{"type":"reasoning","summary":[{"text":"summary"}],"encrypted_content":"secret blob"},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hellohello"}]}]}}`,
	), true)
	if len(blocks(d, "text")) != 1 || blocks(d, "text")[0].Text != "hellohello" || len(blocks(d, "reasoning")) != 1 || blocks(d, "reasoning")[0].Text != "summary" || len(blocks(d, "opaque")) != 1 {
		t.Fatalf("%+v", d)
	}
}

func TestChatParallelToolsAndChoices(t *testing.T) {
	d := Parse(eventCapture(
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"a","function":{"name":"first","arguments":"{\"x\":"}},{"index":1,"id":"b","function":{"name":"second","arguments":"{\"y\":"}}]}},{"index":1,"delta":{"content":"other"}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"function":{"arguments":"2}"}},{"index":0,"function":{"arguments":"1}"}}]},"finish_reason":"tool_calls"}]}`,
	), true)
	c := blocks(d, "tool_call")
	if len(c) != 2 || obj(c[0].Data)["x"] != float64(1) || obj(c[1].Data)["y"] != float64(2) || blocks(d, "text")[0].Group != "choice/1" {
		t.Fatalf("%+v", d)
	}
}

func TestAnthropicSSE(t *testing.T) {
	d := Parse(map[string]any{"raw_text": "event: content_block_start\r\ndata: {\"index\":2,\"content_block\":{\"type\":\"tool_use\",\"id\":\"c\",\"name\":\"read\",\"input\":{}}}\r\n\r\n" +
		"event: content_block_delta\ndata: {\"index\":2,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"path\\\":\\\"a\\\"}\"}}\n\n" +
		"event: message_stop\ndata: {}\n\n"}, true)
	c := blocks(d, "tool_call")
	if d.Protocol != "anthropic" || !d.Ended || len(c) != 1 || obj(c[0].Data)["path"] != "a" {
		t.Fatalf("%+v", d)
	}
}

func TestSSEMultilineCommentsAndBrokenTail(t *testing.T) {
	d := Parse(map[string]any{"raw_text": ": heartbeat\n\nevent: response.output_text.delta\ndata: {\"delta\":\n data:ignored\ndata: \"ok\"}\n\ndata: {broken", "truncated": true}, true)
	if len(blocks(d, "text")) != 1 || blocks(d, "text")[0].Text != "ok" || len(d.Warnings) < 2 || d.Ended {
		t.Fatalf("%+v", d)
	}
}

func TestGeminiStreamAndSafety(t *testing.T) {
	d := Parse(eventCapture(
		`{"candidates":[{"index":0,"content":{"parts":[{"text":"think","thought":true}]}}]}`,
		`{"candidates":[{"index":0,"content":{"parts":[{"text":"hello"}]}},{"index":1,"content":{"parts":[{"functionCall":{"id":"x","name":"search","args":{"q":"a"}}}]}}]}`,
		`{"candidates":[{"index":0,"content":{"parts":[{"text":" world"}]},"finishReason":"STOP"},{"index":1,"finishReason":"STOP"}]}`,
	), true)
	text := ""
	for _, b := range blocks(d, "text") {
		text += b.Text
	}
	if text != "hello world" || len(blocks(d, "reasoning")) != 1 || len(blocks(d, "tool_call")) != 1 || !d.Ended {
		t.Fatalf("%+v", d)
	}
	d = Parse(payload(t, `{"promptFeedback":{"blockReason":"SAFETY"}}`), true)
	if !d.Ended || len(d.Warnings) != 1 {
		t.Fatalf("%+v", d)
	}
}

func TestRequestProtocols(t *testing.T) {
	cases := []struct {
		raw, kind string
		count     int
	}{
		{`{"instructions":"system","input":[{"role":"user","content":"hi"},{"type":"function_call_output","call_id":"c","output":"done"}],"previous_response_id":"prev","tools":[{"name":"f"}]}`, "tool_result", 1},
		{`{"messages":[{"role":"user","content":"hi"},{"role":"assistant","tool_calls":[{"id":"c","function":{"name":"f","arguments":"{}"}}]},{"role":"tool","tool_call_id":"c","content":"done"}]}`, "tool_call", 1},
		{`{"system":[{"type":"text","text":"system"}],"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"c","content":"done"}]}]}`, "tool_result", 1},
		{`{"systemInstruction":{"parts":[{"text":"system"}]},"contents":[{"role":"user","parts":[{"functionResponse":{"name":"f","response":{"result":"ok"}}}]}]}`, "tool_result", 1},
	}
	for _, tt := range cases {
		d := Parse(payload(t, tt.raw), false)
		if len(blocks(d, tt.kind)) != tt.count {
			t.Fatalf("%s: %+v", tt.raw, d)
		}
	}
}

func TestUnknownAndAttachments(t *testing.T) {
	d := Parse(payload(t, `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}},{"type":"future_part","value":42}]}]}`), false)
	if len(blocks(d, "attachment")) != 1 || len(blocks(d, "unknown")) != 1 {
		t.Fatalf("%+v", d)
	}
	encoded, _ := json.Marshal(d)
	if strings.Contains(string(encoded), "AAAA") {
		t.Fatal("binary leaked")
	}
}

func TestRedactsEncodedArgumentsAndSources(t *testing.T) {
	secret := func(k string) bool { return k == "password" }
	m := eventCapture(`{"type":"response.function_call_arguments.delta","delta":"{\"password\":\"secret\",\"path\":\"a\"}"}`, `{"type":"response.completed","response":{}}`)
	d := Parse(m, true)
	RedactDocument(&d, secret)
	for _, v := range []any{d, Redact(m, secret), Redact(map[string]any{"arguments": `{"password":"secret"}`}, secret)} {
		encoded, _ := json.Marshal(v)
		if strings.Contains(string(encoded), "secret") {
			t.Fatalf("secret leaked: %s", encoded)
		}
	}
	if obj(blocks(d, "tool_call")[0].Data)["path"] != "a" {
		t.Fatal("lost non-secret arguments")
	}
}

func TestIncompleteArgumentsAreNotInvented(t *testing.T) {
	d := Parse(eventCapture(`{"type":"response.function_call_arguments.delta","delta":"{\"password\":\"oops"}`), true)
	c := blocks(d, "tool_call")[0]
	if c.Status != "invalid_arguments" || d.Ended {
		t.Fatalf("%+v", d)
	}
	RedactDocument(&d, func(k string) bool { return k == "password" })
	if strings.Contains(c.Text, "oops") {
		t.Fatal("incomplete secret leaked")
	}
}

func TestAdditionalContentShapes(t *testing.T) {
	cases := []struct{ raw, kind, text string }{
		{`{"type":"message","role":"assistant","content":[{"type":"text","text":"answer"},{"type":"thinking","thinking":"summary"},{"type":"redacted_thinking","data":"hidden"}],"stop_reason":"end_turn"}`, "reasoning", "summary"},
		{`{"object":"response","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"message","content":[{"type":"refusal","refusal":"no"}]}]}`, "refusal", "no"},
		{`{"input":[{"type":"custom_tool_call","call_id":"c","name":"patch","input":"patch body"},{"type":"custom_tool_call_output","call_id":"c","output":"ok"},{"type":"item_reference","id":"previous"}]}`, "custom_tool_call", "patch body"},
		{`{"choices":[{"index":0,"message":{"content":"answer","reasoning_content":"thought","refusal":"no","function_call":{"name":"old","arguments":"{}"}},"finish_reason":"stop"}]}`, "reasoning", "thought"},
		{`{"error":{"message":"provider failed"}}`, "error", ""},
		{`{"raw_text":"plain response"}`, "text", "plain response"},
		{`{"input":[{"type":"web_search_call","id":"s","status":"completed","action":{"query":"q"}}]}`, "tool_call", ""},
	}
	for _, tt := range cases {
		d := Parse(payload(t, tt.raw), true)
		b := blocks(d, tt.kind)
		if len(b) != 1 || b[0].Text != tt.text {
			t.Fatalf("%s: %+v", tt.raw, d)
		}
	}
}

func TestAnthropicThinkingAndInvalidInput(t *testing.T) {
	d := Parse(eventCapture(
		`{"type":"message_start","message":{"content":[]}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"summary"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"opaque"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"answer"}}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
		`{"type":"message_stop"}`,
	), true)
	if len(blocks(d, "opaque")) != 1 || blocks(d, "reasoning")[0].Text != "summary" || blocks(d, "text")[0].Text != "answer" || !d.Ended {
		t.Fatalf("%+v", d)
	}
}

func TestGeminiJSONArrayDoesNotOverwriteChunks(t *testing.T) {
	d := Parse(payload(t, `{"raw_json":[{"candidates":[{"content":{"parts":[{"text":"hello"}]}}]},{"candidates":[{"content":{"parts":[{"text":" world"}]},"finishReason":"STOP"}]}]}`), true)
	if len(blocks(d, "text")) != 1 || blocks(d, "text")[0].Text != "hello world" {
		t.Fatalf("%+v", d)
	}
}

func TestOpaqueMetadataAndEncodedRedaction(t *testing.T) {
	d := Parse(payload(t, `{"contents":[{"role":"model","parts":[{"inlineData":{"mimeType":"image/png","data":"binary"}},{"fileData":{"fileUri":"https://example.test/a","mimeType":"audio/wav"}},{"executableCode":{"code":"print(1)"}},{"text":"answer","thoughtSignature":"sig"}]}]}`), false)
	if len(blocks(d, "attachment")) != 2 || len(blocks(d, "opaque")) != 1 {
		t.Fatalf("%+v", d)
	}
	d = Parse(payload(t, `{"choices":[{"message":{"content":"{\"password\":\"secret\"}"}}]}`), true)
	RedactDocument(&d, func(k string) bool { return k == "password" })
	if strings.Contains(blocks(d, "text")[0].Text, "secret") {
		t.Fatal("encoded text secret leaked")
	}
}

func TestOneFinishedChoiceDoesNotHideIncompleteOtherChoice(t *testing.T) {
	d := Parse(eventCapture(
		`{"choices":[{"index":0,"delta":{"content":"done"},"finish_reason":"stop"},{"index":1,"delta":{"content":"partial"}}]}`,
		`{"choices":[]}`,
	), true)
	if d.Ended || len(d.Warnings) == 0 {
		t.Fatalf("%+v", d)
	}
}

func TestResponsePartsFollowIndicesNotArrivalOrder(t *testing.T) {
	d := Parse(eventCapture(
		`{"type":"response.output_text.delta","output_index":0,"content_index":1,"delta":"second"}`,
		`{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"first"}`,
		`{"type":"response.completed","response":{}}`,
	), true)
	b := blocks(d, "text")
	if len(b) != 2 || b[0].Text != "first" || b[1].Text != "second" {
		t.Fatalf("%+v", d)
	}
}

func TestGeminiStreamRetainsEachAttachment(t *testing.T) {
	const imageEvent = `{"candidates":[{"index":0,"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"AAAA"}}]}}]}`
	events := []string{
		imageEvent,
		`{"candidates":[{"index":0,"content":{"parts":[{"text":"Between images"}]}},{"index":1,"content":{"parts":[{"inlineData":{"mimeType":"image/jpeg","data":"BBBBBB"}}]},"finishReason":"STOP"}]}`,
		imageEvent,
		`{"candidates":[{"index":0,"content":{"parts":[{"fileData":{"mimeType":"image/webp","fileUri":"https://example.test/final.webp"}},{"text":"After images"}]},"finishReason":"STOP"}]}`,
	}
	for _, format := range []string{"sse", "ndjson", "json_array"} {
		t.Run(format, func(t *testing.T) {
			raw := strings.Join(events, "\n")
			if format == "sse" {
				raw = "data: " + strings.Join(events, "\n\ndata: ") + "\n\n"
			} else if format == "json_array" {
				raw = "[" + strings.Join(events, ",") + "]"
			}
			d := Parse(map[string]any{"raw_text": raw}, true)
			if d.Protocol != "gemini" || d.Format != format || !d.Ended || len(d.Warnings) != 0 {
				t.Fatalf("unexpected document: %+v", d)
			}
			attachments := blocks(d, "attachment")
			if len(attachments) != 4 {
				t.Fatalf("want 4 attachments, got %d: %+v", len(attachments), d)
			}
			for i, want := range []struct {
				field, mime, group string
				event, candidate   int
			}{
				{"inlineData", "image/png", "candidate/0", 0, 0},
				{"inlineData", "image/jpeg", "candidate/1", 1, 1},
				{"inlineData", "image/png", "candidate/0", 2, 0},
				{"fileData", "image/webp", "candidate/0", 3, 0},
			} {
				b := attachments[i]
				source := fmt.Sprintf("$.raw_text#%d", want.event)
				if format == "json_array" {
					source = fmt.Sprintf("$.raw_text[%d]", want.event)
				}
				source += fmt.Sprintf(".candidates[%d].parts[0]", want.candidate)
				if obj(obj(b.Data)[want.field])["mimeType"] != want.mime || b.Source != source || b.Group != want.group {
					t.Fatalf("attachment %d has wrong metadata or provenance: %+v", i, b)
				}
			}
			var sequence []string
			for _, b := range d.Blocks {
				if b.Kind != "metadata" {
					sequence = append(sequence, b.Kind+":"+b.Text)
				}
			}
			if strings.Join(sequence, "|") != "attachment:|text:Between images|attachment:|attachment:|attachment:|text:After images" {
				t.Fatalf("mixed content reordered: %v", sequence)
			}
			encoded, _ := json.Marshal(d)
			if strings.Contains(string(encoded), "AAAA") || strings.Contains(string(encoded), "BBBBBB") {
				t.Fatal("binary data leaked")
			}
		})
	}
}

func TestFailedResponsesKeepReadableOutput(t *testing.T) {
	raw := `{"object":"response","status":"failed","error":{"code":"server_error","message":"Failed after partial output"},"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Retained answer"}]},{"type":"function_call","name":"read","call_id":"c","arguments":"{\"path\":\"a\"}"}]}`
	withoutObject := payload(t, raw)
	delete(withoutObject, "object")
	for name, capture := range map[string]map[string]any{
		"direct":         payload(t, raw),
		"without_object": withoutObject,
		"raw_json":       {"raw_json": payload(t, raw)},
		"raw_text":       {"raw_text": raw},
		"failed_event":   eventCapture(`{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"obsolete delta"}`, `{"type":"response.failed","response":`+raw+`}`),
	} {
		t.Run(name, func(t *testing.T) {
			d := Parse(capture, true)
			if d.Protocol != "responses" || !d.Ended || !hasWarning(d, "failed") {
				t.Fatalf("unexpected failure state: %+v", d)
			}
			texts, calls, failures := blocks(d, "text"), blocks(d, "tool_call"), blocks(d, "error")
			if len(texts) != 1 || texts[0].Text != "Retained answer" || texts[0].Role != "assistant" {
				t.Fatalf("missing readable answer: %+v", texts)
			}
			if len(calls) != 1 || calls[0].CallID != "c" || obj(calls[0].Data)["path"] != "a" {
				t.Fatalf("missing retained tool call: %+v", calls)
			}
			if len(failures) != 1 || obj(failures[0].Data)["code"] != "server_error" {
				t.Fatalf("missing provider error: %+v", failures)
			}
		})
	}
}

func TestStandaloneErrorsRemainErrors(t *testing.T) {
	for _, raw := range []string{
		`{"error":{"code":"server_error","message":"failed"}}`,
		`{"type":"error","error":{"message":"failed"},"output":[]}`,
	} {
		d := Parse(payload(t, raw), true)
		if len(d.Blocks) != 1 || d.Blocks[0].Kind != "error" || !d.Ended || !hasWarning(d, "failed") {
			t.Fatalf("unexpected standalone error: %+v", d)
		}
	}
}
