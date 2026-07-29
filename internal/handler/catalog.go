package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go-order-lab/internal/response"
	"go-order-lab/internal/service"
)

type CatalogHandler struct {
	catalog *service.CatalogService
}

func NewCatalogHandler(catalog *service.CatalogService) *CatalogHandler {
	return &CatalogHandler{catalog: catalog}
}

func (h *CatalogHandler) CreateProduct(c *gin.Context) {
	var req service.CreateProductInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	product, err := h.catalog.CreateProduct(req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Created(c, product)
}

func (h *CatalogHandler) ListProducts(c *gin.Context) {
	products, err := h.catalog.ListProducts()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, products)
}

func (h *CatalogHandler) CreateActivity(c *gin.Context) {
	var req service.CreateActivityInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	activity, err := h.catalog.CreateActivity(req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Created(c, activity)
}

func (h *CatalogHandler) ListActivities(c *gin.Context) {
	activities, err := h.catalog.ListActivities()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, activities)
}

func (h *CatalogHandler) UpdateActivityStatus(c *gin.Context) {
	activityID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid activity id")
		return
	}
	var req service.UpdateActivityStatusInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	activity, err := h.catalog.UpdateActivityStatus(uint(activityID), req.Status)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, activity)
}
