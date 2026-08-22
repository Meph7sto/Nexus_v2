import { apiClient } from '../client'

export type MonthlyLedgerStatus = 'unpaid' | 'partial' | 'settled' | 'overpaid' | 'waived'

export interface MonthlyLedgerRow {
  user_id: number
  email: string
  username: string
  deleted: boolean
  usage_amount: number
  pricing_usage_amount: number
  multiplier: number
  receivable_amount: number
  paid_amount: number
  outstanding_amount: number
  overpaid_amount: number
  status: MonthlyLedgerStatus
  payment_count: number
  last_paid_at?: string | null
}

export interface MonthlyLedgerSummary {
  usage_amount: number
  receivable_amount: number
  paid_amount: number
  outstanding_amount: number
  overpaid_amount: number
  user_count: number
  unpaid_count: number
  partial_count: number
  settled_count: number
  overpaid_count: number
  waived_count: number
}

export interface MonthlyLedgerListResponse {
  items: MonthlyLedgerRow[]
  summary: MonthlyLedgerSummary
  total: number
  page: number
  page_size: number
  pages: number
  month: string
  current_month: string
  can_record_payments: boolean
}

export interface MonthlyLedgerPayment {
  id: number
  user_id: number
  billing_month: string
  amount: number
  paid_at: string
  note: string
  created_by: number
  created_by_email: string
  updated_by: number
  updated_by_email: string
  created_at: string
  updated_at: string
}

export interface MonthlyLedgerPaymentInput {
  amount: number
  paid_at: string
  note?: string
}

export interface MonthlyLedgerListParams {
  month?: string
  user_id?: number
  q?: string
  status?: MonthlyLedgerStatus | ''
  page?: number
  page_size?: number
  sort_by?: 'user' | 'usage_amount' | 'multiplier' | 'receivable_amount' | 'paid_amount' | 'outstanding_amount' | 'last_paid_at'
  sort_order?: 'asc' | 'desc'
}

export async function list(params: MonthlyLedgerListParams = {}): Promise<MonthlyLedgerListResponse> {
  const { data } = await apiClient.get<MonthlyLedgerListResponse>('/admin/monthly-ledger', { params })
  return data
}

export async function listPayments(month: string, userId: number): Promise<MonthlyLedgerPayment[]> {
  const { data } = await apiClient.get<MonthlyLedgerPayment[]>(`/admin/monthly-ledger/${encodeURIComponent(month)}/users/${userId}/payments`)
  return data
}

export async function setMultiplier(month: string, userId: number, multiplier: number) {
  const { data } = await apiClient.put(`/admin/monthly-ledger/${encodeURIComponent(month)}/users/${userId}/multiplier`, { multiplier })
  return data
}

export async function createPayment(month: string, userId: number, input: MonthlyLedgerPaymentInput) {
  const { data } = await apiClient.post<MonthlyLedgerPayment>(`/admin/monthly-ledger/${encodeURIComponent(month)}/users/${userId}/payments`, input)
  return data
}

export async function updatePayment(paymentId: number, input: MonthlyLedgerPaymentInput) {
  const { data } = await apiClient.put<MonthlyLedgerPayment>(`/admin/monthly-ledger/payments/${paymentId}`, input)
  return data
}

export async function deletePayment(paymentId: number) {
  const { data } = await apiClient.delete<{ id: number }>(`/admin/monthly-ledger/payments/${paymentId}`)
  return data
}

const monthlyLedgerAPI = {
  list,
  listPayments,
  setMultiplier,
  createPayment,
  updatePayment,
  deletePayment,
}

export default monthlyLedgerAPI
