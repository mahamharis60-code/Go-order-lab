package service

import (
	"errors"
	"testing"

	"go-order-lab/internal/model"
)

func TestActivityOrderUniqueKeyRejectsDuplicateActivityOrders(t *testing.T) {
	db := newServiceTestDB(t)
	activity := seedActivity(t, db, 5)
	key := model.NewActivityOrderKey(1001, activity.ID)

	first := model.Order{
		OrderNo:          "ORD_UNIQUE_FIRST",
		UserID:           1001,
		ProductID:        activity.ProductID,
		ActivityID:       activity.ID,
		ActivityOrderKey: key,
		Amount:           activity.Price,
		Status:           model.OrderStatusQueued,
	}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first activity order: %v", err)
	}

	second := model.Order{
		OrderNo:          "ORD_UNIQUE_SECOND",
		UserID:           1001,
		ProductID:        activity.ProductID,
		ActivityID:       activity.ID,
		ActivityOrderKey: key,
		Amount:           activity.Price,
		Status:           model.OrderStatusQueued,
	}
	if err := db.Create(&second).Error; err == nil {
		t.Fatal("create duplicate activity order succeeded, want unique constraint error")
	}
}

func TestCartOrdersAllowMultipleNullActivityOrderKeys(t *testing.T) {
	db := newServiceTestDB(t)

	first := model.Order{
		OrderNo:   "ORD_CART_FIRST",
		UserID:    1001,
		ProductID: 2001,
		Amount:    100,
		Status:    model.OrderStatusWaitPay,
	}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first cart order: %v", err)
	}

	second := model.Order{
		OrderNo:   "ORD_CART_SECOND",
		UserID:    1001,
		ProductID: 2002,
		Amount:    200,
		Status:    model.OrderStatusWaitPay,
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second cart order: %v", err)
	}
}

func TestCreateOrderMapsDatabaseUniqueConflictToDuplicateOrder(t *testing.T) {
	db := newServiceTestDB(t)
	activity := seedActivity(t, db, 5)
	key := model.NewActivityOrderKey(1001, activity.ID)
	existing := model.Order{
		OrderNo:          "ORD_EXISTING_ACTIVITY",
		UserID:           1001,
		ProductID:        activity.ProductID,
		ActivityID:       activity.ID,
		ActivityOrderKey: key,
		Amount:           activity.Price,
		Status:           model.OrderStatusQueued,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing order: %v", err)
	}

	orders := NewOrderService(db, nil, nil)
	_, _, err := orders.CreateOrder(1001, CreateOrderInput{ActivityID: activity.ID, RequestID: "dup-db-guard"})
	if !errors.Is(err, ErrDuplicateOrder) {
		t.Fatalf("CreateOrder error = %v, want ErrDuplicateOrder", err)
	}
}
