package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go-order-lab/internal/config"
	"go-order-lab/internal/database"
	"go-order-lab/internal/model"
	"go-order-lab/internal/service"
	"gorm.io/gorm"
)

func newHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.Open(config.Config{
		DBDriver:  "sqlite",
		DBSource:  filepath.Join(t.TempDir(), "handler.db"),
		JWTSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("open handler db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	return db
}

func TestAdminOverviewHandlerReturnsWrappedMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newHandlerTestDB(t)
	order := model.Order{
		OrderNo:        "ORD_HANDLER_PAID",
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
	adminHandler := NewAdminHandler(service.NewAdminService(db))
	router.GET("/admin/overview", adminHandler.Overview)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/overview", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code int `json:"code"`
		Data struct {
			TotalOrders int64 `json:"total_orders"`
			PaidGMV     int64 `json:"paid_gmv"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != 0 || body.Data.TotalOrders != 1 || body.Data.PaidGMV != 10000 {
		t.Fatalf("response = %+v, want code=0 total_orders=1 paid_gmv=10000", body)
	}
}

func TestAdminListOrdersHandlerBindsQueryFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newHandlerTestDB(t)
	orders := []model.Order{
		{OrderNo: "ORD_HANDLER_MATCH", UserID: 1001, ProductID: 2001, ActivityID: 3001, Amount: 10000, Status: model.OrderStatusWaitPay, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{OrderNo: "ORD_HANDLER_OTHER", UserID: 1002, ProductID: 2001, ActivityID: 3001, Amount: 10000, Status: model.OrderStatusWaitPay, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	if err := db.Create(&orders).Error; err != nil {
		t.Fatalf("create orders: %v", err)
	}

	router := gin.New()
	adminHandler := NewAdminHandler(service.NewAdminService(db))
	router.GET("/admin/orders", adminHandler.ListOrders)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/orders?status=WAIT_PAY&user_id=1001&activity_id=3001&limit=5", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data struct {
			Total int64 `json:"total"`
			Items []struct {
				OrderNo string `json:"order_no"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.Total != 1 || len(body.Data.Items) != 1 || body.Data.Items[0].OrderNo != "ORD_HANDLER_MATCH" {
		t.Fatalf("response = %+v, want one matched order", body.Data)
	}
}
