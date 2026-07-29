package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-order-lab/internal/model"
	"gorm.io/gorm"
)

var (
	ErrActivityNotFound     = errors.New("activity not found")
	ErrActivityNotAvailable = errors.New("activity is not available")
	ErrSoldOut              = errors.New("activity sold out")
	ErrDuplicateOrder       = errors.New("user already has an order for this activity")
	ErrOrderNotFound        = errors.New("order not found")
	ErrIllegalTransition    = errors.New("illegal order status transition")
)

type OrderService struct {
	db               *gorm.DB
	tasks            chan OrderTask
	redisStock       stockCache
	rabbit           *RabbitQueue
	rabbitMaxRetries int
	wg               sync.WaitGroup
}

type stockCache interface {
	Reserve(context.Context, model.Activity, uint) (int, error)
	Release(context.Context, uint, uint) error
	Prewarm(context.Context, model.Activity) error
	SetStock(context.Context, uint, int, time.Time) error
	GetStock(context.Context, uint) (int, bool, error)
}

type OrderTask struct {
	OrderNo string
}

type CreateOrderInput struct {
	ActivityID uint   `json:"activity_id" binding:"required"`
	RequestID  string `json:"request_id"`
}

type CheckoutCartInput struct {
	AddressID    uint `json:"address_id" binding:"required"`
	UserCouponID uint `json:"user_coupon_id"`
}

type ExpireOrdersInput struct {
	TimeoutSeconds int `json:"timeout_seconds"`
}

type PaymentCallbackInput struct {
	OrderNo       string `json:"order_no" binding:"required"`
	TransactionNo string `json:"transaction_no" binding:"required"`
	Status        string `json:"status" binding:"required"`
}

type CancelOrderResult struct {
	OrderNo       string `json:"order_no"`
	Status        string `json:"status"`
	ReturnedStock bool   `json:"returned_stock"`
}

type PaymentResult struct {
	OrderNo          string `json:"order_no"`
	TransactionNo    string `json:"transaction_no"`
	OrderStatus      string `json:"order_status"`
	AlreadyProcessed bool   `json:"already_processed"`
	Message          string `json:"message"`
}

type ExpireOrdersResult struct {
	ExpiredCount int `json:"expired_count"`
}

type CompensateInput struct {
	QueuedTimeoutSeconds int `json:"queued_timeout_seconds"`
	PayTimeoutSeconds    int `json:"pay_timeout_seconds"`
}

type CompensationResult struct {
	RequeuedOrders  int `json:"requeued_orders"`
	ClosedOrders    int `json:"closed_orders"`
	EndedActivities int `json:"ended_activities"`
	FailedCount     int `json:"failed_count"`
}

type StockReconcileInput struct {
	ActivityID uint `json:"activity_id"`
	Repair     bool `json:"repair"`
}

type StockReconcileResult struct {
	Checked    int                  `json:"checked"`
	Missing    int                  `json:"missing"`
	Mismatched int                  `json:"mismatched"`
	Repaired   int                  `json:"repaired"`
	Items      []StockReconcileItem `json:"items"`
}

type StockReconcileItem struct {
	ActivityID   uint   `json:"activity_id"`
	ActivityName string `json:"activity_name"`
	Status       string `json:"status"`
	MySQLStock   int    `json:"mysql_stock"`
	RedisStock   int    `json:"redis_stock"`
	RedisExists  bool   `json:"redis_exists"`
	Repaired     bool   `json:"repaired"`
}

type CompensationWorkerConfig struct {
	Enabled              bool
	IntervalSeconds      int
	QueuedTimeoutSeconds int
	PayTimeoutSeconds    int
}

func NewOrderService(db *gorm.DB, redisStock stockCache, rabbit *RabbitQueue) *OrderService {
	return &OrderService{
		db:               db,
		tasks:            make(chan OrderTask, 128),
		redisStock:       redisStock,
		rabbit:           rabbit,
		rabbitMaxRetries: 3,
	}
}

func (s *OrderService) SetRabbitMaxRetries(n int) {
	if n < 0 {
		n = 0
	}
	s.rabbitMaxRetries = n
}

func (s *OrderService) StartWorkers(n int) {
	s.StartWorkersContext(context.Background(), n)
}

func (s *OrderService) StartWorkersContext(ctx context.Context, n int) {
	if ctx == nil {
		ctx = context.Background()
	}
	if n <= 0 {
		n = 1
	}
	if s.rabbit != nil {
		err := s.startRabbitWorkers(ctx, n)
		if err == nil {
			return
		}
		s.log("rabbitmq_consume_fallback", "", 0, "failed", err.Error())
	}
	for i := 0; i < n; i++ {
		workerID := i + 1
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.worker(ctx, workerID)
		}()
	}
}

func (s *OrderService) StartCompensationWorker(cfg CompensationWorkerConfig) {
	s.StartCompensationWorkerContext(context.Background(), cfg)
}

func (s *OrderService) StartCompensationWorkerContext(ctx context.Context, cfg CompensationWorkerConfig) {
	if !cfg.Enabled {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg.IntervalSeconds <= 0 {
		cfg.IntervalSeconds = 60
	}
	if cfg.QueuedTimeoutSeconds < 0 {
		cfg.QueuedTimeoutSeconds = 0
	}
	if cfg.PayTimeoutSeconds < 0 {
		cfg.PayTimeoutSeconds = 0
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(time.Duration(cfg.IntervalSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				s.log("compensation_worker_stopped", "", 0, "success", ctx.Err().Error())
				return
			case <-ticker.C:
			}
			result, err := s.Compensate(CompensateInput{
				QueuedTimeoutSeconds: cfg.QueuedTimeoutSeconds,
				PayTimeoutSeconds:    cfg.PayTimeoutSeconds,
			})
			if err != nil {
				s.log("compensation_worker", "", 0, "failed", err.Error())
				continue
			}
			s.log("compensation_worker", "", 0, "success", fmt.Sprintf("requeued=%d closed=%d ended=%d failed=%d", result.RequeuedOrders, result.ClosedOrders, result.EndedActivities, result.FailedCount))
		}
	}()
	s.log("compensation_worker_started", "", 0, "success", fmt.Sprintf("interval=%ds queued_timeout=%ds pay_timeout=%ds", cfg.IntervalSeconds, cfg.QueuedTimeoutSeconds, cfg.PayTimeoutSeconds))
}

func (s *OrderService) Wait() {
	s.wg.Wait()
}

func (s *OrderService) CreateOrder(userID uint, input CreateOrderInput) (model.Order, int, error) {
	return s.CreateOrderContext(context.Background(), userID, input)
}

func (s *OrderService) CreateOrderContext(ctx context.Context, userID uint, input CreateOrderInput) (model.Order, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return model.Order{}, 0, err
	}
	if input.RequestID == "" {
		input.RequestID = "req_" + randomText(8)
	}
	if s.redisStock != nil {
		return s.createOrderWithRedis(ctx, userID, input)
	}
	return s.createOrderWithDB(ctx, userID, input)
}

func (s *OrderService) createOrderWithDB(ctx context.Context, userID uint, input CreateOrderInput) (model.Order, int, error) {
	var order model.Order
	stockLeft := 0
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var activity model.Activity
		if err := tx.First(&activity, input.ActivityID).Error; err != nil {
			return ErrActivityNotFound
		}
		now := time.Now()
		if err := ensureActivityOrderable(activity, now); err != nil {
			return err
		}
		if activity.Stock <= 0 {
			return ErrSoldOut
		}

		var count int64
		if err := tx.Model(&model.Order{}).
			Where("user_id = ? AND activity_id = ?", userID, input.ActivityID).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrDuplicateOrder
		}

		activity.Stock--
		if err := tx.Save(&activity).Error; err != nil {
			return err
		}
		stockLeft = activity.Stock

		order = model.Order{
			OrderNo:          newOrderNo(),
			RequestID:        input.RequestID,
			UserID:           userID,
			ProductID:        activity.ProductID,
			ActivityID:       activity.ID,
			ActivityOrderKey: model.NewActivityOrderKey(userID, activity.ID),
			OriginalAmount:   activity.Price,
			Amount:           activity.Price,
			Status:           model.OrderStatusQueued,
		}
		if err := tx.Create(&order).Error; err != nil {
			if isActivityOrderUniqueConflict(err) {
				return ErrDuplicateOrder
			}
			return err
		}
		return nil
	})
	if err != nil {
		s.log("create_order_rejected", "", userID, "failed", err.Error())
		return model.Order{}, 0, err
	}

	s.publishOrderTask(order.OrderNo)
	s.log("stock_reserved", order.OrderNo, userID, "success", fmt.Sprintf("stock_left=%d", stockLeft))
	return order, stockLeft, nil
}

func (s *OrderService) createOrderWithRedis(ctx context.Context, userID uint, input CreateOrderInput) (model.Order, int, error) {
	db := s.db.WithContext(ctx)
	var activity model.Activity
	if err := db.First(&activity, input.ActivityID).Error; err != nil {
		s.log("create_order_rejected", "", userID, "failed", ErrActivityNotFound.Error())
		return model.Order{}, 0, ErrActivityNotFound
	}
	now := time.Now()
	if err := ensureActivityOrderable(activity, now); err != nil {
		s.log("create_order_rejected", "", userID, "failed", err.Error())
		return model.Order{}, 0, err
	}

	reserveCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	stockLeft, err := s.redisStock.Reserve(reserveCtx, activity, userID)
	if err != nil {
		s.log("create_order_rejected", "", userID, "failed", err.Error())
		return model.Order{}, 0, err
	}

	var order model.Order
	err = db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.Order{}).
			Where("user_id = ? AND activity_id = ?", userID, input.ActivityID).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrDuplicateOrder
		}

		updateNow := time.Now()
		result := tx.Model(&model.Activity{}).
			Where("id = ? AND stock > 0 AND (status = ? OR status = '') AND start_at <= ? AND end_at >= ?", input.ActivityID, model.ActivityStatusPublished, updateNow, updateNow).
			UpdateColumn("stock", gorm.Expr("stock - ?", 1))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var current model.Activity
			if err := tx.First(&current, input.ActivityID).Error; err == nil && current.Stock <= 0 {
				return ErrSoldOut
			}
			return ErrActivityNotAvailable
		}

		order = model.Order{
			OrderNo:          newOrderNo(),
			RequestID:        input.RequestID,
			UserID:           userID,
			ProductID:        activity.ProductID,
			ActivityID:       activity.ID,
			ActivityOrderKey: model.NewActivityOrderKey(userID, activity.ID),
			OriginalAmount:   activity.Price,
			Amount:           activity.Price,
			Status:           model.OrderStatusQueued,
		}
		if err := tx.Create(&order).Error; err != nil {
			if isActivityOrderUniqueConflict(err) {
				return ErrDuplicateOrder
			}
			return err
		}
		return nil
	})
	if err != nil {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = s.redisStock.Release(releaseCtx, input.ActivityID, userID)
		releaseCancel()
		s.log("create_order_rejected", "", userID, "failed", err.Error())
		return model.Order{}, 0, err
	}

	s.publishOrderTask(order.OrderNo)
	s.log("stock_reserved_redis", order.OrderNo, userID, "success", fmt.Sprintf("stock_left=%d", stockLeft))
	return order, stockLeft, nil
}

func (s *OrderService) CheckoutCart(userID uint, input CheckoutCartInput) (model.Order, error) {
	var order model.Order
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var address model.Address
		if err := tx.Where("id = ? AND user_id = ?", input.AddressID, userID).First(&address).Error; err != nil {
			return errors.New("address not found")
		}

		var cartItems []model.CartItem
		if err := tx.Preload("Product").Where("user_id = ?", userID).Find(&cartItems).Error; err != nil {
			return err
		}
		if len(cartItems) == 0 {
			return errors.New("cart is empty")
		}

		originalAmount := int64(0)
		for _, item := range cartItems {
			if item.Quantity <= 0 {
				return errors.New("invalid cart item quantity")
			}
			var product model.Product
			if err := tx.First(&product, item.ProductID).Error; err != nil {
				return errors.New("product not found")
			}
			if product.Stock < item.Quantity {
				return fmt.Errorf("product %s stock is not enough", product.Name)
			}
			product.Stock -= item.Quantity
			if err := tx.Save(&product).Error; err != nil {
				return err
			}
			item.Product = product
			originalAmount += product.Price * int64(item.Quantity)
		}

		discountAmount, err := s.applyCoupon(tx, userID, input.UserCouponID, originalAmount)
		if err != nil {
			return err
		}
		payAmount := originalAmount - discountAmount
		if payAmount < 0 {
			payAmount = 0
		}

		order = model.Order{
			OrderNo:        newOrderNo(),
			RequestID:      "cart_" + randomText(8),
			UserID:         userID,
			ProductID:      cartItems[0].ProductID,
			AddressID:      input.AddressID,
			UserCouponID:   input.UserCouponID,
			OriginalAmount: originalAmount,
			DiscountAmount: discountAmount,
			Amount:         payAmount,
			Status:         model.OrderStatusWaitPay,
		}
		if err := tx.Create(&order).Error; err != nil {
			return err
		}

		for _, item := range cartItems {
			orderItem := model.OrderItem{
				OrderID:     order.ID,
				OrderNo:     order.OrderNo,
				ProductID:   item.ProductID,
				ProductName: item.Product.Name,
				Price:       item.Product.Price,
				Quantity:    item.Quantity,
			}
			if err := tx.Create(&orderItem).Error; err != nil {
				return err
			}
		}
		return tx.Where("user_id = ?", userID).Delete(&model.CartItem{}).Error
	})
	if err != nil {
		s.log("checkout_cart_failed", "", userID, "failed", err.Error())
		return model.Order{}, err
	}
	s.log("checkout_cart", order.OrderNo, userID, "success", fmt.Sprintf("amount=%d discount=%d", order.Amount, order.DiscountAmount))
	return order, nil
}

func (s *OrderService) ListOrders(userID uint) ([]model.Order, error) {
	var orders []model.Order
	err := s.db.Where("user_id = ?", userID).Order("id desc").Find(&orders).Error
	return orders, err
}

func (s *OrderService) GetOrder(userID uint, orderNo string) (model.Order, error) {
	var order model.Order
	err := s.db.Where("user_id = ? AND order_no = ?", userID, orderNo).First(&order).Error
	if err != nil {
		return model.Order{}, ErrOrderNotFound
	}
	return order, nil
}

func (s *OrderService) CancelOrder(userID uint, orderNo string) (CancelOrderResult, error) {
	result := CancelOrderResult{OrderNo: orderNo}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order model.Order
		if err := tx.Where("user_id = ? AND order_no = ?", userID, orderNo).First(&order).Error; err != nil {
			return ErrOrderNotFound
		}
		if order.Status != model.OrderStatusQueued && order.Status != model.OrderStatusWaitPay {
			return ErrIllegalTransition
		}

		if err := s.restoreStock(tx, order); err != nil {
			return err
		}
		order.Status = model.OrderStatusCancelled
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		result.Status = order.Status
		result.ReturnedStock = true
		return nil
	})
	if err != nil {
		s.log("cancel_order_failed", orderNo, userID, "failed", err.Error())
		return CancelOrderResult{}, err
	}
	s.log("cancel_order", orderNo, userID, "success", "order cancelled and stock returned")
	return result, nil
}

func (s *OrderService) ExpireOrders(input ExpireOrdersInput) (ExpireOrdersResult, error) {
	if input.TimeoutSeconds < 0 {
		input.TimeoutSeconds = 0
	}
	cutoff := time.Now().Add(-time.Duration(input.TimeoutSeconds) * time.Second)
	var orders []model.Order
	if err := s.db.Where("status IN ? AND created_at <= ?", []string{model.OrderStatusQueued, model.OrderStatusWaitPay}, cutoff).Find(&orders).Error; err != nil {
		return ExpireOrdersResult{}, err
	}

	expired := 0
	for _, item := range orders {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var order model.Order
			if err := tx.Where("order_no = ?", item.OrderNo).First(&order).Error; err != nil {
				return err
			}
			if order.Status != model.OrderStatusQueued && order.Status != model.OrderStatusWaitPay {
				return nil
			}
			if err := s.restoreStock(tx, order); err != nil {
				return err
			}
			order.Status = model.OrderStatusClosed
			return tx.Save(&order).Error
		})
		if err != nil {
			s.log("expire_order_failed", item.OrderNo, item.UserID, "failed", err.Error())
			continue
		}
		expired++
		s.log("expire_order", item.OrderNo, item.UserID, "success", "order timeout closed and stock returned")
	}
	return ExpireOrdersResult{ExpiredCount: expired}, nil
}

func (s *OrderService) Compensate(input CompensateInput) (CompensationResult, error) {
	if input.QueuedTimeoutSeconds < 0 {
		input.QueuedTimeoutSeconds = 0
	}
	if input.PayTimeoutSeconds < 0 {
		input.PayTimeoutSeconds = 0
	}

	now := time.Now()
	result := CompensationResult{}
	queuedCutoff := now.Add(-time.Duration(input.QueuedTimeoutSeconds) * time.Second)
	var queuedOrders []model.Order
	if err := s.db.Where("status = ? AND updated_at <= ?", model.OrderStatusQueued, queuedCutoff).Find(&queuedOrders).Error; err != nil {
		return CompensationResult{}, err
	}
	for _, item := range queuedOrders {
		requeued := false
		err := s.db.Transaction(func(tx *gorm.DB) error {
			update := tx.Model(&model.Order{}).
				Where("order_no = ? AND status = ?", item.OrderNo, model.OrderStatusQueued).
				Update("updated_at", now)
			if update.Error != nil {
				return update.Error
			}
			requeued = update.RowsAffected > 0
			return nil
		})
		if err != nil {
			result.FailedCount++
			s.log("compensate_requeue_failed", item.OrderNo, item.UserID, "failed", err.Error())
			continue
		}
		if !requeued {
			continue
		}
		s.publishOrderTask(item.OrderNo)
		result.RequeuedOrders++
		s.log("compensate_requeue_order", item.OrderNo, item.UserID, "success", "stale queued order was published again")
	}

	payCutoff := now.Add(-time.Duration(input.PayTimeoutSeconds) * time.Second)
	var waitPayOrders []model.Order
	if err := s.db.Where("status = ? AND created_at <= ?", model.OrderStatusWaitPay, payCutoff).Find(&waitPayOrders).Error; err != nil {
		return result, err
	}
	for _, item := range waitPayOrders {
		closed := false
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var order model.Order
			if err := tx.Where("order_no = ?", item.OrderNo).First(&order).Error; err != nil {
				return err
			}
			if order.Status != model.OrderStatusWaitPay {
				return nil
			}
			if err := s.restoreStock(tx, order); err != nil {
				return err
			}
			order.Status = model.OrderStatusClosed
			if err := tx.Save(&order).Error; err != nil {
				return err
			}
			closed = true
			return nil
		})
		if err != nil {
			result.FailedCount++
			s.log("compensate_close_failed", item.OrderNo, item.UserID, "failed", err.Error())
			continue
		}
		if closed {
			result.ClosedOrders++
			s.log("compensate_close_order", item.OrderNo, item.UserID, "success", "timeout wait-pay order closed and stock returned")
		}
	}

	var activities []model.Activity
	if err := s.db.Where("(status = ? OR status = '') AND end_at <= ?", model.ActivityStatusPublished, now).Find(&activities).Error; err != nil {
		return result, err
	}
	for _, item := range activities {
		ended := false
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var activity model.Activity
			if err := tx.First(&activity, item.ID).Error; err != nil {
				return err
			}
			if activity.Status != model.ActivityStatusPublished && activity.Status != "" {
				return nil
			}
			activity.Status = model.ActivityStatusEnded
			if err := tx.Save(&activity).Error; err != nil {
				return err
			}
			ended = true
			return nil
		})
		if err != nil {
			result.FailedCount++
			s.log("compensate_end_activity_failed", "", 0, "failed", fmt.Sprintf("activity=%d reason=%s", item.ID, err.Error()))
			continue
		}
		if !ended {
			continue
		}
		result.EndedActivities++
		s.log("compensate_end_activity", "", 0, "success", fmt.Sprintf("activity=%d marked ENDED", item.ID))
	}

	return result, nil
}

func (s *OrderService) ReconcileStock(input StockReconcileInput) (StockReconcileResult, error) {
	if s.redisStock == nil {
		return StockReconcileResult{}, errors.New("redis stock store is not enabled")
	}

	var activities []model.Activity
	query := s.db.Order("id desc")
	if input.ActivityID > 0 {
		query = query.Where("id = ?", input.ActivityID)
	}
	if err := query.Find(&activities).Error; err != nil {
		return StockReconcileResult{}, err
	}

	result := StockReconcileResult{Items: make([]StockReconcileItem, 0)}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, activity := range activities {
		result.Checked++
		redisStock, exists, err := s.redisStock.GetStock(ctx, activity.ID)
		if err != nil {
			return result, err
		}
		item := StockReconcileItem{
			ActivityID:   activity.ID,
			ActivityName: activity.Name,
			Status:       activity.Status,
			MySQLStock:   activity.Stock,
			RedisStock:   redisStock,
			RedisExists:  exists,
		}

		if !exists {
			result.Missing++
			if input.Repair {
				if err := s.redisStock.SetStock(ctx, activity.ID, activity.Stock, activity.EndAt); err != nil {
					return result, err
				}
				item.Repaired = true
				result.Repaired++
				s.log("stock_reconcile_repair", "", 0, "success", fmt.Sprintf("activity=%d missing redis stock repaired to mysql_stock=%d", activity.ID, activity.Stock))
			} else {
				s.log("stock_reconcile_missing", "", 0, "failed", fmt.Sprintf("activity=%d mysql_stock=%d", activity.ID, activity.Stock))
			}
			result.Items = append(result.Items, item)
			continue
		}

		if redisStock != activity.Stock {
			result.Mismatched++
			if input.Repair {
				if err := s.redisStock.SetStock(ctx, activity.ID, activity.Stock, activity.EndAt); err != nil {
					return result, err
				}
				item.Repaired = true
				result.Repaired++
				s.log("stock_reconcile_repair", "", 0, "success", fmt.Sprintf("activity=%d redis_stock=%d mysql_stock=%d", activity.ID, redisStock, activity.Stock))
			} else {
				s.log("stock_reconcile_mismatch", "", 0, "failed", fmt.Sprintf("activity=%d redis_stock=%d mysql_stock=%d", activity.ID, redisStock, activity.Stock))
			}
			result.Items = append(result.Items, item)
		}
	}

	return result, nil
}

func (s *OrderService) PaymentCallback(input PaymentCallbackInput) (PaymentResult, error) {
	var result PaymentResult
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var payment model.Payment
		if err := tx.Where("transaction_no = ?", input.TransactionNo).First(&payment).Error; err == nil {
			var order model.Order
			_ = tx.Where("order_no = ?", payment.OrderNo).First(&order).Error
			result = PaymentResult{
				OrderNo:          payment.OrderNo,
				TransactionNo:    input.TransactionNo,
				OrderStatus:      order.Status,
				AlreadyProcessed: true,
				Message:          "duplicate callback ignored by transaction id",
			}
			return nil
		}

		var order model.Order
		if err := tx.Where("order_no = ?", input.OrderNo).First(&order).Error; err != nil {
			return ErrOrderNotFound
		}
		payment = model.Payment{
			TransactionNo: input.TransactionNo,
			OrderNo:       input.OrderNo,
			Status:        input.Status,
		}
		if input.Status != "SUCCESS" {
			if err := tx.Create(&payment).Error; err != nil {
				return err
			}
			result = PaymentResult{OrderNo: order.OrderNo, TransactionNo: input.TransactionNo, OrderStatus: order.Status, Message: "non-success callback recorded"}
			return nil
		}
		if order.Status != model.OrderStatusWaitPay {
			return ErrIllegalTransition
		}
		order.Status = model.OrderStatusPaid
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		if err := tx.Create(&payment).Error; err != nil {
			return err
		}
		result = PaymentResult{OrderNo: order.OrderNo, TransactionNo: input.TransactionNo, OrderStatus: order.Status, Message: "payment success, order status changed to PAID"}
		return nil
	})
	if err != nil {
		s.log("payment_callback_failed", input.OrderNo, 0, "failed", err.Error())
		return PaymentResult{}, err
	}
	s.log("payment_callback", result.OrderNo, 0, "success", result.Message)
	return result, nil
}

func (s *OrderService) applyCoupon(tx *gorm.DB, userID, userCouponID uint, originalAmount int64) (int64, error) {
	if userCouponID == 0 {
		return 0, nil
	}
	var userCoupon model.UserCoupon
	if err := tx.Preload("Coupon").Where("id = ? AND user_id = ?", userCouponID, userID).First(&userCoupon).Error; err != nil {
		return 0, errors.New("user coupon not found")
	}
	if userCoupon.Status != model.UserCouponUnused {
		return 0, errors.New("user coupon is not unused")
	}
	now := time.Now()
	if now.Before(userCoupon.Coupon.StartAt) || now.After(userCoupon.Coupon.EndAt) {
		userCoupon.Status = model.UserCouponExpired
		_ = tx.Save(&userCoupon).Error
		return 0, errors.New("coupon expired")
	}
	if originalAmount < userCoupon.Coupon.Threshold {
		return 0, errors.New("coupon threshold not reached")
	}
	userCoupon.Status = model.UserCouponUsed
	if err := tx.Save(&userCoupon).Error; err != nil {
		return 0, err
	}
	return userCoupon.Coupon.Discount, nil
}

func ensureActivityOrderable(activity model.Activity, now time.Time) error {
	status := activity.Status
	if status == "" {
		status = model.ActivityStatusPublished
	}
	if status != model.ActivityStatusPublished {
		return ErrActivityNotAvailable
	}
	if now.Before(activity.StartAt) || now.After(activity.EndAt) {
		return ErrActivityNotAvailable
	}
	return nil
}

func isActivityOrderUniqueConflict(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "activity_order_key") ||
		strings.Contains(text, "idx_orders_activity_order_key")
}

func (s *OrderService) restoreStock(tx *gorm.DB, order model.Order) error {
	if order.ActivityID > 0 {
		var activity model.Activity
		if err := tx.First(&activity, order.ActivityID).Error; err != nil {
			return err
		}
		activity.Stock++
		if err := tx.Save(&activity).Error; err != nil {
			return err
		}
		if s.redisStock != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return s.redisStock.Release(ctx, order.ActivityID, order.UserID)
		}
		return nil
	}

	var items []model.OrderItem
	if err := tx.Where("order_no = ?", order.OrderNo).Find(&items).Error; err != nil {
		return err
	}
	for _, item := range items {
		var product model.Product
		if err := tx.First(&product, item.ProductID).Error; err != nil {
			return err
		}
		product.Stock += item.Quantity
		if err := tx.Save(&product).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *OrderService) ListLogs() ([]model.OperationLog, error) {
	var logs []model.OperationLog
	err := s.db.Order("id desc").Limit(100).Find(&logs).Error
	return logs, err
}

func (s *OrderService) publishOrderTask(orderNo string) {
	if s.rabbit != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := s.rabbit.Publish(ctx, orderNo)
		cancel()
		if err == nil {
			s.log("order_task_published", orderNo, 0, "success", "published to RabbitMQ")
			return
		}
		s.log("order_task_publish_fallback", orderNo, 0, "failed", err.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	select {
	case s.tasks <- OrderTask{OrderNo: orderNo}:
	case <-ctx.Done():
		s.log("order_task_publish_failed", orderNo, 0, "failed", ctx.Err().Error())
	}
}

func (s *OrderService) startRabbitWorkers(ctx context.Context, n int) error {
	deliveries, err := s.rabbit.Consume()
	if err != nil {
		return err
	}
	for i := 0; i < n; i++ {
		workerID := i + 1
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			for {
				select {
				case <-ctx.Done():
					s.log("rabbitmq_worker_stopped", "", 0, "success", fmt.Sprintf("worker=%d reason=%s", workerID, ctx.Err().Error()))
					return
				case delivery, ok := <-deliveries:
					if !ok {
						return
					}
					msg, err := decodeRabbitOrderMessage(delivery.Body)
					if err != nil {
						s.log("order_task_decode_failed", "", 0, "failed", err.Error())
						_ = delivery.Nack(false, false)
						continue
					}
					if err := s.processOrderCreated(ctx, workerID, msg.OrderNo); err != nil {
						if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
							_ = delivery.Nack(false, true)
							return
						}
						if s.handleRabbitOrderFailure(ctx, msg, err) {
							_ = delivery.Ack(false)
							continue
						}
						_ = delivery.Nack(false, true)
						continue
					}
					_ = delivery.Ack(false)
				}
			}
		}()
	}
	s.log("rabbitmq_workers_started", "", 0, "success", fmt.Sprintf("workers=%d", n))
	return nil
}

func (s *OrderService) handleRabbitOrderFailure(ctx context.Context, msg RabbitOrderMessage, processErr error) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if msg.Retry < s.rabbitMaxRetries {
		next := msg
		next.Retry++
		if err := s.rabbit.PublishRetry(ctx, next); err != nil {
			s.log("order_task_retry_failed", msg.OrderNo, 0, "failed", err.Error())
			return false
		}
		s.log("order_task_retry", msg.OrderNo, 0, "success", fmt.Sprintf("retry=%d reason=%s", next.Retry, processErr.Error()))
		return true
	}
	if err := s.rabbit.PublishDeadLetter(ctx, msg, processErr.Error()); err != nil {
		s.log("order_task_dead_letter_failed", msg.OrderNo, 0, "failed", err.Error())
		return false
	}
	s.log("order_task_dead_letter", msg.OrderNo, 0, "failed", fmt.Sprintf("retry=%d reason=%s", msg.Retry, processErr.Error()))
	return true
}

func (s *OrderService) worker(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			s.log("order_worker_stopped", "", 0, "success", fmt.Sprintf("worker=%d reason=%s", workerID, ctx.Err().Error()))
			return
		case task := <-s.tasks:
			if err := s.processOrderCreated(ctx, workerID, task.OrderNo); errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
		}
	}
}

func (s *OrderService) processOrderCreated(ctx context.Context, workerID int, orderNo string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-time.After(120 * time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var order model.Order
		if err := tx.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return err
		}
		if order.Status != model.OrderStatusQueued {
			return nil
		}
		order.Status = model.OrderStatusWaitPay
		return tx.Save(&order).Error
	})
	if err != nil {
		s.log("order_created_async", orderNo, 0, "failed", err.Error())
		return err
	}
	s.log("order_created_async", orderNo, 0, "success", fmt.Sprintf("worker=%d moved order to WAIT_PAY", workerID))
	return nil
}

func (s *OrderService) log(action, orderNo string, userID uint, result, message string) {
	_ = s.db.Create(&model.OperationLog{
		Action:  action,
		OrderNo: orderNo,
		UserID:  userID,
		Result:  result,
		Message: message,
	}).Error
}

func randomText(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)[:n]
}

func newOrderNo() string {
	return "ORD" + time.Now().Format("20060102150405") + randomText(8)
}
