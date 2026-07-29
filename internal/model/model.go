package model

import (
	"fmt"
	"time"
)

const (
	OrderStatusQueued    = "QUEUED"
	OrderStatusWaitPay   = "WAIT_PAY"
	OrderStatusPaid      = "PAID"
	OrderStatusCancelled = "CANCELLED"
	OrderStatusClosed    = "CLOSED"
)

const (
	ActivityStatusPublished = "PUBLISHED"
	ActivityStatusOffline   = "OFFLINE"
	ActivityStatusEnded     = "ENDED"
)

const (
	UserRoleUser  = "USER"
	UserRoleAdmin = "ADMIN"
)

func IsValidUserRole(role string) bool {
	switch role {
	case UserRoleUser, UserRoleAdmin:
		return true
	default:
		return false
	}
}

func IsValidActivityStatus(status string) bool {
	switch status {
	case ActivityStatusPublished, ActivityStatusOffline, ActivityStatusEnded:
		return true
	default:
		return false
	}
}

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"size:128;not null" json:"-"`
	Role         string    `gorm:"size:24;index;not null;default:USER" json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Product struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128;not null" json:"name"`
	Price     int64     `gorm:"not null" json:"price"`
	Stock     int       `gorm:"not null" json:"stock"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Address struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Receiver  string    `gorm:"size:64;not null" json:"receiver"`
	Phone     string    `gorm:"size:32;not null" json:"phone"`
	Province  string    `gorm:"size:64" json:"province"`
	City      string    `gorm:"size:64" json:"city"`
	Detail    string    `gorm:"size:255;not null" json:"detail"`
	IsDefault bool      `gorm:"not null" json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CartItem struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex:idx_user_product;not null" json:"user_id"`
	ProductID uint      `gorm:"uniqueIndex:idx_user_product;not null" json:"product_id"`
	Product   Product   `json:"product"`
	Quantity  int       `gorm:"not null" json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Coupon struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Title     string    `gorm:"size:128;not null" json:"title"`
	Threshold int64     `gorm:"not null" json:"threshold"`
	Discount  int64     `gorm:"not null" json:"discount"`
	Stock     int       `gorm:"not null" json:"stock"`
	StartAt   time.Time `gorm:"not null" json:"start_at"`
	EndAt     time.Time `gorm:"not null" json:"end_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const (
	UserCouponUnused  = "UNUSED"
	UserCouponUsed    = "USED"
	UserCouponExpired = "EXPIRED"
)

type UserCoupon struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex:idx_user_coupon;not null" json:"user_id"`
	CouponID  uint      `gorm:"uniqueIndex:idx_user_coupon;not null" json:"coupon_id"`
	Coupon    Coupon    `json:"coupon"`
	Status    string    `gorm:"size:24;index;not null" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Activity struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProductID uint      `gorm:"index;not null" json:"product_id"`
	Product   Product   `json:"-"`
	Name      string    `gorm:"size:128;not null" json:"name"`
	Price     int64     `gorm:"not null" json:"price"`
	Stock     int       `gorm:"not null" json:"stock"`
	Status    string    `gorm:"size:24;index;not null;default:PUBLISHED" json:"status"`
	StartAt   time.Time `gorm:"not null" json:"start_at"`
	EndAt     time.Time `gorm:"not null" json:"end_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Order struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	OrderNo          string    `gorm:"size:40;uniqueIndex;not null" json:"order_no"`
	RequestID        string    `gorm:"size:80;index" json:"request_id"`
	UserID           uint      `gorm:"index;not null" json:"user_id"`
	ProductID        uint      `gorm:"index;not null" json:"product_id"`
	ActivityID       uint      `gorm:"index;not null" json:"activity_id"`
	ActivityOrderKey *string   `gorm:"size:96;uniqueIndex" json:"-"`
	AddressID        uint      `gorm:"index" json:"address_id"`
	UserCouponID     uint      `gorm:"index" json:"user_coupon_id"`
	OriginalAmount   int64     `gorm:"not null;default:0" json:"original_amount"`
	DiscountAmount   int64     `gorm:"not null;default:0" json:"discount_amount"`
	Amount           int64     `gorm:"not null" json:"amount"`
	Status           string    `gorm:"size:24;index;not null" json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func NewActivityOrderKey(userID, activityID uint) *string {
	if userID == 0 || activityID == 0 {
		return nil
	}
	key := fmt.Sprintf("%d:%d", userID, activityID)
	return &key
}

type OrderItem struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	OrderID     uint      `gorm:"index;not null" json:"order_id"`
	OrderNo     string    `gorm:"size:40;index;not null" json:"order_no"`
	ProductID   uint      `gorm:"index;not null" json:"product_id"`
	ProductName string    `gorm:"size:128;not null" json:"product_name"`
	Price       int64     `gorm:"not null" json:"price"`
	Quantity    int       `gorm:"not null" json:"quantity"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Payment struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	TransactionNo string    `gorm:"size:80;uniqueIndex;not null" json:"transaction_no"`
	OrderNo       string    `gorm:"size:40;index;not null" json:"order_no"`
	Status        string    `gorm:"size:24;not null" json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type OperationLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Action    string    `gorm:"size:64;index;not null" json:"action"`
	OrderNo   string    `gorm:"size:40;index" json:"order_no"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Result    string    `gorm:"size:24;not null" json:"result"`
	Message   string    `gorm:"size:512" json:"message"`
	CreatedAt time.Time `json:"created_at"`
}
