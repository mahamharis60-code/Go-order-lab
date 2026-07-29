package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go-order-lab/internal/middleware"
	"go-order-lab/internal/model"
	"gorm.io/gorm"
)

type AuthService struct {
	db        *gorm.DB
	jwtSecret string
}

func NewAuthService(db *gorm.DB, jwtSecret string) *AuthService {
	return &AuthService{db: db, jwtSecret: jwtSecret}
}

func (s *AuthService) Register(username, password string) (model.User, string, error) {
	if username == "" || len(password) < 6 {
		return model.User{}, "", errors.New("username and password length >= 6 are required")
	}
	user := model.User{
		Username:     username,
		PasswordHash: hashPassword(password),
		Role:         model.UserRoleUser,
	}
	if err := s.db.Create(&user).Error; err != nil {
		return model.User{}, "", err
	}
	token, err := s.signToken(user)
	return user, token, err
}

func (s *AuthService) Login(username, password string) (model.User, string, error) {
	var user model.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		return model.User{}, "", errors.New("invalid username or password")
	}
	if user.PasswordHash != hashPassword(password) {
		return model.User{}, "", errors.New("invalid username or password")
	}
	if !model.IsValidUserRole(user.Role) {
		user.Role = model.UserRoleUser
	}
	token, err := s.signToken(user)
	return user, token, err
}

func (s *AuthService) EnsureAdmin(username, password string) (model.User, error) {
	if username == "" || len(password) < 6 {
		return model.User{}, errors.New("admin username and password length >= 6 are required")
	}
	var user model.User
	err := s.db.Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		user = model.User{
			Username:     username,
			PasswordHash: hashPassword(password),
			Role:         model.UserRoleAdmin,
		}
		return user, s.db.Create(&user).Error
	}
	if err != nil {
		return model.User{}, err
	}
	user.PasswordHash = hashPassword(password)
	user.Role = model.UserRoleAdmin
	return user, s.db.Save(&user).Error
}

func (s *AuthService) signToken(user model.User) (string, error) {
	role := user.Role
	if !model.IsValidUserRole(role) {
		role = model.UserRoleUser
	}
	claims := middleware.Claims{
		UserID: user.ID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func hashPassword(password string) string {
	sum := sha256.Sum256([]byte("order-lab:" + password))
	return hex.EncodeToString(sum[:])
}
