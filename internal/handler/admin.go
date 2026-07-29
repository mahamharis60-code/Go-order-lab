package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go-order-lab/internal/response"
	"go-order-lab/internal/service"
)

type AdminHandler struct {
	admin *service.AdminService
}

func NewAdminHandler(admin *service.AdminService) *AdminHandler {
	return &AdminHandler{admin: admin}
}

func (h *AdminHandler) Overview(c *gin.Context) {
	overview, err := h.admin.Overview()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, overview)
}

func (h *AdminHandler) ListOrders(c *gin.Context) {
	query := service.AdminOrderQuery{
		Status:     c.Query("status"),
		UserID:     parseUintQuery(c, "user_id"),
		ActivityID: parseUintQuery(c, "activity_id"),
		Limit:      parseIntQuery(c, "limit"),
	}
	result, err := h.admin.ListOrders(query)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, result)
}

func parseUintQuery(c *gin.Context, key string) uint {
	value := c.Query(key)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0
	}
	return uint(parsed)
}

func parseIntQuery(c *gin.Context, key string) int {
	value := c.Query(key)
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}
