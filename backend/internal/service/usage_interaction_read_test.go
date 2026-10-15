package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type interactionReadRepository struct {
	usageInteractionTestRepository
	record     *UsageInteraction
	includeRaw bool
}

func (r *interactionReadRepository) GetByUsageLogID(_ context.Context, _ int64, raw bool) (*UsageInteraction, error) {
	r.includeRaw = raw
	return r.record, nil
}

func TestUsageInteractionReadReconstructsAndRedactsWithoutRaw(t *testing.T) {
	repo := &interactionReadRepository{record: &UsageInteraction{
		RequestContent: map[string]any{"messages": []any{map[string]any{"role": "user", "content": "question"}}},
		ResponseContent: map[string]any{"raw_text": strings.Join([]string{
			`{"type":"response.function_call_arguments.delta","delta":"{\"password\":\"captured-secret\",\"path\":\"a\"}"}`,
			`{"type":"response.completed","response":{"status":"completed"}}`,
		}, "\n")},
		RawRequestJSON:  map[string]any{"private": "privileged-only"},
		RawResponseJSON: map[string]any{"private": "privileged-only"},
	}}
	svc := NewUsageInteractionService(repo, nil)
	got, err := svc.GetByUsageLogID(context.Background(), 1, false)
	require.NoError(t, err)
	require.False(t, repo.includeRaw)
	require.Nil(t, got.RawRequestJSON)
	require.True(t, got.RedactionApplied)
	require.Contains(t, got.RedactionKeys, "password")
	require.Equal(t, "question", got.ParsedRequest.Blocks[0].Text)
	require.Equal(t, "a", got.ParsedResponse.Blocks[0].Data.(map[string]any)["path"])
	data, err := json.Marshal(got)
	require.NoError(t, err)
	require.NotContains(t, string(data), "captured-secret")
	require.NotContains(t, string(data), "privileged-only")
	require.Contains(t, repo.record.ResponseContent["raw_text"], "captured-secret", "reading must not mutate repository state")
	got, err = svc.GetByUsageLogID(context.Background(), 1, true)
	require.NoError(t, err)
	require.True(t, repo.includeRaw)
	require.Equal(t, "privileged-only", got.RawRequestJSON["private"])
	require.Equal(t, "question", got.ParsedRequest.Blocks[0].Text)
}

func TestUsageInteractionReadRedactsRecoveredPrefixesAndNestedContent(t *testing.T) {
	capture := map[string]any{"truncated": true, "raw_text": `{"password":"capture-secret","input":[{"type":"function_call_output","call_id":"c","output":[{"type":"text","text":"{\"password\":\"capture-secret\",\"safe\":1}"},{"type":"image","source":{"data":"BINARY"}}]},{"content":"unfinished-secret`}
	repo := &interactionReadRepository{record: &UsageInteraction{
		RequestContent: capture,
		RawRequestJSON: capture,
	}}
	before, err := json.Marshal(repo.record)
	require.NoError(t, err)
	svc := NewUsageInteractionService(repo, nil)
	for _, raw := range []bool{false, true} {
		got, err := svc.GetByUsageLogID(context.Background(), 1, raw)
		require.NoError(t, err)
		require.Equal(t, "json_prefix", got.ParsedRequest.Format)
		require.Contains(t, got.ParsedRequest.Warnings, "recovered_prefix")
		require.True(t, got.RedactionApplied)
		require.Contains(t, got.RedactionKeys, "password")
		require.Len(t, got.ParsedRequest.Blocks[0].Children, 2)
		require.Equal(t, "partial", got.ParsedRequest.Blocks[0].Children[0].Status)
		encoded, err := json.Marshal(got)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), "capture-secret")
		require.NotContains(t, string(encoded), "unfinished-secret")
		if !raw {
			require.Nil(t, got.RawRequestJSON)
		}
	}
	after, err := json.Marshal(repo.record)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after), "read derivation must not mutate the stored capture")
}

func TestUsageInteractionReadKeepsFailedResponseOutputRedacted(t *testing.T) {
	repo := &interactionReadRepository{record: &UsageInteraction{
		ResponseContent: map[string]any{"raw_text": `{"object":"response","status":"failed","error":{"code":"server_error","password":"error-secret"},"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Retained answer"}]},{"type":"function_call","name":"read","call_id":"c","arguments":"{\"path\":\"a\",\"password\":\"argument-secret\"}"}]}`},
	}}
	before, err := json.Marshal(repo.record)
	require.NoError(t, err)
	got, err := NewUsageInteractionService(repo, nil).GetByUsageLogID(context.Background(), 1, false)
	require.NoError(t, err)
	require.Equal(t, "responses", got.ParsedResponse.Protocol)
	require.True(t, got.ParsedResponse.Ended)
	require.Contains(t, got.ParsedResponse.Warnings, "failed")
	require.Len(t, got.ParsedResponse.Blocks, 3)
	require.Equal(t, "text", got.ParsedResponse.Blocks[0].Kind)
	require.Equal(t, "Retained answer", got.ParsedResponse.Blocks[0].Text)
	require.Equal(t, "tool_call", got.ParsedResponse.Blocks[1].Kind)
	require.Equal(t, "a", got.ParsedResponse.Blocks[1].Data.(map[string]any)["path"])
	require.Equal(t, "error", got.ParsedResponse.Blocks[2].Kind)
	require.True(t, got.RedactionApplied)
	require.Nil(t, got.RawResponseJSON)
	encoded, err := json.Marshal(got)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "error-secret")
	require.NotContains(t, string(encoded), "argument-secret")
	after, err := json.Marshal(repo.record)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))
}
