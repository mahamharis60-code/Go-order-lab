package service

import (
	"testing"
	"time"

	"go-order-lab/internal/model"
)

func TestAdminOverviewCountsPaidGMVAndFailureLogs(t *testing.T) {
	db := newServiceTestDB(t)
	activity := seedActivity(t, db, 6)
	now := time.Now()

	orders := []model.Order{
		{
			OrderNo:        "ORD_ADMIN_PAID",
			UserID:         1001,
			ProductID:      activity.ProductID,
			ActivityID:     activity.ID,
			OriginalAmount: 12000,
			Amount:         10000,
			Status:         model.OrderStatusPaid,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			OrderNo:        "ORD_ADMIN_WAIT_PAY",
			UserID:         1002,
			ProductID:      activity.ProductID,
			ActivityID:     activity.ID,
			OriginalAmount: 9000,
			Amount:         9000,
			Status:         model.OrderStatusWaitPay,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			OrderNo:        "ORD_ADMIN_CLOSED",
			UserID:         1003,
			ProductID:      activity.ProductID,
			ActivityID:     activity.ID,
			OriginalAmount: 8000,
			Amount:         8000,
			Status:         model.OrderStatusClosed,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}
	if err := db.Create(&orders).Error; err != nil {
		t.Fatalf("create admin orders: %v", err)
	}
	logs := []model.OperationLog{
		{Action: "payment_callback", OrderNo: "ORD_ADMIN_PAID", UserID: 1001, Result: "success", Message: "paid"},
		{Action: "stock_reconcile_mismatch", OrderNo: "", UserID: 0, Result: "failed", Message: "stock mismatch"},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("create admin logs: %v", err)
	}

	admin := NewAdminService(db)
	overview, err := admin.Overview()
	if err != nil {
		t.Fatalf("admin overview: %v", err)
	}

	if overview.TotalOrders != 3 {
		t.Fatalf("total orders = %d, want 3", overview.TotalOrders)
	}
	if overview.WaitPayOrders != 1 || overview.PaidOrders != 1 || overview.ClosedOrders != 1 {
		t.Fatalf("status counts = wait_pay:%d paid:%d closed:%d, want 1/1/1", overview.WaitPayOrders, overview.PaidOrders, overview.ClosedOrders)
	}
	if overview.PaidGMV != 10000 {
		t.Fatalf("paid gmv = %d, want only paid amount 10000", overview.PaidGMV)
	}
	if overview.FailedLogs != 1 {
		t.Fatalf("failed logs = %d, want 1", overview.FailedLogs)
	}
}

func TestAdminListOrdersFiltersByStatusUserAndActivity(t *testing.T) {
	db := newServiceTestDB(t)
	activity := seedActivity(t, db, 6)
	otherActivity := seedActivity(t, db, 6)
	now := time.Now()

	orders := []model.Order{
		{OrderNo: "ORD_ADMIN_MATCH", UserID: 1001, ProductID: activity.ProductID, ActivityID: activity.ID, Amount: 10000, Status: model.OrderStatusWaitPay, CreatedAt: now, UpdatedAt: now},
		{OrderNo: "ORD_ADMIN_OTHER_USER", UserID: 1002, ProductID: activity.ProductID, ActivityID: activity.ID, Amount: 10000, Status: model.OrderStatusWaitPay, CreatedAt: now, UpdatedAt: now},
		{OrderNo: "ORD_ADMIN_OTHER_STATUS", UserID: 1001, ProductID: activity.ProductID, ActivityID: activity.ID, Amount: 10000, Status: model.OrderStatusPaid, CreatedAt: now, UpdatedAt: now},
		{OrderNo: "ORD_ADMIN_OTHER_ACTIVITY", UserID: 1001, ProductID: otherActivity.ProductID, ActivityID: otherActivity.ID, Amount: 10000, Status: model.OrderStatusWaitPay, CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&orders).Error; err != nil {
		t.Fatalf("create admin orders: %v", err)
	}

	admin := NewAdminService(db)
	result, err := admin.ListOrders(AdminOrderQuery{
		Status:     model.OrderStatusWaitPay,
		UserID:     1001,
		ActivityID: activity.ID,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("admin list orders: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("orders len = %d, want 1", len(result.Items))
	}
	if result.Items[0].OrderNo != "ORD_ADMIN_MATCH" {
		t.Fatalf("order no = %q, want ORD_ADMIN_MATCH", result.Items[0].OrderNo)
	}
	if result.Total != 1 {
		t.Fatalf("total = %d, want 1", result.Total)
	}
}
