package service

import (
	"errors"

	"go-order-lab/internal/model"
	"gorm.io/gorm"
)

type AddressService struct {
	db *gorm.DB
}

func NewAddressService(db *gorm.DB) *AddressService {
	return &AddressService{db: db}
}

type CreateAddressInput struct {
	Receiver  string `json:"receiver" binding:"required"`
	Phone     string `json:"phone" binding:"required"`
	Province  string `json:"province"`
	City      string `json:"city"`
	Detail    string `json:"detail" binding:"required"`
	IsDefault bool   `json:"is_default"`
}

func (s *AddressService) Create(userID uint, input CreateAddressInput) (model.Address, error) {
	var address model.Address
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if input.IsDefault {
			if err := tx.Model(&model.Address{}).Where("user_id = ?", userID).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		address = model.Address{
			UserID:    userID,
			Receiver:  input.Receiver,
			Phone:     input.Phone,
			Province:  input.Province,
			City:      input.City,
			Detail:    input.Detail,
			IsDefault: input.IsDefault,
		}
		return tx.Create(&address).Error
	})
	return address, err
}

func (s *AddressService) List(userID uint) ([]model.Address, error) {
	var addresses []model.Address
	err := s.db.Where("user_id = ?", userID).Order("is_default desc, id desc").Find(&addresses).Error
	return addresses, err
}

func (s *AddressService) SetDefault(userID, addressID uint) (model.Address, error) {
	var address model.Address
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND user_id = ?", addressID, userID).First(&address).Error; err != nil {
			return errors.New("address not found")
		}
		if err := tx.Model(&model.Address{}).Where("user_id = ?", userID).Update("is_default", false).Error; err != nil {
			return err
		}
		address.IsDefault = true
		return tx.Save(&address).Error
	})
	return address, err
}
