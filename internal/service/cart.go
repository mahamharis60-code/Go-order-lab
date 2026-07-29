package service

import (
	"errors"

	"go-order-lab/internal/model"
	"gorm.io/gorm"
)

type CartService struct {
	db *gorm.DB
}

func NewCartService(db *gorm.DB) *CartService {
	return &CartService{db: db}
}

type AddCartItemInput struct {
	ProductID uint `json:"product_id" binding:"required"`
	Quantity  int  `json:"quantity" binding:"required"`
}

func (s *CartService) AddItem(userID uint, input AddCartItemInput) (model.CartItem, error) {
	if input.Quantity <= 0 {
		return model.CartItem{}, errors.New("quantity must be positive")
	}
	var product model.Product
	if err := s.db.First(&product, input.ProductID).Error; err != nil {
		return model.CartItem{}, errors.New("product not found")
	}
	if product.Stock < input.Quantity {
		return model.CartItem{}, errors.New("product stock is not enough")
	}

	var item model.CartItem
	err := s.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Where("user_id = ? AND product_id = ?", userID, input.ProductID).First(&item).Error
		if err == nil {
			item.Quantity += input.Quantity
			return tx.Save(&item).Error
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		item = model.CartItem{UserID: userID, ProductID: input.ProductID, Quantity: input.Quantity}
		return tx.Create(&item).Error
	})
	if err != nil {
		return model.CartItem{}, err
	}
	_ = s.db.Preload("Product").First(&item, item.ID).Error
	return item, nil
}

func (s *CartService) List(userID uint) ([]model.CartItem, error) {
	var items []model.CartItem
	err := s.db.Preload("Product").Where("user_id = ?", userID).Order("id desc").Find(&items).Error
	return items, err
}

func (s *CartService) UpdateQuantity(userID, itemID uint, quantity int) (model.CartItem, error) {
	if quantity <= 0 {
		return model.CartItem{}, errors.New("quantity must be positive")
	}
	var item model.CartItem
	if err := s.db.Where("id = ? AND user_id = ?", itemID, userID).First(&item).Error; err != nil {
		return model.CartItem{}, errors.New("cart item not found")
	}
	item.Quantity = quantity
	if err := s.db.Save(&item).Error; err != nil {
		return model.CartItem{}, err
	}
	_ = s.db.Preload("Product").First(&item, item.ID).Error
	return item, nil
}

func (s *CartService) DeleteItem(userID, itemID uint) error {
	return s.db.Where("id = ? AND user_id = ?", itemID, userID).Delete(&model.CartItem{}).Error
}
