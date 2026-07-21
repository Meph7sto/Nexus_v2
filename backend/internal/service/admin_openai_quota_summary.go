package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const openAIQuotaSummaryAccountPageSize = 500

// GetOpenAIQuotaSummary loads every OpenAI account through the current V2
// repository contract before projecting its canonical quota snapshots. It
// treats inconsistent pagination as an error so the admin API never reports a
// silently partial summary.
func (s *adminServiceImpl) GetOpenAIQuotaSummary(ctx context.Context, input OpenAIQuotaSummaryInput) (*OpenAIQuotaSummaryResponse, error) {
	if s == nil || s.accountRepo == nil {
		return nil, fmt.Errorf("openai quota summary account repository is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	if input.ProjectionAt.IsZero() {
		input.ProjectionAt = now
	}
	if input.GeneratedAt.IsZero() {
		input.GeneratedAt = now
	}

	accounts, err := s.listAllOpenAIAccountsForQuotaSummary(ctx)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	summary := BuildOpenAIQuotaSummary(accounts, input)
	return &summary, nil
}

func (s *adminServiceImpl) listAllOpenAIAccountsForQuotaSummary(ctx context.Context) ([]Account, error) {
	accounts := make([]Account, 0, openAIQuotaSummaryAccountPageSize)
	seenAccountIDs := make(map[int64]struct{})
	var expectedTotal int64 = -1
	expectedPages := 0

	for page := 1; ; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		params := pagination.PaginationParams{
			Page:      page,
			PageSize:  openAIQuotaSummaryAccountPageSize,
			SortBy:    "id",
			SortOrder: pagination.SortOrderAsc,
		}
		batch, pageInfo, err := s.accountRepo.ListWithFilters(ctx, params, PlatformOpenAI, "", "", "", 0, "")
		if err != nil {
			return nil, fmt.Errorf("list openai accounts page %d: %w", page, err)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if pageInfo == nil {
			return nil, fmt.Errorf("openai quota summary page %d returned no pagination metadata", page)
		}
		if pageInfo.Page != page || pageInfo.PageSize != params.PageSize || pageInfo.Total < 0 {
			return nil, fmt.Errorf("openai quota summary page %d returned inconsistent pagination metadata", page)
		}

		pageCount := openAIQuotaSummaryPageCount(pageInfo.Total, params.PageSize)
		if pageInfo.Pages != pageCount {
			return nil, fmt.Errorf("openai quota summary page %d returned inconsistent total/pages metadata", page)
		}
		if expectedTotal == -1 {
			expectedTotal = pageInfo.Total
			expectedPages = pageCount
		} else if pageInfo.Total != expectedTotal || pageCount != expectedPages {
			return nil, fmt.Errorf("openai quota summary page %d changed pagination metadata during scan", page)
		}

		if expectedTotal == 0 {
			if len(batch) != 0 {
				return nil, fmt.Errorf("openai quota summary returned accounts for an empty result")
			}
			return accounts, nil
		}
		if page > expectedPages || len(batch) == 0 {
			return nil, fmt.Errorf("openai quota summary page %d ended before all %d accounts were loaded", page, expectedTotal)
		}

		for _, account := range batch {
			if _, duplicate := seenAccountIDs[account.ID]; duplicate {
				return nil, fmt.Errorf("openai quota summary encountered duplicate account %d across pages", account.ID)
			}
			seenAccountIDs[account.ID] = struct{}{}
			accounts = append(accounts, account)
		}
		if int64(len(accounts)) > expectedTotal {
			return nil, fmt.Errorf("openai quota summary loaded more than expected %d accounts", expectedTotal)
		}
		if int64(len(accounts)) == expectedTotal {
			return accounts, nil
		}
		if page == expectedPages {
			return nil, fmt.Errorf("openai quota summary loaded %d of %d accounts", len(accounts), expectedTotal)
		}
	}
}

func openAIQuotaSummaryPageCount(total int64, pageSize int) int {
	if total <= 0 {
		return 0
	}
	return int((total + int64(pageSize) - 1) / int64(pageSize))
}
