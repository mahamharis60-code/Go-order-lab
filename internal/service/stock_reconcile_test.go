package service

import (
	"context"
	"testing"
	"time"

	"go-order-lab/internal/model"
)

type fakeStockCache struct {
	stocks map[uint]int
}

func newFakeStockCache() *fakeStockCache {
	return &fakeStockCache{stocks: map[uint]int{}}
}

func (f *fakeStockCache) Reserve(context.Context, model.Activity, uint) (int, error) {
	return 0, nil
}

func (f *fakeStockCache) Release(context.Context, uint, uint) error {
	return nil
}

func (f *fakeStockCache) Prewarm(ctx context.Context, activity model.Activity) error {
	return f.SetStock(ctx, activity.ID, activity.Stock, activity.EndAt)
}

func (f *fakeStockCache) SetStock(_ context.Context, activityID uint, stock int, _ time.Time) error {
	f.stocks[activityID] = stock
	return nil
}

func (f *fakeStockCache) GetStock(_ context.Context, activityID uint) (int, bool, error) {
	stock, ok := f.stocks[activityID]
	return stock, ok, nil
}

func TestReconcileStockReportsMissingRedisStock(t *testing.T) {
	db := newServiceTestDB(t)
	activity := seedActivity(t, db, 5)
	orders := NewOrderService(db, newFakeStockCache(), nil)

	result, err := orders.ReconcileStock(StockReconcileInput{ActivityID: activity.ID})
	if err != nil {
		t.Fatalf("reconcile stock: %v", err)
	}
	if result.Checked != 1 || result.Missing != 1 || result.Mismatched != 0 || result.Repaired != 0 {
		t.Fatalf("result = %+v, want checked=1 missing=1 mismatched=0 repaired=0", result)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.ActivityID != activity.ID || item.RedisExists {
		t.Fatalf("item = %+v, want missing redis stock for activity %d", item, activity.ID)
	}
}

func TestReconcileStockRepairsMismatch(t *testing.T) {
	db := newServiceTestDB(t)
	activity := seedActivity(t, db, 4)
	cache := newFakeStockCache()
	cache.stocks[activity.ID] = 1
	orders := NewOrderService(db, cache, nil)

	result, err := orders.ReconcileStock(StockReconcileInput{ActivityID: activity.ID, Repair: true})
	if err != nil {
		t.Fatalf("reconcile stock: %v", err)
	}
	if result.Checked != 1 || result.Mismatched != 1 || result.Repaired != 1 {
		t.Fatalf("result = %+v, want checked=1 mismatched=1 repaired=1", result)
	}
	if got := cache.stocks[activity.ID]; got != activity.Stock {
		t.Fatalf("redis stock = %d, want %d", got, activity.Stock)
	}
	if len(result.Items) != 1 || !result.Items[0].Repaired {
		t.Fatalf("items = %+v, want one repaired item", result.Items)
	}
}
