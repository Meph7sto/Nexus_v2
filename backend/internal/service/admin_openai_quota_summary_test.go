package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type openAIQuotaSummaryAccountRepoStub struct {
	AccountRepository
	list  func(context.Context, pagination.PaginationParams, string, string, string, string, int64, string) ([]Account, *pagination.PaginationResult, error)
	calls []openAIQuotaSummaryListCall
}

type openAIQuotaSummaryListCall struct {
	params      pagination.PaginationParams
	platform    string
	accountType string
	status      string
	search      string
	groupID     int64
	privacyMode string
}

func (s *openAIQuotaSummaryAccountRepoStub) ListWithFilters(
	ctx context.Context,
	params pagination.PaginationParams,
	platform, accountType, status, search string,
	groupID int64,
	privacyMode string,
) ([]Account, *pagination.PaginationResult, error) {
	s.calls = append(s.calls, openAIQuotaSummaryListCall{
		params:      params,
		platform:    platform,
		accountType: accountType,
		status:      status,
		search:      search,
		groupID:     groupID,
		privacyMode: privacyMode,
	})
	return s.list(ctx, params, platform, accountType, status, search, groupID, privacyMode)
}

func TestAdminServiceGetOpenAIQuotaSummaryLoadsAllPages(t *testing.T) {
	projectionAt := time.Date(2026, 7, 20, 15, 0, 0, 0, time.UTC)
	group := &Group{ID: 10, Name: "Alpha"}
	repo := &openAIQuotaSummaryAccountRepoStub{}
	repo.list = func(_ context.Context, params pagination.PaginationParams, _ string, _ string, _ string, _ string, _ int64, _ string) ([]Account, *pagination.PaginationResult, error) {
		total := int64(params.PageSize + 1)
		pageInfo := &pagination.PaginationResult{
			Total:    total,
			Page:     params.Page,
			PageSize: params.PageSize,
			Pages:    2,
		}
		switch params.Page {
		case 1:
			accounts := make([]Account, 0, params.PageSize)
			for id := 1; id <= params.PageSize; id++ {
				accounts = append(accounts, openAIQuotaSummaryTestAccount(
					int64(id),
					"page-one",
					StatusActive,
					map[string]any{"plan_type": "pro"},
					openAIQuotaSummaryCompleteExtra(projectionAt, 25, 50),
					group,
				))
			}
			return accounts, pageInfo, nil
		case 2:
			return []Account{openAIQuotaSummaryTestAccount(
				int64(params.PageSize+1),
				"page-two",
				StatusActive,
				map[string]any{"plan_type": "pro"},
				openAIQuotaSummaryCompleteExtra(projectionAt, 25, 50),
				group,
			)}, pageInfo, nil
		default:
			return nil, nil, errors.New("unexpected page")
		}
	}

	adminService := &adminServiceImpl{accountRepo: repo}
	summary, err := adminService.GetOpenAIQuotaSummary(context.Background(), OpenAIQuotaSummaryInput{
		ProjectionAt: projectionAt,
		GeneratedAt:  projectionAt,
	})

	require.NoError(t, err)
	require.Len(t, repo.calls, 2)
	for index, call := range repo.calls {
		require.Equal(t, index+1, call.params.Page)
		require.Positive(t, call.params.PageSize)
		require.Equal(t, "id", call.params.SortBy)
		require.Equal(t, pagination.SortOrderAsc, call.params.SortOrder)
		require.Equal(t, PlatformOpenAI, call.platform)
		require.Empty(t, call.accountType)
		require.Empty(t, call.status)
		require.Empty(t, call.search)
		require.Zero(t, call.groupID)
		require.Empty(t, call.privacyMode)
	}
	require.Len(t, summary.Groups, 1)
	require.Equal(t, repo.calls[0].params.PageSize+1, summary.Groups[0].Rows[0].IncludedCount)
}

func TestAdminServiceGetOpenAIQuotaSummaryHandlesEmptyAndRejectsInconsistentPages(t *testing.T) {
	projectionAt := time.Date(2026, 7, 20, 15, 0, 0, 0, time.UTC)

	t.Run("empty", func(t *testing.T) {
		repo := &openAIQuotaSummaryAccountRepoStub{
			list: func(_ context.Context, params pagination.PaginationParams, _ string, _ string, _ string, _ string, _ int64, _ string) ([]Account, *pagination.PaginationResult, error) {
				return []Account{}, &pagination.PaginationResult{Page: params.Page, PageSize: params.PageSize}, nil
			},
		}
		summary, err := (&adminServiceImpl{accountRepo: repo}).GetOpenAIQuotaSummary(context.Background(), OpenAIQuotaSummaryInput{
			ProjectionAt: projectionAt,
			GeneratedAt:  projectionAt,
		})
		require.NoError(t, err)
		require.Empty(t, summary.Groups)
	})

	for _, testCase := range []struct {
		name string
		list func(pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error)
	}{
		{
			name: "missing metadata",
			list: func(_ pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
				return nil, nil, nil
			},
		},
		{
			name: "contradictory page count",
			list: func(params pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
				return []Account{openAIQuotaSummaryTestAccount(1, "one", StatusActive, nil, nil)}, &pagination.PaginationResult{
					Total:    1,
					Page:     params.Page,
					PageSize: params.PageSize,
					Pages:    2,
				}, nil
			},
		},
		{
			name: "empty batch before total",
			list: func(params pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
				return []Account{}, &pagination.PaginationResult{
					Total:    1,
					Page:     params.Page,
					PageSize: params.PageSize,
					Pages:    1,
				}, nil
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &openAIQuotaSummaryAccountRepoStub{
				list: func(_ context.Context, params pagination.PaginationParams, _ string, _ string, _ string, _ string, _ int64, _ string) ([]Account, *pagination.PaginationResult, error) {
					return testCase.list(params)
				},
			}
			_, err := (&adminServiceImpl{accountRepo: repo}).GetOpenAIQuotaSummary(context.Background(), OpenAIQuotaSummaryInput{
				ProjectionAt: projectionAt,
				GeneratedAt:  projectionAt,
			})
			require.Error(t, err)
		})
	}
}

func TestAdminServiceGetOpenAIQuotaSummaryHonorsContextAndRepositoryErrors(t *testing.T) {
	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		repo := &openAIQuotaSummaryAccountRepoStub{
			list: func(context.Context, pagination.PaginationParams, string, string, string, string, int64, string) ([]Account, *pagination.PaginationResult, error) {
				t.Fatal("repository must not be called after context cancellation")
				return nil, nil, nil
			},
		}
		_, err := (&adminServiceImpl{accountRepo: repo}).GetOpenAIQuotaSummary(ctx, OpenAIQuotaSummaryInput{})
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("repository error", func(t *testing.T) {
		expectedErr := errors.New("database unavailable")
		repo := &openAIQuotaSummaryAccountRepoStub{
			list: func(context.Context, pagination.PaginationParams, string, string, string, string, int64, string) ([]Account, *pagination.PaginationResult, error) {
				return nil, nil, expectedErr
			},
		}
		_, err := (&adminServiceImpl{accountRepo: repo}).GetOpenAIQuotaSummary(context.Background(), OpenAIQuotaSummaryInput{})
		require.ErrorIs(t, err, expectedErr)
	})
}
