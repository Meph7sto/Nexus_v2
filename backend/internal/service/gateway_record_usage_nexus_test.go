//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnifiedRecordUsagePreservesNexusInteractionAndSingleBilling(t *testing.T) {
	for _, tc := range []struct {
		name       string
		recording  bool
		storageErr error
	}{
		{"recording enabled", true, nil},
		{"recording disabled", false, nil},
		{"interaction storage unavailable", true, errors.New("storage unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usageRepo := &usageInteractionTestUsageLogRepository{nextID: 77}
			userRepo := &openAIRecordUsageUserRepoStub{}
			interactionRepo := &usageInteractionTestRepository{createErr: tc.storageErr}
			svc := newGatewayRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{})
			svc.SetUsageInteractionService(NewUsageInteractionService(interactionRepo,
				&usageInteractionTestSettingRepository{values: usageInteractionTestSettings(tc.recording, false, "7")}))
			err := svc.RecordUsage(context.Background(), &RecordUsageInput{
				Result: &ForwardResult{RequestID: "nexus-unified-billing", Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 100, OutputTokens: 20}},
				APIKey: &APIKey{ID: 3}, User: &User{ID: 2}, Account: &Account{ID: 4},
				Interaction: &UsageInteractionCapture{RequestContent: map[string]any{"prompt": "hello"}},
			})
			require.NoError(t, err)
			require.Equal(t, 1, userRepo.deductCalls)
			require.Equal(t, 1, usageRepo.createCalls)
			require.Equal(t, tc.recording, interactionRepo.created)
			if tc.recording {
				require.Equal(t, int64(77), interactionRepo.input.UsageLogID)
			}
		})
	}
}
