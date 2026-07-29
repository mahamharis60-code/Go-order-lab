package service

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"go-order-lab/internal/config"
	"go-order-lab/internal/database"
	"go-order-lab/internal/model"
	"gorm.io/gorm"
)

func newServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.Open(config.Config{
		DBDriver:  "sqlite",
		DBSource:  filepath.Join(t.TempDir(), "test.db"),
		JWTSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
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

func seedActivity(t *testing.T, db *gorm.DB, stock int) model.Activity {
	t.Helper()
	catalog := NewCatalogService(db)
	product, err := catalog.CreateProduct(CreateProductInput{Name: "demo-phone", Price: 299900, Stock: 100})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	activity, err := catalog.CreateActivity(CreateActivityInput{
		ProductID:       product.ID,
		Name:            "demo-sale",
		Price:           269900,
		Stock:           stock,
		DurationSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}
	return activity
}

func TestActivityStatusRejectsOfflineOrders(t *testing.T) {
	db := newServiceTestDB(t)
	activity := seedActivity(t, db, 5)
	catalog := NewCatalogService(db)
	orders := NewOrderService(db, nil, nil)

	if activity.Status != model.ActivityStatusPublished {
		t.Fatalf("activity status = %q, want %q", activity.Status, model.ActivityStatusPublished)
	}
	if _, err := catalog.UpdateActivityStatus(activity.ID, model.ActivityStatusOffline); err != nil {
		t.Fatalf("offline activity: %v", err)
	}

	_, _, err := orders.CreateOrder(1001, CreateOrderInput{ActivityID: activity.ID, RequestID: "offline-req"})
	if !errors.Is(err, ErrActivityNotAvailable) {
		t.Fatalf("CreateOrder error = %v, want ErrActivityNotAvailable", err)
	}
}

func TestCompensateClosesWaitPayOrderAndRestoresActivityStock(t *testing.T) {
	db := newServiceTestDB(t)
	activity := seedActivity(t, db, 4)
	orders := NewOrderService(db, nil, nil)
	oldTime := time.Now().Add(-time.Hour)
	order := model.Order{
		OrderNo:        "ORD_COMP_CLOSE",
		RequestID:      "close-req",
		UserID:         1001,
		ProductID:      activity.ProductID,
		ActivityID:     activity.ID,
		OriginalAmount: activity.Price,
		Amount:         activity.Price,
		Status:         model.OrderStatusWaitPay,
		CreatedAt:      oldTime,
		UpdatedAt:      oldTime,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}
	if err := db.Model(&model.Activity{}).Where("id = ?", activity.ID).Update("stock", 3).Error; err != nil {
		t.Fatalf("reserve stock: %v", err)
	}

	result, err := orders.Compensate(CompensateInput{QueuedTimeoutSeconds: 3600, PayTimeoutSeconds: 0})
	if err != nil {
		t.Fatalf("compensate: %v", err)
	}
	if result.ClosedOrders != 1 {
		t.Fatalf("closed orders = %d, want 1", result.ClosedOrders)
	}

	var gotOrder model.Order
	if err := db.Where("order_no = ?", order.OrderNo).First(&gotOrder).Error; err != nil {
		t.Fatalf("query order: %v", err)
	}
	if gotOrder.Status != model.OrderStatusClosed {
		t.Fatalf("order status = %q, want %q", gotOrder.Status, model.OrderStatusClosed)
	}

	var gotActivity model.Activity
	if err := db.First(&gotActivity, activity.ID).Error; err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if gotActivity.Stock != 4 {
		t.Fatalf("activity stock = %d, want 4", gotActivity.Stock)
	}
}

func TestCompensateRequeuesStaleQueuedOrder(t *testing.T) {
	db := newServiceTestDB(t)
	activity := seedActivity(t, db, 3)
	orders := NewOrderService(db, nil, nil)
	oldTime := time.Now().Add(-time.Hour)
	order := model.Order{
		OrderNo:        "ORD_COMP_REQUEUE",
		RequestID:      "requeue-req",
		UserID:         1002,
		ProductID:      activity.ProductID,
		ActivityID:     activity.ID,
		OriginalAmount: activity.Price,
		Amount:         activity.Price,
		Status:         model.OrderStatusQueued,
		CreatedAt:      oldTime,
		UpdatedAt:      oldTime,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}

	result, err := orders.Compensate(CompensateInput{QueuedTimeoutSeconds: 0, PayTimeoutSeconds: 3600})
	if err != nil {
		t.Fatalf("compensate: %v", err)
	}
	if result.RequeuedOrders != 1 {
		t.Fatalf("requeued orders = %d, want 1", result.RequeuedOrders)
	}

	select {
	case task := <-orders.tasks:
		if task.OrderNo != order.OrderNo {
			t.Fatalf("task order = %q, want %q", task.OrderNo, order.OrderNo)
		}
	default:
		t.Fatal("expected requeue task")
	}
}

func TestCompensateMarksEndedActivities(t *testing.T) {
	db := newServiceTestDB(t)
	activity := seedActivity(t, db, 3)
	past := time.Now().Add(-time.Hour)
	if err := db.Model(&model.Activity{}).
		Where("id = ?", activity.ID).
		Updates(map[string]interface{}{"end_at": past, "status": model.ActivityStatusPublished}).Error; err != nil {
		t.Fatalf("set ended activity: %v", err)
	}
	orders := NewOrderService(db, nil, nil)

	result, err := orders.Compensate(CompensateInput{QueuedTimeoutSeconds: 3600, PayTimeoutSeconds: 3600})
	if err != nil {
		t.Fatalf("compensate: %v", err)
	}
	if result.EndedActivities != 1 {
		t.Fatalf("ended activities = %d, want 1", result.EndedActivities)
	}

	var got model.Activity
	if err := db.First(&got, activity.ID).Error; err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if got.Status != model.ActivityStatusEnded {
		t.Fatalf("activity status = %q, want %q", got.Status, model.ActivityStatusEnded)
	}
}
