package config

import "testing"

func TestLoadReadsDatabasePoolSettings(t *testing.T) {
	t.Setenv("ORDER_DB_MAX_OPEN_CONNS", "16")
	t.Setenv("ORDER_DB_MAX_IDLE_CONNS", "8")
	t.Setenv("ORDER_DB_CONN_MAX_LIFETIME_SECONDS", "120")
	t.Setenv("ORDER_REQUEST_TIMEOUT_SECONDS", "6")
	t.Setenv("ORDER_SHUTDOWN_TIMEOUT_SECONDS", "15")

	cfg := Load()

	if cfg.DBMaxOpenConns != 16 {
		t.Fatalf("DBMaxOpenConns = %d, want 16", cfg.DBMaxOpenConns)
	}
	if cfg.DBMaxIdleConns != 8 {
		t.Fatalf("DBMaxIdleConns = %d, want 8", cfg.DBMaxIdleConns)
	}
	if cfg.DBConnMaxLifetimeSeconds != 120 {
		t.Fatalf("DBConnMaxLifetimeSeconds = %d, want 120", cfg.DBConnMaxLifetimeSeconds)
	}
	if cfg.RequestTimeoutSeconds != 6 {
		t.Fatalf("RequestTimeoutSeconds = %d, want 6", cfg.RequestTimeoutSeconds)
	}
	if cfg.ShutdownTimeoutSeconds != 15 {
		t.Fatalf("ShutdownTimeoutSeconds = %d, want 15", cfg.ShutdownTimeoutSeconds)
	}
}
