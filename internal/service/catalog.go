package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"go-order-lab/internal/model"
	"gorm.io/gorm"
)

type CatalogService struct {
	db         *gorm.DB
	redisStock stockCache
}

func NewCatalogService(db *gorm.DB) *CatalogService {
	return &CatalogService{db: db}
}

func (s *CatalogService) SetRedisStockStore(redisStock stockCache) {
	s.redisStock = redisStock
}

type CreateProductInput struct {
	Name  string `json:"name" binding:"required"`
	Price int64  `json:"price" binding:"required"`
	Stock int    `json:"stock" binding:"min=0"`
}

type CreateActivityInput struct {
	ProductID       uint   `json:"product_id" binding:"required"`
	Name            string `json:"name" binding:"required"`
	Price           int64  `json:"price" binding:"required"`
	Stock           int    `json:"stock" binding:"required"`
	DurationSeconds int    `json:"duration_seconds"`
}

type UpdateActivityStatusInput struct {
	Status string `json:"status" binding:"required"`
}

func (s *CatalogService) CreateProduct(input CreateProductInput) (model.Product, error) {
	if input.Price <= 0 || input.Stock < 0 {
		return model.Product{}, errors.New("price must be positive and stock cannot be negative")
	}
	product := model.Product{Name: input.Name, Price: input.Price, Stock: input.Stock}
	return product, s.db.Create(&product).Error
}

func (s *CatalogService) ListProducts() ([]model.Product, error) {
	var products []model.Product
	err := s.db.Order("id desc").Find(&products).Error
	return products, err
}

func (s *CatalogService) CreateActivity(input CreateActivityInput) (model.Activity, error) {
	if input.Price <= 0 || input.Stock <= 0 {
		return model.Activity{}, errors.New("price and stock must be positive")
	}
	if input.DurationSeconds <= 0 {
		input.DurationSeconds = 24 * 3600
	}

	var product model.Product
	if err := s.db.First(&product, input.ProductID).Error; err != nil {
		return model.Activity{}, errors.New("product not found")
	}
	if input.Stock > product.Stock {
		return model.Activity{}, errors.New("activity stock cannot exceed product stock")
	}

	now := time.Now()
	activity := model.Activity{
		ProductID: input.ProductID,
		Name:      input.Name,
		Price:     input.Price,
		Stock:     input.Stock,
		Status:    model.ActivityStatusPublished,
		StartAt:   now,
		EndAt:     now.Add(time.Duration(input.DurationSeconds) * time.Second),
	}
	if err := s.db.Create(&activity).Error; err != nil {
		return model.Activity{}, err
	}
	if s.redisStock != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.redisStock.Prewarm(ctx, activity); err != nil {
			log.Printf("activity stock prewarm failed activity_id=%d err=%v", activity.ID, err)
		}
	}
	return activity, nil
}

func (s *CatalogService) ListActivities() ([]model.Activity, error) {
	var activities []model.Activity
	err := s.db.Order("id desc").Find(&activities).Error
	return activities, err
}

func (s *CatalogService) UpdateActivityStatus(activityID uint, status string) (model.Activity, error) {
	if !model.IsValidActivityStatus(status) {
		return model.Activity{}, fmt.Errorf("invalid activity status: %s", status)
	}
	var activity model.Activity
	if err := s.db.First(&activity, activityID).Error; err != nil {
		return model.Activity{}, errors.New("activity not found")
	}
	activity.Status = status
	return activity, s.db.Save(&activity).Error
}
