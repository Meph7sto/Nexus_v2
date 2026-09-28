import { apiClient } from '../client'
import type { MonthlyLedgerPaymentInput } from './monthlyLedger'

export interface IncomeFields { title: string; customer: string; category: string; note: string; amount: number; cost: number }
export interface IncomeEntryInput extends IncomeFields { billing_date: string; due_date: string }
export interface IncomeEntry extends IncomeEntryInput {
  id: number; schedule_id: number | null; paid_amount: number; outstanding_amount: number; overpaid_amount: number; profit: number
  status: 'unpaid' | 'partial' | 'settled' | 'overpaid'; created_by: number; updated_by: number
}
export interface IncomeScheduleInput extends IncomeFields { first_date: string; interval_months: number; end_date: string | null }
export interface IncomeSchedule extends IncomeScheduleInput { id: number; state: 'active' | 'paused' | 'ended'; next_index: number }
export interface IncomePayment extends MonthlyLedgerPaymentInput { id: number; entry_id: number; created_by: number; updated_by: number }
export interface IncomeSummary { receivable_amount: number; paid_amount: number; outstanding_amount: number; overpaid_amount: number; cost: number; profit: number }
export interface IncomeList { items: IncomeEntry[]; total: number; summary: IncomeSummary; categories: string[] }
const root = '/admin/monthly-ledger'
export const incomeAPI = {
  async overview(month: string) { return (await apiClient.get<IncomeSummary>(`${root}/overview`, { params: { month } })).data },
  async list(params: { month: string; q?: string; category?: string; status?: string; page?: number; page_size?: number }) { return (await apiClient.get<IncomeList>(`${root}/income-entries`, { params })).data },
  async saveEntry(id: number, input: IncomeEntryInput) { return (id ? await apiClient.put<IncomeEntry>(`${root}/income-entries/${id}`, input) : await apiClient.post<IncomeEntry>(`${root}/income-entries`, input)).data },
  async deleteEntry(id: number) { await apiClient.delete(`${root}/income-entries/${id}`) },
  async payments(id: number) { return (await apiClient.get<IncomePayment[]>(`${root}/income-entries/${id}/payments`)).data },
  async savePayment(entry: number, id: number, input: MonthlyLedgerPaymentInput) { return (id ? await apiClient.put<IncomePayment>(`${root}/income-payments/${id}`, input) : await apiClient.post<IncomePayment>(`${root}/income-entries/${entry}/payments`, input)).data },
  async deletePayment(id: number) { await apiClient.delete(`${root}/income-payments/${id}`) },
  async schedules() { return (await apiClient.get<IncomeSchedule[]>(`${root}/income-schedules`)).data },
  async preview(input: IncomeScheduleInput) { return (await apiClient.post<{ count: number }>(`${root}/income-schedules/preview`, input)).data },
  async saveSchedule(id: number, input: IncomeScheduleInput) { return (id ? await apiClient.put<IncomeSchedule>(`${root}/income-schedules/${id}`, input) : await apiClient.post<IncomeSchedule>(`${root}/income-schedules`, input)).data },
  async setState(id: number, state: IncomeSchedule['state']) { await apiClient.put(`${root}/income-schedules/${id}/state`, { state }) },
}
