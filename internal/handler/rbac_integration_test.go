package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go-order-lab/internal/middleware"
	"go-order-lab/internal/model"
	"go-order-lab/internal/service"
)

func TestAdminOverviewRequiresAdminRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newHandlerTestDB(t)
	secret := "rbac-secret"
	auth := service.NewAuthService(db, secret)

	_, userToken, err := auth.Register("buyer", "123456")
	if err != nil {
		t.Fatalf("register buyer: %v", err)
	}
	if _, err := auth.EnsureAdmin("admin", "admin123456"); err != nil {
		t.Fatalf("ensure admin: %v", err)
	}
	_, adminToken, err := auth.Login("admin", "admin123456")
	if err != nil {
		t.Fatalf("login admin: %v", err)
	}

	order := model.Order{
		OrderNo:        "ORD_RBAC_OVERVIEW",
		UserID:         1001,
		ProductID:      2001,
		ActivityID:     3001,
		OriginalAmount: 12000,
		Amount:         10000,
		Status:         model.OrderStatusPaid,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}

	router := gin.New()
	protected := router.Group("/api")
	protected.Use(middleware.Auth(secret))
	admin := protected.Group("/admin")
	admin.Use(middleware.RequireRole(model.UserRoleAdmin))
	adminHandler := NewAdminHandler(service.NewAdminService(db))
	admin.GET("/overview", adminHandler.Overview)

	assertAdminOverviewStatus(t, router, userToken, http.StatusForbidden)
	assertAdminOverviewStatus(t, router, adminToken, http.StatusOK)
}

func assertAdminOverviewStatus(t *testing.T, router http.Handler, token string, want int) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/overview", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, request)
	if recorder.Code != want {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, want, recorder.Body.String())
	}
}
