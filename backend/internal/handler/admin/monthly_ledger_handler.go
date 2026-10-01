package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type MonthlyLedgerHandler struct {
	service *service.MonthlyLedgerService
}

func NewMonthlyLedgerHandler(monthlyLedgerService *service.MonthlyLedgerService) *MonthlyLedgerHandler {
	return &MonthlyLedgerHandler{service: monthlyLedgerService}
}

func (h *MonthlyLedgerHandler) EmailQuota(c *gin.Context) {
	quota, err := h.service.EmailQuota(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, quota)
}

func (h *MonthlyLedgerHandler) ListEmailHistory(c *gin.Context) {
	userID, ok := parseMonthlyLedgerID(c, "user_id")
	if !ok {
		return
	}
	page, size := response.ParsePagination(c)
	result, err := h.service.ListEmailHistory(c.Request.Context(), c.Param("month"), userID, page, size)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *MonthlyLedgerHandler) SetEmailDailyLimit(c *gin.Context) {
	var req struct {
		DailyLimit *int `json:"daily_limit" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.DailyLimit == nil {
		response.BadRequest(c, "daily_limit is required and must be an integer")
		return
	}
	if err := h.service.SetEmailDailyLimit(c.Request.Context(), *req.DailyLimit, monthlyLedgerActorID(c)); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditAction(c, "admin.monthly_ledger.email_limit.update")
	middleware.SetAuditExtra(c, map[string]any{"ledger_email_daily_limit": *req.DailyLimit})
	response.Success(c, gin.H{"daily_limit": *req.DailyLimit})
}

func (h *MonthlyLedgerHandler) SendManualEmail(c *gin.Context) {
	var req service.MonthlyLedgerManualEmailInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid email request")
		return
	}
	middleware.SetAuditAction(c, "admin.monthly_ledger.email.send")
	middleware.SetAuditExtra(c, map[string]any{"ledger_user_id": req.UserID, "ledger_month": req.Month, "new_amount": req.Amount, "ledger_email_request_id": req.RequestID})
	if err := h.service.SendManualEmail(c.Request.Context(), req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"sent": true})
}

func (h *MonthlyLedgerHandler) ListEmailPreferences(c *gin.Context) {
	page, size := response.ParsePagination(c)
	items, total, err := h.service.ListEmailPreferences(c.Request.Context(), strings.TrimSpace(c.Query("q")), pagination.PaginationParams{Page: page, PageSize: size})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": size})
}

func (h *MonthlyLedgerHandler) SetEmailPreference(c *gin.Context) {
	userID, ok := parseMonthlyLedgerID(c, "user_id")
	if !ok {
		return
	}
	var req struct {
		Enabled *bool `json:"enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		response.BadRequest(c, "enabled is required")
		return
	}
	if err := h.service.SetEmailPreference(c.Request.Context(), userID, *req.Enabled, monthlyLedgerActorID(c)); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditAction(c, "admin.monthly_ledger.email_preference.update")
	middleware.SetAuditExtra(c, map[string]any{"ledger_user_id": userID, "enabled": *req.Enabled})
	response.Success(c, gin.H{"user_id": userID, "enabled": *req.Enabled})
}

func (h *MonthlyLedgerHandler) SetEmailPreferences(c *gin.Context) {
	var req struct {
		UserIDs []int64 `json:"user_ids"`
		Enabled *bool   `json:"enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		response.BadRequest(c, "user_ids and enabled are required")
		return
	}
	count, err := h.service.SetEmailPreferences(c.Request.Context(), req.UserIDs, *req.Enabled, monthlyLedgerActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditAction(c, "admin.monthly_ledger.email_preference.batch_update")
	middleware.SetAuditExtra(c, map[string]any{"matched_count": count, "enabled": *req.Enabled})
	response.Success(c, gin.H{"updated_count": count, "enabled": *req.Enabled})
}

func (h *MonthlyLedgerHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	userID, ok := parseOptionalMonthlyLedgerUserID(c)
	if !ok {
		return
	}
	result, err := h.service.List(c.Request.Context(), c.Query("month"), service.MonthlyLedgerListParams{
		Pagination: pagination.PaginationParams{
			Page: page, PageSize: pageSize,
			SortBy:    c.DefaultQuery("sort_by", "outstanding_amount"),
			SortOrder: c.DefaultQuery("sort_order", "desc"),
		},
		Query:  strings.TrimSpace(c.Query("q")),
		Status: strings.TrimSpace(c.Query("status")),
		UserID: userID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func parseOptionalMonthlyLedgerUserID(c *gin.Context) (int64, bool) {
	raw := strings.TrimSpace(c.Query("user_id"))
	if raw == "" {
		return 0, true
	}
	userID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || userID <= 0 {
		response.Error(c, http.StatusBadRequest, "Invalid user_id")
		return 0, false
	}
	return userID, true
}

func (h *MonthlyLedgerHandler) ListPayments(c *gin.Context) {
	userID, ok := parseMonthlyLedgerID(c, "user_id")
	if !ok {
		return
	}
	payments, err := h.service.ListPayments(c.Request.Context(), c.Param("month"), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, payments)
}

type monthlyLedgerMultiplierRequest struct {
	Multiplier *float64 `json:"multiplier" binding:"required"`
}

func (h *MonthlyLedgerHandler) SetSettlement(c *gin.Context) {
	userID, ok := parseMonthlyLedgerID(c, "user_id")
	if !ok {
		return
	}
	var req struct {
		ManuallySettled *bool `json:"manually_settled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ManuallySettled == nil {
		response.BadRequest(c, "Invalid settlement request")
		return
	}
	change, err := h.service.SetSettlement(c.Request.Context(), c.Param("month"), userID, *req.ManuallySettled, monthlyLedgerActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditAction(c, "admin.monthly_ledger.settlement.update")
	middleware.SetAuditExtra(c, map[string]any{
		"ledger_user_id": userID, "ledger_month": change.BillingMonth,
		"old_manually_settled": change.PreviousManuallySettled, "new_manually_settled": change.ManuallySettled,
	})
	response.Success(c, change)
}

type monthlyLedgerMultipliersRequest struct {
	UserIDs    []int64  `json:"user_ids" binding:"required"`
	Multiplier *float64 `json:"multiplier" binding:"required"`
}

func (h *MonthlyLedgerHandler) SetMultiplier(c *gin.Context) {
	userID, ok := parseMonthlyLedgerID(c, "user_id")
	if !ok {
		return
	}
	var req monthlyLedgerMultiplierRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Multiplier == nil {
		response.BadRequest(c, "Invalid multiplier request")
		return
	}
	change, err := h.service.SetMultiplier(c.Request.Context(), c.Param("month"), userID, *req.Multiplier, monthlyLedgerActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditAction(c, "admin.monthly_ledger.multiplier.update")
	middleware.SetAuditExtra(c, map[string]any{
		"ledger_user_id": userID, "ledger_month": change.BillingMonth,
		"old_multiplier": change.PreviousMultiplier, "new_multiplier": change.Multiplier,
	})
	response.Success(c, change)
}

func (h *MonthlyLedgerHandler) SetMultipliers(c *gin.Context) {
	var req monthlyLedgerMultipliersRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Multiplier == nil {
		response.BadRequest(c, "Invalid multipliers request")
		return
	}
	result, err := h.service.SetMultipliers(c.Request.Context(), c.Param("month"), req.UserIDs, *req.Multiplier, monthlyLedgerActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditAction(c, "admin.monthly_ledger.multiplier.batch_update")
	middleware.SetAuditExtra(c, map[string]any{
		"ledger_user_ids": req.UserIDs, "ledger_month": result.BillingMonth,
		"updated_count": result.UpdatedCount, "new_multiplier": result.Multiplier,
	})
	response.Success(c, result)
}

func (h *MonthlyLedgerHandler) CreatePayment(c *gin.Context) {
	userID, ok := parseMonthlyLedgerID(c, "user_id")
	if !ok {
		return
	}
	var req service.MonthlyLedgerPaymentInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid payment request")
		return
	}
	payment, err := h.service.CreatePayment(c.Request.Context(), c.Param("month"), userID, req, monthlyLedgerActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditAction(c, "admin.monthly_ledger.payment.create")
	middleware.SetAuditExtra(c, map[string]any{
		"ledger_user_id": userID, "ledger_month": payment.BillingMonth,
		"ledger_payment_id": payment.ID, "new_amount": payment.Amount,
	})
	response.Success(c, payment)
}

func (h *MonthlyLedgerHandler) UpdatePayment(c *gin.Context) {
	paymentID, ok := parseMonthlyLedgerID(c, "id")
	if !ok {
		return
	}
	var req service.MonthlyLedgerPaymentInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid payment request")
		return
	}
	change, err := h.service.UpdatePayment(c.Request.Context(), paymentID, req, monthlyLedgerActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditAction(c, "admin.monthly_ledger.payment.update")
	middleware.SetAuditExtra(c, map[string]any{
		"ledger_user_id": change.After.UserID, "ledger_month": change.After.BillingMonth,
		"ledger_payment_id": paymentID, "old_amount": change.Before.Amount,
		"new_amount": change.After.Amount,
	})
	response.Success(c, change.After)
}

func (h *MonthlyLedgerHandler) DeletePayment(c *gin.Context) {
	paymentID, ok := parseMonthlyLedgerID(c, "id")
	if !ok {
		return
	}
	payment, err := h.service.DeletePayment(c.Request.Context(), paymentID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditAction(c, "admin.monthly_ledger.payment.delete")
	middleware.SetAuditExtra(c, map[string]any{
		"ledger_user_id": payment.UserID, "ledger_month": payment.BillingMonth,
		"ledger_payment_id": payment.ID, "old_amount": payment.Amount,
	})
	response.Success(c, gin.H{"id": payment.ID})
}

func parseMonthlyLedgerID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "Invalid "+name)
		return 0, false
	}
	return id, true
}

func monthlyLedgerActorID(c *gin.Context) int64 {
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		return subject.UserID
	}
	return 0
}
