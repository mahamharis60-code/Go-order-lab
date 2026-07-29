package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go-order-lab/internal/middleware"
	"go-order-lab/internal/response"
	"go-order-lab/internal/service"
)

type CouponHandler struct {
	coupons *service.CouponService
}

func NewCouponHandler(coupons *service.CouponService) *CouponHandler {
	return &CouponHandler{coupons: coupons}
}

func (h *CouponHandler) CreateCoupon(c *gin.Context) {
	var req service.CreateCouponInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	coupon, err := h.coupons.Create(req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Created(c, coupon)
}

func (h *CouponHandler) ListCoupons(c *gin.Context) {
	coupons, err := h.coupons.List()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, coupons)
}

func (h *CouponHandler) ClaimCoupon(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid coupon id")
		return
	}
	userCoupon, err := h.coupons.Claim(middleware.UserID(c), uint(id))
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Created(c, userCoupon)
}

func (h *CouponHandler) MyCoupons(c *gin.Context) {
	coupons, err := h.coupons.MyCoupons(middleware.UserID(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, coupons)
}
