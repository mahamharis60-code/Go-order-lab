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
