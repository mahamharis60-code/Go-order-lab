package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go-order-lab/internal/metrics"
	"go-order-lab/internal/middleware"
	"go-order-lab/internal/response"
	"go-order-lab/internal/service"
)

type OrderHandler struct {
	orders *service.OrderService
}

func NewOrderHandler(orders *service.OrderService) *OrderHandler {
	return &OrderHandler{orders: orders}
}

func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var req service.CreateOrderInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	order, stockLeft, err := h.orders.CreateOrderContext(c.Request.Context(), middleware.UserID(c), req)
	if err != nil {
		metrics.IncBusinessEvent("activity_order", orderErrorMetric(err))
		response.Error(c, orderErrorStatus(err), err.Error())
		return
	}
	metrics.IncBusinessEvent("activity_order", "accepted")
	response.Accepted(c, gin.H{
		"order_no":   order.OrderNo,
		"status":     order.Status,
		"stock_left": stockLeft,
	})
}

func (h *OrderHandler) CheckoutCart(c *gin.Context) {
	var req service.CheckoutCartInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	order, err := h.orders.CheckoutCart(middleware.UserID(c), req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Created(c, order)
}

func (h *OrderHandler) ListOrders(c *gin.Context) {
	orders, err := h.orders.ListOrders(middleware.UserID(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, orders)
}

func (h *OrderHandler) GetOrder(c *gin.Context) {
	order, err := h.orders.GetOrder(middleware.UserID(c), c.Param("order_no"))
	if err != nil {
		response.Error(c, orderErrorStatus(err), err.Error())
		return
	}
	response.OK(c, order)
}

func (h *OrderHandler) CancelOrder(c *gin.Context) {
	result, err := h.orders.CancelOrder(middleware.UserID(c), c.Param("order_no"))
	if err != nil {
		response.Error(c, orderErrorStatus(err), err.Error())
		return
	}
	response.OK(c, result)
}

func (h *OrderHandler) ExpireOrders(c *gin.Context) {
	var req service.ExpireOrdersInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.orders.ExpireOrders(req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, result)
}

func (h *OrderHandler) Compensate(c *gin.Context) {
	var req service.CompensateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.orders.Compensate(req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, result)
}

func (h *OrderHandler) ReconcileStock(c *gin.Context) {
	var req service.StockReconcileInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.orders.ReconcileStock(req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, result)
}

func (h *OrderHandler) PaymentCallback(c *gin.Context) {
	var req service.PaymentCallbackInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.orders.PaymentCallback(req)
	if err != nil {
		metrics.IncBusinessEvent("payment_callback", orderErrorMetric(err))
		response.Error(c, orderErrorStatus(err), err.Error())
		return
	}
	metrics.IncBusinessEvent("payment_callback", "processed")
	response.OK(c, result)
}

func (h *OrderHandler) ListLogs(c *gin.Context) {
	logs, err := h.orders.ListLogs()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, logs)
}

func orderErrorStatus(err error) int {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout
	case errors.Is(err, context.Canceled):
		return http.StatusRequestTimeout
	case errors.Is(err, service.ErrActivityNotFound), errors.Is(err, service.ErrOrderNotFound):
		return http.StatusNotFound
	case errors.Is(err, service.ErrActivityNotAvailable), errors.Is(err, service.ErrSoldOut), errors.Is(err, service.ErrDuplicateOrder), errors.Is(err, service.ErrIllegalTransition):
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}

func orderErrorMetric(err error) string {
	switch {
	case errors.Is(err, service.ErrActivityNotFound):
		return "activity_not_found"
	case errors.Is(err, service.ErrOrderNotFound):
		return "order_not_found"
	case errors.Is(err, service.ErrActivityNotAvailable):
		return "activity_not_available"
	case errors.Is(err, service.ErrSoldOut):
		return "sold_out"
	case errors.Is(err, service.ErrDuplicateOrder):
		return "duplicate_order"
	case errors.Is(err, service.ErrIllegalTransition):
		return "illegal_transition"
	default:
		return "error"
	}
}
