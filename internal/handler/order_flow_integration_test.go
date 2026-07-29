package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go-order-lab/internal/middleware"
	"go-order-lab/internal/model"
	"go-order-lab/internal/service"
	"gorm.io/gorm"
)

func TestOrderHTTPFlowCreatesOrderAndDeduplicatesPaymentCallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newHandlerTestDB(t)
	secret := "order-flow-secret"

	authService := service.NewAuthService(db, secret)
	catalogService := service.NewCatalogService(db)
	orderService := service.NewOrderService(db, nil, nil)
	workerCtx, stopWorkers := context.WithCancel(context.Background())
	orderService.StartWorkersContext(workerCtx, 1)
	t.Cleanup(func() {
		stopWorkers()
		orderService.Wait()
	})

	router := gin.New()
	authHandler := NewAuthHandler(authService)
	orderHandler := NewOrderHandler(orderService)

	api := router.Group("/api")
	api.POST("/auth/register", authHandler.Register)
	api.POST("/auth/login", authHandler.Login)
	api.POST("/payments/callback", orderHandler.PaymentCallback)
	protected := api.Group("")
	protected.Use(middleware.Auth(secret))
	protected.POST("/orders", orderHandler.CreateOrder)
	protected.GET("/orders/:order_no", orderHandler.GetOrder)

	product, err := catalogService.CreateProduct(service.CreateProductInput{
		Name:  "flow-phone",
		Price: 199900,
		Stock: 10,
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	activity, err := catalogService.CreateActivity(service.CreateActivityInput{
		ProductID:       product.ID,
		Name:            "flow-sale",
		Price:           99900,
		Stock:           2,
		DurationSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	registerResp := doJSON(t, router, http.MethodPost, "/api/auth/register", map[string]any{
		"username": "flow_user",
		"password": "123456",
	}, "")
	if registerResp.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body=%s", registerResp.Code, registerResp.Body.String())
	}
	token := jsonPath[string](t, registerResp.Body.Bytes(), "data", "token")

	orderResp := doJSON(t, router, http.MethodPost, "/api/orders", map[string]any{
		"activity_id": activity.ID,
		"request_id":  "req-flow-001",
	}, token)
	if orderResp.Code != http.StatusAccepted {
		t.Fatalf("create order status = %d, body=%s", orderResp.Code, orderResp.Body.String())
	}
	orderNo := jsonPath[string](t, orderResp.Body.Bytes(), "data", "order_no")
	stockLeft := jsonPath[float64](t, orderResp.Body.Bytes(), "data", "stock_left")
	if stockLeft != 1 {
		t.Fatalf("stock_left = %.0f, want 1", stockLeft)
	}

	duplicateResp := doJSON(t, router, http.MethodPost, "/api/orders", map[string]any{
		"activity_id": activity.ID,
		"request_id":  "req-flow-duplicate",
	}, token)
	if duplicateResp.Code != http.StatusConflict {
		t.Fatalf("duplicate order status = %d, want 409, body=%s", duplicateResp.Code, duplicateResp.Body.String())
	}

	waitForOrderStatus(t, db, orderNo, model.OrderStatusWaitPay)

	paymentResp := doJSON(t, router, http.MethodPost, "/api/payments/callback", map[string]any{
		"order_no":       orderNo,
		"transaction_no": "txn-flow-001",
		"status":         "SUCCESS",
	}, "")
	if paymentResp.Code != http.StatusOK {
		t.Fatalf("payment status = %d, body=%s", paymentResp.Code, paymentResp.Body.String())
	}
	if status := jsonPath[string](t, paymentResp.Body.Bytes(), "data", "order_status"); status != model.OrderStatusPaid {
		t.Fatalf("payment order_status = %s, want %s", status, model.OrderStatusPaid)
	}
	if alreadyProcessed := jsonPath[bool](t, paymentResp.Body.Bytes(), "data", "already_processed"); alreadyProcessed {
		t.Fatalf("first payment callback should not be marked already_processed")
	}

	duplicatePaymentResp := doJSON(t, router, http.MethodPost, "/api/payments/callback", map[string]any{
		"order_no":       orderNo,
		"transaction_no": "txn-flow-001",
		"status":         "SUCCESS",
	}, "")
	if duplicatePaymentResp.Code != http.StatusOK {
		t.Fatalf("duplicate payment status = %d, body=%s", duplicatePaymentResp.Code, duplicatePaymentResp.Body.String())
	}
	if alreadyProcessed := jsonPath[bool](t, duplicatePaymentResp.Body.Bytes(), "data", "already_processed"); !alreadyProcessed {
		t.Fatalf("duplicate payment callback should be marked already_processed")
	}
}

func doJSON(t *testing.T, router http.Handler, method, path string, payload any, token string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func jsonPath[T any](t *testing.T, body []byte, keys ...string) T {
	t.Helper()
	var current any
	if err := json.Unmarshal(body, &current); err != nil {
		t.Fatalf("decode json: %v, body=%s", err, string(body))
	}
	for _, key := range keys {
		obj, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("json path %v stopped at non-object %#v", keys, current)
		}
		current, ok = obj[key]
		if !ok {
			t.Fatalf("json path %v missing key %s in body=%s", keys, key, string(body))
		}
	}
	value, ok := current.(T)
	if !ok {
		t.Fatalf("json path %v type = %T, want target type", keys, current)
	}
	return value
}

func waitForOrderStatus(t *testing.T, db *gorm.DB, orderNo, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	last := "order not checked yet"
	for time.Now().Before(deadline) {
		var order model.Order
		if err := db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			last = err.Error()
		} else if order.Status == want {
			return
		} else {
			last = fmt.Sprintf("status=%s", order.Status)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("order %s did not reach status %s: %s", orderNo, want, last)
}
