package service

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"go-order-lab/internal/middleware"
	"go-order-lab/internal/model"
)

func TestRegisterCreatesUserRoleToken(t *testing.T) {
	db := newServiceTestDB(t)
	auth := NewAuthService(db, "rbac-secret")

	user, token, err := auth.Register("normal-user", "123456")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	if user.Role != model.UserRoleUser {
		t.Fatalf("registered role = %q, want %q", user.Role, model.UserRoleUser)
	}

	claims := parseTestClaims(t, token, "rbac-secret")
	if claims.UserID != user.ID || claims.Role != model.UserRoleUser {
		t.Fatalf("claims = user_id:%d role:%q, want user_id:%d role:%q", claims.UserID, claims.Role, user.ID, model.UserRoleUser)
	}
}

func TestEnsureAdminCreatesAdminAndLoginCarriesRole(t *testing.T) {
	db := newServiceTestDB(t)
	auth := NewAuthService(db, "rbac-secret")

	admin, err := auth.EnsureAdmin("admin", "admin123456")
	if err != nil {
		t.Fatalf("ensure admin: %v", err)
	}
	if admin.Role != model.UserRoleAdmin {
		t.Fatalf("admin role = %q, want %q", admin.Role, model.UserRoleAdmin)
	}

	loginUser, token, err := auth.Login("admin", "admin123456")
	if err != nil {
		t.Fatalf("login admin: %v", err)
	}
	claims := parseTestClaims(t, token, "rbac-secret")
	if loginUser.ID != admin.ID || claims.UserID != admin.ID || claims.Role != model.UserRoleAdmin {
		t.Fatalf("login user=%d claims=%d/%q, want admin=%d/%q", loginUser.ID, claims.UserID, claims.Role, admin.ID, model.UserRoleAdmin)
	}
}

func parseTestClaims(t *testing.T, tokenText, secret string) *middleware.Claims {
	t.Helper()
	claims := &middleware.Claims{}
	token, err := jwt.ParseWithClaims(tokenText, claims, func(token *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		t.Fatalf("parse token: valid=%v err=%v", token != nil && token.Valid, err)
	}
	return claims
}
