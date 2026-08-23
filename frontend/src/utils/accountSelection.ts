import { fetchAllPaginatedIDs } from './paginatedSelection'

interface AccountIDRow {
  id: number
}

interface AccountListPage {
  items: AccountIDRow[]
  total: number
  pages?: number
}

type AccountPageFetcher = (
  page: number,
  pageSize: number,
  filters: Record<string, unknown>
) => Promise<AccountListPage>

const SELECT_ALL_PAGE_SIZE = 1000

export async function fetchAllAccountIds(
  fetchPage: AccountPageFetcher,
  filters: Record<string, unknown>
): Promise<number[]> {
  const requestFilters = {
    ...filters,
    lite: '1',
    include_scheduler_score: '0'
  }
  return fetchAllPaginatedIDs(
    (page, pageSize) => fetchPage(page, pageSize, requestFilters),
    account => account.id,
    { pageSize: SELECT_ALL_PAGE_SIZE, incompleteError: '账号列表结果不完整' }
  )
}
