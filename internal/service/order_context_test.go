package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCreateOrderContextRejectsCancelledRequest(t *testing.T) {
	db := newServiceTestDB(t)
	activity := seedActivity(t, db, 5)
	orders := NewOrderService(db, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := orders.CreateOrderContext(ctx, 1001, CreateOrderInput{ActivityID: activity.ID, RequestID: "cancelled-request"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateOrderContext error = %v, want context.Canceled", err)
	}
}

func TestCreateOrderUsesDatabasePathWhenRedisPointerIsNil(t *testing.T) {
	db := newServiceTestDB(t)
	activity := seedActivity(t, db, 5)
	var redisStock *RedisStockStore
	orders := NewOrderService(db, redisStock, nil)

	order, stockLeft, err := orders.CreateOrder(1001, CreateOrderInput{ActivityID: activity.ID, RequestID: "typed-nil-redis"})
	if err != nil {
		t.Fatalf("CreateOrder error = %v, want nil", err)
	}
	if order.Status != "QUEUED" {
		t.Fatalf("order status = %q, want QUEUED", order.Status)
	}
	if stockLeft != 4 {
		t.Fatalf("stock left = %d, want 4", stockLeft)
	}
}

func TestWorkersStopAfterContextCancellation(t *testing.T) {
	db := newServiceTestDB(t)
	orders := NewOrderService(db, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())

	orders.StartWorkersContext(ctx, 1)
	cancel()

	stopped := make(chan struct{})
	go func() {
		orders.Wait()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("workers did not stop after context cancellation")
	}
}
