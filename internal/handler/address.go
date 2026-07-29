package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go-order-lab/internal/middleware"
	"go-order-lab/internal/response"
	"go-order-lab/internal/service"
)

type AddressHandler struct {
	addresses *service.AddressService
}

func NewAddressHandler(addresses *service.AddressService) *AddressHandler {
	return &AddressHandler{addresses: addresses}
}

func (h *AddressHandler) CreateAddress(c *gin.Context) {
	var req service.CreateAddressInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	address, err := h.addresses.Create(middleware.UserID(c), req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Created(c, address)
}

func (h *AddressHandler) ListAddresses(c *gin.Context) {
	addresses, err := h.addresses.List(middleware.UserID(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, addresses)
}

func (h *AddressHandler) SetDefault(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid address id")
		return
	}
	address, err := h.addresses.SetDefault(middleware.UserID(c), uint(id))
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, address)
}
