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
