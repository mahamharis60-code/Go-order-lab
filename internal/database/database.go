package database

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"go-order-lab/internal/config"
	"go-order-lab/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch cfg.DBDriver {
	case "mysql":
		dialector = mysql.Open(cfg.DBSource)
	case "sqlite", "":
		if err := os.MkdirAll(filepath.Dir(cfg.DBSource), 0755); err != nil {
			return nil, err
		}
		dialector = sqlite.Open(cfg.DBSource)
	default:
		return nil, fmt.Errorf("unsupported db driver: %s", cfg.DBDriver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if cfg.DBDriver == "sqlite" || cfg.DBDriver == "" {
		sqlDB.SetMaxOpenConns(1)
	} else {
		sqlDB.SetMaxOpenConns(cfg.DBMaxOpenConns)
		sqlDB.SetMaxIdleConns(cfg.DBMaxIdleConns)
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.DBConnMaxLifetimeSeconds) * time.Second)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.Address{},
		&model.Product{},
		&model.CartItem{},
		&model.Coupon{},
		&model.UserCoupon{},
		&model.Activity{},
		&model.Order{},
		&model.OrderItem{},
		&model.Payment{},
		&model.OperationLog{},
	); err != nil {
		return nil, err
	}
	if err := backfillActivityOrderKeys(db); err != nil {
		return nil, err
	}
	if err := db.Model(&model.Activity{}).
		Where("status = '' OR status IS NULL").
		Update("status", model.ActivityStatusPublished).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&model.User{}).
		Where("role = '' OR role IS NULL").
		Update("role", model.UserRoleUser).Error; err != nil {
		return nil, err
	}
	if db.Migrator().HasIndex(&model.Order{}, "idx_user_activity") {
		_ = db.Migrator().DropIndex(&model.Order{}, "idx_user_activity")
	}
	return db, nil
}

func backfillActivityOrderKeys(db *gorm.DB) error {
	var orders []model.Order
	if err := db.
		Where("activity_id > 0 AND (activity_order_key = '' OR activity_order_key IS NULL)").
		Find(&orders).Error; err != nil {
		return err
	}
	for _, order := range orders {
		key := model.NewActivityOrderKey(order.UserID, order.ActivityID)
		if key == nil {
			continue
		}
		if err := db.Model(&model.Order{}).
			Where("id = ?", order.ID).
			Update("activity_order_key", *key).Error; err != nil {
			return err
		}
	}
	return nil
}
