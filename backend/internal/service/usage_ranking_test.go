package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type usageRankingRepositoryStub struct {
	UsageLogRepository
	rows []usagestats.UserUsageRankingItem
}

func (s *usageRankingRepositoryStub) GetUserUsageRanking(
	context.Context,
	pagination.PaginationParams,
	usagestats.UserUsageRankingSort,
	time.Time,
	time.Time,
) ([]usagestats.UserUsageRankingItem, *pagination.PaginationResult, error) {
	return s.rows, &pagination.PaginationResult{Total: int64(len(s.rows)), Page: 1, PageSize: 20, Pages: 1}, nil
}

func TestGetUserUsageRankingRedactsIdentityForRegularUsers(t *testing.T) {
	repo := &usageRankingRepositoryStub{rows: []usagestats.UserUsageRankingItem{{
		Rank: 1, UserID: 7, Nickname: "Alice", Email: "alice@example.test",
	}}}
	service := NewUsageService(repo, nil, nil, nil)

	items, _, err := service.GetUserUsageRanking(context.Background(), UsageRankingParams{})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Nil(t, items[0].UserID)
	require.Equal(t, "A***e", items[0].Nickname)
	require.Equal(t, "a****@example.test", items[0].Email)
}

func TestGetUserUsageRankingIncludesIdentityOnlyWhenRequested(t *testing.T) {
	repo := &usageRankingRepositoryStub{rows: []usagestats.UserUsageRankingItem{{
		Rank: 1, UserID: 7, Nickname: "Alice", Email: "alice@example.test",
	}}}
	service := NewUsageService(repo, nil, nil, nil)

	items, _, err := service.GetUserUsageRanking(context.Background(), UsageRankingParams{IncludeIdentity: true})

	require.NoError(t, err)
	require.NotNil(t, items[0].UserID)
	require.Equal(t, int64(7), *items[0].UserID)
	require.Equal(t, "Alice", items[0].Nickname)
	require.Equal(t, "alice@example.test", items[0].Email)
}
