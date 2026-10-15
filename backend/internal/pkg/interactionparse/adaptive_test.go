package interactionparse

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func hasWarning(d Document, warning string) bool {
	for _, w := range d.Warnings {
		if strings.SplitN(w, ":", 2)[0] == warning {
			return true
		}
	}
	return false
}

func TestTruncatedJSONRecoversOnlyCompletePrefixItems(t *testing.T) {
	cases := []struct {
		name, raw, protocol string
		response            bool
	}{
		{"responses", `{"instructions":"retained system","input":[{"role":"user","content":"retained message"},{"role":"user","content":"unfinished`, "responses", false},
		{"chat", `{"messages":[{"role":"user","content":"retained message"},{"role":"assistant","content":"unfinished`, "chat_completions", false},
		{"anthropic", `{"system":"retained system","messages":[{"role":"user","content":[{"type":"text","text":"retained message"}]},{"role":"user","content":[{"type":"text","text":"unfinished`, "anthropic", false},
		{"gemini", `{"contents":[{"role":"user","parts":[{"text":"retained message"}]},{"role":"user","parts":[{"text":"unfinished`, "gemini", false},
		{"response", `{"object":"response","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"retained message"}]},{"type":"function_call","arguments":"unfinished`, "responses", true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			d := Parse(map[string]any{"raw_text": tt.raw, "truncated": true}, tt.response)
			if d.Protocol != tt.protocol || d.Format != "json_prefix" || d.Ended || !hasWarning(d, "truncated") || !hasWarning(d, "recovered_prefix") {
				t.Fatalf("unexpected recovered document: %+v", d)
			}
			found := false
			for _, b := range d.Blocks {
				if b.Text == "retained message" {
					found = true
				}
				if strings.Contains(b.Text, "unfinished") || b.Status != "partial" {
					t.Fatalf("incomplete content was invented or not marked: %+v", b)
				}
			}
			if !found {
				t.Fatal("complete message prefix was lost")
			}
		})
	}
}

func TestPrefixRecoveryAtEveryByteBoundary(t *testing.T) {
	// JSON encoding also exercises escapes and multi-byte text at cut boundaries.
	firstBytes, _ := json.Marshal(map[string]any{"role": "user", "content": "first \"quoted\" message"})
	first := string(firstBytes)
	secondBytes, _ := json.Marshal(map[string]any{"role": "user", "content": "second \u4e2d\u6587 message"})
	prefix := `{"input":[`
	raw := prefix + first + "," + string(secondBytes) + "]}"
	for end := len(prefix); end < len(raw); end++ {
		d := Parse(map[string]any{"raw_text": raw[:end], "truncated": true}, false)
		text := blocks(d, "text")
		want := 0
		if end >= len(prefix)+len(first) {
			want = 1
		}
		if end >= len(raw)-2 {
			want = 2
		}
		if len(text) != want {
			t.Fatalf("cut at %d: got %d complete messages, want %d", end, len(text), want)
		}
	}
}

func TestPrefixRecoveryPreservesReportedItemStatus(t *testing.T) {
	d := Parse(map[string]any{"raw_text": `{"input":[{"type":"function_call","name":"read","call_id":"c","arguments":"{}","status":"completed"},{"content":"unfinished`, "truncated": true}, false)
	if len(blocks(d, "tool_call")) != 1 || blocks(d, "tool_call")[0].Status != "completed" || !hasWarning(d, "recovered_prefix") {
		t.Fatal("partial capture replaced the explicit status of an already complete item")
	}
}

func TestBrokenJSONIsDiagnosedWithoutSkippingPastCorruption(t *testing.T) {
	cases := []string{
		`{"input":[{"role":"user","content":"safe"},BROKEN,{"role":"user","content":"must not recover"}]}`,
		`{"input":[{"role":"user","content":"safe"},{"role":"user","content":"bad\q"}]}`,
		"{\n\"input\": [\n{\"role\":\"user\",\"content\":\"safe\"},\nBROKEN,\n{\"role\":\"user\",\"content\":\"must not recover\"}\n]}\n",
	}
	for _, raw := range cases {
		d := Parse(map[string]any{"raw_text": raw}, false)
		text := blocks(d, "text")
		if len(text) != 1 || text[0].Text != "safe" || !hasWarning(d, "invalid_json") || !hasWarning(d, "recovered_prefix") {
			t.Fatalf("unexpected corruption recovery: %+v", d)
		}
	}
	d := Parse(map[string]any{"raw_text": `{"input":[{"role":"user","content":"unfinished`}, false)
	if len(d.Blocks) != 0 || !hasWarning(d, "incomplete_json") || hasWarning(d, "recovered_prefix") {
		t.Fatalf("expected an empty, diagnosed incomplete capture: %+v", d)
	}
}

func TestPrettyPrintedPrefixIsNotMistakenForNDJSON(t *testing.T) {
	raw := "{\n\"input\": [\n{\"role\":\"user\",\"content\":\"retained message\"}\n"
	d := Parse(map[string]any{"raw_text": raw, "truncated": true}, false)
	if d.Protocol != "responses" || d.Format != "json_prefix" || len(blocks(d, "text")) != 1 || blocks(d, "text")[0].Source != "$.raw_text.input[0].content" {
		_, err := recoverJSONPrefix(raw)
		t.Fatalf("pretty printed prefix lost its envelope: %+v; prefix error: %T %v", d, err, err)
	}
}

func TestUnknownResponseItemsRetainTheirIndexedOrder(t *testing.T) {
	d := Parse(payload(t, `{"output":[{"type":"future_message","content":"first"},{"type":"message","role":"assistant","content":[{"type":"future_part","text":"second"},{"type":"output_text","text":"third"}]}],"status":"completed"}`), true)
	if len(d.Blocks) != 3 || d.Blocks[0].Kind != "unknown" || d.Blocks[1].Kind != "unknown" || d.Blocks[2].Text != "third" {
		t.Fatalf("unknown items were moved out of source order: %+v", d)
	}
}

func TestRecoveredSourceAndChildrenRemainRedacted(t *testing.T) {
	m := map[string]any{"raw_text": `{"password":"capture-secret","input":[{"type":"function_call_output","call_id":"c","output":[{"type":"text","text":"{\"password\":\"capture-secret\",\"safe\":1}"},{"type":"image","source":{"type":"base64","data":"binary"}}]},{"content":"unfinished-secret`, "truncated": true}
	d := Parse(m, false)
	secret := func(k string) bool { return k == "password" }
	RedactDocument(&d, secret)
	for _, v := range []any{d, Redact(m, secret)} {
		encoded, err := json.Marshal(v)
		if err != nil || strings.Contains(string(encoded), "capture-secret") || strings.Contains(string(encoded), "unfinished-secret") {
			t.Fatalf("recovered source leaked incomplete or secret data: %s, %v", encoded, err)
		}
	}
	if m["raw_text"] == Redact(m, secret).(map[string]any)["raw_text"] {
		t.Fatal("source representation was not sanitized")
	}
}

func TestResponsesExtensionItemsAndHeartbeats(t *testing.T) {
	d := Parse(payload(t, `{"input":[
		{"type":"additional_tools","role":"developer","tools":[{"type":"function","name":"read"}]},
		{"type":"agent_message","author":"/root","recipient":"/root/helper","content":[{"type":"input_text","text":"retained task"},{"type":"encrypted_content","encrypted_content":"opaque-secret"}]},
		{"type":"compaction","id":"cmp_1","encrypted_content":"opaque-secret"}
	]}`), false)
	definitions := blocks(d, "tool_definitions")
	agents := blocks(d, "agent_message")
	compactions := blocks(d, "compaction")
	if len(definitions) != 1 || definitions[0].Role != "developer" || len(agents) != 1 || len(compactions) != 1 || len(blocks(d, "unknown")) != 0 {
		t.Fatalf("extensions were not classified: %+v", d)
	}
	encoded, _ := json.Marshal(d)
	if !strings.Contains(string(encoded), "retained task") || strings.Contains(string(encoded), "opaque-secret") || !strings.Contains(string(encoded), "/root/helper") {
		t.Fatalf("agent routing or opaque handling is incorrect: %s", encoded)
	}
	d = Parse(eventCapture(`{"type":"keepalive"}`, `{"type":"heartbeat"}`, `{"type":"ping"}`, `{"type":"response.completed","response":{}}`), true)
	if len(d.Blocks) != 0 || len(d.Warnings) != 0 || !d.Ended {
		t.Fatalf("heartbeats polluted the response: %+v", d)
	}
}

func TestMixedToolResultsAreStructuredAcrossProtocols(t *testing.T) {
	cases := []string{
		`{"input":[{"type":"function_call_output","call_id":"c","output":[{"type":"text","text":"readable result"},{"type":"image_url","image_url":{"url":"data:image/png;base64,BINARY"}}]}]}`,
		`{"messages":[{"role":"tool","tool_call_id":"c","content":[{"type":"text","text":"readable result"},{"type":"image","source":{"data":"BINARY"}}]}]}`,
		`{"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"c","content":[{"type":"text","text":"readable result"},{"type":"image","source":{"data":"BINARY"}}]}]}]}`,
		`{"contents":[{"role":"user","parts":[{"functionResponse":{"id":"c","name":"read","response":{"content":[{"type":"text","text":"readable result"},{"type":"image","source":{"data":"BINARY"}}]}}}]}]}`,
	}
	for _, raw := range cases {
		d := Parse(payload(t, raw), false)
		results := blocks(d, "tool_result")
		if len(results) != 1 {
			t.Fatalf("missing tool result: %+v", d)
		}
		encoded, _ := json.Marshal(results[0])
		var result struct{ Children []Block }
		_ = json.Unmarshal(encoded, &result)
		if len(result.Children) != 2 || result.Children[0].Kind != "text" || result.Children[0].Text != "readable result" || result.Children[1].Kind != "attachment" || strings.Contains(string(encoded), "BINARY") {
			t.Fatalf("mixed results are not safe content blocks: %s", encoded)
		}
	}
}

func TestEncodedAndNestedToolResultsRemainSafe(t *testing.T) {
	d := Parse(payload(t, `{"input":[{"type":"function_call_output","call_id":"c","output":"[{\"type\":\"text\",\"text\":\"encoded text\"},{\"type\":\"image\",\"source\":{\"data\":\"BINARY\"}}]"},{"type":"function_call_output","call_id":"d","output":[{"type":"tool_result","tool_use_id":"nested","content":[{"type":"image","source":{"data":"BINARY"}}]}]}]}`), false)
	encoded, _ := json.Marshal(d)
	if strings.Contains(string(encoded), "BINARY") || !strings.Contains(string(encoded), "encoded text") {
		t.Fatalf("nested or encoded media was not compacted: %s", encoded)
	}
	first := blocks(d, "tool_result")[0]
	if len(first.Children) != 2 || first.Children[0].Source != "$.input[0].output[0]" {
		t.Fatalf("encoded tool result lost its source path: %+v", first)
	}
}

func TestToolResultTextIsNotReformattedAsEmbeddedJSON(t *testing.T) {
	text := "{\n  \"second\": 2,\n  \"first\": 1\n}"
	output := []any{map[string]any{"type": "text", "text": text}}
	d := Parse(map[string]any{"input": []any{
		map[string]any{"type": "function_call_output", "call_id": "c", "output": output},
		map[string]any{"type": "function_call_output", "call_id": "d", "output": text},
	}}, false)
	results := blocks(d, "tool_result")
	if obj(arr(results[0].Data)[0])["text"] != text || results[0].Children[0].Text != text || results[1].Data != text {
		t.Fatal("readable tool output was reformatted as JSON instead of being preserved as text")
	}
}

func TestRedactionPreservesUnchangedJSONText(t *testing.T) {
	text := "{\n  \"second\": 2,\n  \"first\": 1\n}"
	if got := Redact(text, func(string) bool { return false }); got != text {
		t.Fatal("redaction reformatted text that did not require changes")
	}
	fragment := `{"type":"response.function_call_arguments.delta","delta":"unfinished-secret"}`
	got := Redact(fragment, func(string) bool { return false })
	if strings.Contains(str(got), "unfinished-secret") {
		t.Fatal("preserving formatting bypassed fragment withholding")
	}
}

func TestUnknownReadableFieldsArePreservedWithoutGuessingProtocol(t *testing.T) {
	d := Parse(payload(t, `{"input":["bare message",{"type":"message","role":"user","content":{"type":"text","text":"singleton part"}},{"type":"future_message","role":"developer","content":[{"type":"future_part","text":"readable extension","confidence":0.9}],"new_field":42}]}`), false)
	encoded, _ := json.Marshal(d)
	for _, retained := range []string{"bare message", "singleton part", "readable extension", "confidence", "new_field"} {
		if !strings.Contains(string(encoded), retained) {
			t.Fatalf("extension content was silently lost: %s", retained)
		}
	}
	if !hasWarning(d, "unknown_content") {
		t.Fatal("unknown interpretation should remain explicit")
	}
	d = Parse(payload(t, `{"type":"future_envelope","content":"readable body","signature":"opaque-secret"}`), true)
	encoded, _ = json.Marshal(d)
	if d.Protocol != "unknown" || !strings.Contains(string(encoded), "readable body") || strings.Contains(string(encoded), "opaque-secret") {
		t.Fatalf("fallback guessed a protocol or exposed opaque content: %s", encoded)
	}
}

func TestUnexpectedCollectionShapesAreNotSilentlyDiscarded(t *testing.T) {
	for _, raw := range []string{
		`{"input":true}`,
		`{"messages":"unexpected messages"}`,
		`{"choices":42}`,
		`{"contents":false}`,
		`{"output":"unexpected output"}`,
		`{"input":[],"tools":{"type":"future_tool","name":"retained"}}`,
	} {
		d := Parse(payload(t, raw), false)
		if len(blocks(d, "unknown")) == 0 || !hasWarning(d, "unknown_content") {
			t.Fatalf("unexpected fields were silently discarded: %s, %+v", raw, d)
		}
	}
}

func FuzzCapturedContent(f *testing.F) {
	for _, seed := range []string{
		`{"input":[{"role":"user","content":"hello"}]}`,
		`{"input":[{"role":"user","content":"hello"},{"content":"cut`,
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\ndata: [DONE]\n\n",
		`{"choices":[null,{},5]}`,
		`{"type":"future","text":"hello"}`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		capture := map[string]any{"raw_text": raw, "truncated": true}
		for _, response := range []bool{false, true} {
			d := Parse(capture, response)
			if !reflect.DeepEqual(d, Parse(capture, response)) {
				t.Fatal("capture parsing is nondeterministic")
			}
			RedactDocument(&d, func(k string) bool { return k == "password" })
			if _, err := json.Marshal(d); err != nil {
				t.Fatal(err)
			}
		}
	})
}
