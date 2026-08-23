export interface PaginatedSelectionPage<T> {
  items: T[]
  total: number
  pages?: number
}

interface FetchAllPaginatedIDsOptions {
  pageSize: number
  incompleteError: string
}

export async function fetchAllPaginatedIDs<T>(
  fetchPage: (page: number, pageSize: number) => Promise<PaginatedSelectionPage<T>>,
  getID: (item: T) => number,
  options: FetchAllPaginatedIDsOptions
): Promise<number[]> {
  const firstPage = await fetchPage(1, options.pageSize)
  const pageCount = Math.max(
    firstPage.pages ?? 0,
    Math.ceil(firstPage.total / options.pageSize)
  )
  const ids = firstPage.items.map(getID)

  for (let page = 2; page <= pageCount; page++) {
    const result = await fetchPage(page, options.pageSize)
    ids.push(...result.items.map(getID))
  }

  const uniqueIDs = Array.from(new Set(ids))
  if (uniqueIDs.length !== firstPage.total) {
    throw new Error(options.incompleteError)
  }
  return uniqueIDs
}
