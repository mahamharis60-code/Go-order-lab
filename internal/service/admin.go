package service

import (
	"go-order-lab/internal/model"
	"gorm.io/gorm"
)

type AdminService struct {
	db *gorm.DB
}

func NewAdminService(db *gorm.DB) *AdminService {
	return &AdminService{db: db}
}

type AdminOverview struct {
	Users           int64                `json:"users"`
	Products        int64                `json:"products"`
	Activities      int64                `json:"activities"`
	TotalOrders     int64                `json:"total_orders"`
	QueuedOrders    int64                `json:"queued_orders"`
	WaitPayOrders   int64                `json:"wait_pay_orders"`
	PaidOrders      int64                `json:"paid_orders"`
	CancelledOrders int64                `json:"cancelled_orders"`
	ClosedOrders    int64                `json:"closed_orders"`
	PaidGMV         int64                `json:"paid_gmv"`
	ProductStock    int64                `json:"product_stock"`
	ActivityStock   int64                `json:"activity_stock"`
	FailedLogs      int64                `json:"failed_logs"`
	RecentFailures  []model.OperationLog `json:"recent_failures"`
}

type AdminOrderQuery struct {
	Status     string `json:"status"`
	UserID     uint   `json:"user_id"`
	ActivityID uint   `json:"activity_id"`
	Limit      int    `json:"limit"`
}

type AdminOrderList struct {
	Total int64         `json:"total"`
	Items []model.Order `json:"items"`
}

func (s *AdminService) Overview() (AdminOverview, error) {
	var overview AdminOverview
	if err := s.db.Model(&model.User{}).Count(&overview.Users).Error; err != nil {
		return AdminOverview{}, err
	}
	if err := s.db.Model(&model.Product{}).Count(&overview.Products).Error; err != nil {
		return AdminOverview{}, err
	}
	if err := s.db.Model(&model.Activity{}).Count(&overview.Activities).Error; err != nil {
		return AdminOverview{}, err
	}
	if err := s.db.Model(&model.Order{}).Count(&overview.TotalOrders).Error; err != nil {
		return AdminOverview{}, err
	}
	if err := s.countOrdersByStatus(model.OrderStatusQueued, &overview.QueuedOrders); err != nil {
		return AdminOverview{}, err
	}
	if err := s.countOrdersByStatus(model.OrderStatusWaitPay, &overview.WaitPayOrders); err != nil {
		return AdminOverview{}, err
	}
	if err := s.countOrdersByStatus(model.OrderStatusPaid, &overview.PaidOrders); err != nil {
		return AdminOverview{}, err
	}
	if err := s.countOrdersByStatus(model.OrderStatusCancelled, &overview.CancelledOrders); err != nil {
		return AdminOverview{}, err
	}
	if err := s.countOrdersByStatus(model.OrderStatusClosed, &overview.ClosedOrders); err != nil {
		return AdminOverview{}, err
	}
	if err := s.db.Model(&model.Order{}).
		Where("status = ?", model.OrderStatusPaid).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&overview.PaidGMV).Error; err != nil {
		return AdminOverview{}, err
	}
	if err := s.db.Model(&model.Product{}).
		Select("COALESCE(SUM(stock), 0)").
		Scan(&overview.ProductStock).Error; err != nil {
		return AdminOverview{}, err
	}
	if err := s.db.Model(&model.Activity{}).
		Select("COALESCE(SUM(stock), 0)").
		Scan(&overview.ActivityStock).Error; err != nil {
		return AdminOverview{}, err
	}
	if err := s.db.Model(&model.OperationLog{}).
		Where("result = ?", "failed").
		Count(&overview.FailedLogs).Error; err != nil {
		return AdminOverview{}, err
	}
	if err := s.db.Where("result = ?", "failed").
		Order("id desc").
		Limit(5).
		Find(&overview.RecentFailures).Error; err != nil {
		return AdminOverview{}, err
	}
	return overview, nil
}

func (s *AdminService) ListOrders(input AdminOrderQuery) (AdminOrderList, error) {
	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	query := s.db.Model(&model.Order{})
	if input.Status != "" {
		query = query.Where("status = ?", input.Status)
	}
	if input.UserID > 0 {
		query = query.Where("user_id = ?", input.UserID)
	}
	if input.ActivityID > 0 {
		query = query.Where("activity_id = ?", input.ActivityID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return AdminOrderList{}, err
	}

	var orders []model.Order
	if err := query.Order("id desc").Limit(limit).Find(&orders).Error; err != nil {
		return AdminOrderList{}, err
	}
	return AdminOrderList{Total: total, Items: orders}, nil
}

func (s *AdminService) countOrdersByStatus(status string, count *int64) error {
	return s.db.Model(&model.Order{}).Where("status = ?", status).Count(count).Error
}
