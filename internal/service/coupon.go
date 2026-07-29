package service

import (
	"errors"
	"time"

	"go-order-lab/internal/model"
	"gorm.io/gorm"
)

type CouponService struct {
	db *gorm.DB
}

func NewCouponService(db *gorm.DB) *CouponService {
	return &CouponService{db: db}
}

type CreateCouponInput struct {
	Title           string `json:"title" binding:"required"`
	Threshold       int64  `json:"threshold"`
	Discount        int64  `json:"discount" binding:"required"`
	Stock           int    `json:"stock" binding:"required"`
	DurationSeconds int    `json:"duration_seconds"`
}

func (s *CouponService) Create(input CreateCouponInput) (model.Coupon, error) {
	if input.Discount <= 0 || input.Stock <= 0 {
		return model.Coupon{}, errors.New("discount and stock must be positive")
	}
	if input.DurationSeconds <= 0 {
		input.DurationSeconds = 7 * 24 * 3600
	}
	now := time.Now()
	coupon := model.Coupon{
		Title:     input.Title,
		Threshold: input.Threshold,
		Discount:  input.Discount,
		Stock:     input.Stock,
		StartAt:   now,
		EndAt:     now.Add(time.Duration(input.DurationSeconds) * time.Second),
	}
	return coupon, s.db.Create(&coupon).Error
}

func (s *CouponService) List() ([]model.Coupon, error) {
	var coupons []model.Coupon
	err := s.db.Order("id desc").Find(&coupons).Error
	return coupons, err
}

func (s *CouponService) Claim(userID, couponID uint) (model.UserCoupon, error) {
	var userCoupon model.UserCoupon
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var coupon model.Coupon
		if err := tx.First(&coupon, couponID).Error; err != nil {
			return errors.New("coupon not found")
		}
		now := time.Now()
		if now.Before(coupon.StartAt) || now.After(coupon.EndAt) {
			return errors.New("coupon is not active")
		}
		if coupon.Stock <= 0 {
			return errors.New("coupon sold out")
		}
		var count int64
		if err := tx.Model(&model.UserCoupon{}).Where("user_id = ? AND coupon_id = ?", userID, couponID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("coupon already claimed")
		}
		coupon.Stock--
		if err := tx.Save(&coupon).Error; err != nil {
			return err
		}
		userCoupon = model.UserCoupon{UserID: userID, CouponID: couponID, Status: model.UserCouponUnused}
		return tx.Create(&userCoupon).Error
	})
	if err != nil {
		return model.UserCoupon{}, err
	}
	_ = s.db.Preload("Coupon").First(&userCoupon, userCoupon.ID).Error
	return userCoupon, nil
}

func (s *CouponService) MyCoupons(userID uint) ([]model.UserCoupon, error) {
	var coupons []model.UserCoupon
	err := s.db.Preload("Coupon").Where("user_id = ?", userID).Order("id desc").Find(&coupons).Error
	return coupons, err
}
