package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func incomeResult(c *gin.Context, result any, err error) {
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
func incomeAudit(c *gin.Context, action string, id int64, input any) {
	middleware.SetAuditAction(c, "admin.monthly_ledger."+action)
	extra := map[string]any{"income_record_id": id}
	switch value := input.(type) {
	case service.IncomeEntryInput:
		extra["new_amount"] = value.Amount
		extra["income_cost"] = value.Cost
		extra["ledger_month"] = value.BillingDate[:7]
	case service.IncomeScheduleInput:
		extra["new_amount"] = value.Amount
		extra["income_cost"] = value.Cost
	case service.MonthlyLedgerPaymentInput:
		extra["new_amount"] = value.Amount
	case string:
		extra["income_state"] = value
	}
	middleware.SetAuditExtra(c, extra)
}
func (h *MonthlyLedgerHandler) IncomeOverview(c *gin.Context) {
	r, e := h.service.IncomeOverview(c.Request.Context(), c.Query("month"))
	incomeResult(c, r, e)
}
func (h *MonthlyLedgerHandler) ListIncome(c *gin.Context) {
	page, size := response.ParsePagination(c)
	r, e := h.service.ListIncome(c.Request.Context(), service.IncomeListParams{Month: c.Query("month"), Query: c.Query("q"), Category: c.Query("category"), Status: c.Query("status"), Page: page, PageSize: size})
	incomeResult(c, r, e)
}
func (h *MonthlyLedgerHandler) SaveIncomeEntry(c *gin.Context) {
	var in service.IncomeEntryInput
	if c.ShouldBindJSON(&in) != nil {
		response.ErrorFrom(c, service.ErrIncomeInvalid)
		return
	}
	var id int64
	if c.Param("id") != "" {
		var ok bool
		id, ok = parseMonthlyLedgerID(c, "id")
		if !ok {
			return
		}
	}
	r, e := h.service.SaveIncomeEntry(c.Request.Context(), id, in, monthlyLedgerActorID(c))
	if e == nil {
		action := "income_entry.create"
		if id != 0 {
			action = "income_entry.update"
		}
		incomeAudit(c, action, r.ID, in)
	}
	incomeResult(c, r, e)
}
func (h *MonthlyLedgerHandler) DeleteIncomeEntry(c *gin.Context) {
	id, ok := parseMonthlyLedgerID(c, "id")
	if !ok {
		return
	}
	e := h.service.DeleteIncomeEntry(c.Request.Context(), id, monthlyLedgerActorID(c))
	if e == nil {
		incomeAudit(c, "income_entry.delete", id, nil)
	}
	incomeResult(c, gin.H{"id": id}, e)
}
func (h *MonthlyLedgerHandler) ListIncomePayments(c *gin.Context) {
	id, ok := parseMonthlyLedgerID(c, "id")
	if !ok {
		return
	}
	r, e := h.service.ListIncomePayments(c.Request.Context(), id)
	incomeResult(c, r, e)
}
func (h *MonthlyLedgerHandler) SaveIncomePayment(c *gin.Context) {
	var in service.MonthlyLedgerPaymentInput
	if c.ShouldBindJSON(&in) != nil {
		response.ErrorFrom(c, service.ErrIncomeInvalid)
		return
	}
	id, ok := parseMonthlyLedgerID(c, "id")
	if !ok {
		return
	}
	var entry int64
	action := "income_payment.update"
	if c.Request.Method == "POST" {
		entry = id
		id = 0
		action = "income_payment.create"
	}
	r, e := h.service.SaveIncomePayment(c.Request.Context(), id, entry, in, monthlyLedgerActorID(c))
	if e == nil {
		incomeAudit(c, action, r.ID, in)
	}
	incomeResult(c, r, e)
}
func (h *MonthlyLedgerHandler) DeleteIncomePayment(c *gin.Context) {
	id, ok := parseMonthlyLedgerID(c, "id")
	if !ok {
		return
	}
	e := h.service.DeleteIncomePayment(c.Request.Context(), id)
	if e == nil {
		incomeAudit(c, "income_payment.delete", id, nil)
	}
	incomeResult(c, gin.H{"id": id}, e)
}
func (h *MonthlyLedgerHandler) ListIncomeSchedules(c *gin.Context) {
	r, e := h.service.ListIncomeSchedules(c.Request.Context())
	incomeResult(c, r, e)
}
func (h *MonthlyLedgerHandler) PreviewIncomeSchedule(c *gin.Context) {
	var in service.IncomeScheduleInput
	if c.ShouldBindJSON(&in) != nil {
		response.ErrorFrom(c, service.ErrIncomeInvalid)
		return
	}
	n, e := h.service.PreviewIncomeSchedule(in)
	incomeResult(c, gin.H{"count": n}, e)
}
func (h *MonthlyLedgerHandler) SaveIncomeSchedule(c *gin.Context) {
	var in service.IncomeScheduleInput
	if c.ShouldBindJSON(&in) != nil {
		response.ErrorFrom(c, service.ErrIncomeInvalid)
		return
	}
	var id int64
	if c.Param("id") != "" {
		var ok bool
		id, ok = parseMonthlyLedgerID(c, "id")
		if !ok {
			return
		}
	}
	r, e := h.service.SaveIncomeSchedule(c.Request.Context(), id, in, monthlyLedgerActorID(c))
	if e == nil {
		action := "income_schedule.create"
		if id != 0 {
			action = "income_schedule.update"
		}
		incomeAudit(c, action, r.ID, in)
	}
	incomeResult(c, r, e)
}
func (h *MonthlyLedgerHandler) SetIncomeScheduleState(c *gin.Context) {
	id, ok := parseMonthlyLedgerID(c, "id")
	if !ok {
		return
	}
	var in struct {
		State string `json:"state"`
	}
	if c.ShouldBindJSON(&in) != nil {
		response.ErrorFrom(c, service.ErrIncomeInvalid)
		return
	}
	e := h.service.SetIncomeScheduleState(c.Request.Context(), id, in.State, monthlyLedgerActorID(c))
	if e == nil {
		incomeAudit(c, "income_schedule.state", id, in.State)
	}
	incomeResult(c, gin.H{"id": id}, e)
}
