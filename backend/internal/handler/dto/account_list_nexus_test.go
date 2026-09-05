package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountListPreservesGroupLabelsWithoutGroupManagementAccess(t *testing.T) {
	account := &Account{ID: 42, GroupIDs: []int64{7}, Groups: []*Group{
		{ID: 7, Name: "codex", Platform: "openai", SubscriptionType: "standard", RateMultiplier: 1.5},
	}}
	raw, err := json.Marshal(AccountListItemFromAccount(account))
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))
	require.NotContains(t, fields, "groups")
	require.NotContains(t, fields, "account_groups")
	require.Equal(t, []any{map[string]any{
		"id": float64(7), "name": "codex", "platform": "openai",
		"subscription_type": "standard", "rate_multiplier": 1.5,
	}}, fields["group_summaries"])
}
