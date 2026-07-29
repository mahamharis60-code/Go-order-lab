package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go-order-lab/internal/middleware"
	"go-order-lab/internal/response"
	"go-order-lab/internal/service"
)

type CartHandler struct {
	carts *service.CartService
}

func NewCartHandler(carts *service.CartService) *CartHandler {
	return &CartHandler{carts: carts}
}

func (h *CartHandler) AddItem(c *gin.Context) {
	var req service.AddCartItemInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	item, err := h.carts.AddItem(middleware.UserID(c), req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Created(c, item)
}

func (h *CartHandler) ListItems(c *gin.Context) {
	items, err := h.carts.List(middleware.UserID(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, items)
}

func (h *CartHandler) UpdateItem(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid cart item id")
		return
	}
	var req struct {
		Quantity int `json:"quantity" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	item, err := h.carts.UpdateQuantity(middleware.UserID(c), uint(id), req.Quantity)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, item)
}

func (h *CartHandler) DeleteItem(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid cart item id")
		return
	}
	if err := h.carts.DeleteItem(middleware.UserID(c), uint(id)); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}
