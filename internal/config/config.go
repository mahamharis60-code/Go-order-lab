package config

import (
	"os"
	"strconv"
)

type Config struct {
	Addr                             string
	DBDriver                         string
	DBSource                         string
	DBMaxOpenConns                   int
	DBMaxIdleConns                   int
	DBConnMaxLifetimeSeconds         int
	RequestTimeoutSeconds            int
	ShutdownTimeoutSeconds           int
	JWTSecret                        string
	AdminUsername                    string
	AdminPassword                    string
	WorkerCount                      int
	RedisEnabled                     bool
	RedisAddr                        string
	RedisPassword                    string
	RedisDB                          int
	RabbitEnabled                    bool
	RabbitURL                        string
	RabbitQueue                      string
	RabbitMaxRetries                 int
	CompensationEnabled              bool
	CompensationIntervalSeconds      int
	CompensationQueuedTimeoutSeconds int
	CompensationPayTimeoutSeconds    int
	RateLimitEnabled                 bool
	RateLimitRPS                     float64
	RateLimitBurst                   int
}

func Load() Config {
	cfg := Config{
		Addr:                             getenv("ORDER_HTTP_ADDR", ":8090"),
		DBDriver:                         getenv("ORDER_DB_DRIVER", "sqlite"),
		DBSource:                         getenv("ORDER_DB_SOURCE", "data/order_lab.db"),
		DBMaxOpenConns:                   getenvInt("ORDER_DB_MAX_OPEN_CONNS", 30),
		DBMaxIdleConns:                   getenvInt("ORDER_DB_MAX_IDLE_CONNS", 15),
		DBConnMaxLifetimeSeconds:         getenvInt("ORDER_DB_CONN_MAX_LIFETIME_SECONDS", 300),
		RequestTimeoutSeconds:            getenvInt("ORDER_REQUEST_TIMEOUT_SECONDS", 5),
		ShutdownTimeoutSeconds:           getenvInt("ORDER_SHUTDOWN_TIMEOUT_SECONDS", 10),
		JWTSecret:                        getenv("ORDER_JWT_SECRET", "dev-order-lab-secret"),
		AdminUsername:                    getenv("ORDER_ADMIN_USERNAME", "admin"),
		AdminPassword:                    getenv("ORDER_ADMIN_PASSWORD", "admin123456"),
		WorkerCount:                      getenvInt("ORDER_WORKER_COUNT", 2),
		RedisEnabled:                     getenvBool("ORDER_REDIS_ENABLED", false),
		RedisAddr:                        getenv("ORDER_REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:                    getenv("ORDER_REDIS_PASSWORD", ""),
		RedisDB:                          getenvInt("ORDER_REDIS_DB", 0),
		RabbitEnabled:                    getenvBool("ORDER_RABBITMQ_ENABLED", false),
		RabbitURL:                        getenv("ORDER_RABBITMQ_URL", "amqp://guest:guest@127.0.0.1:5672/"),
		RabbitQueue:                      getenv("ORDER_RABBITMQ_QUEUE", "order.created"),
		RabbitMaxRetries:                 getenvInt("ORDER_RABBITMQ_MAX_RETRIES", 3),
		CompensationEnabled:              getenvBool("ORDER_COMPENSATION_ENABLED", false),
		CompensationIntervalSeconds:      getenvInt("ORDER_COMPENSATION_INTERVAL_SECONDS", 60),
		CompensationQueuedTimeoutSeconds: getenvInt("ORDER_COMPENSATION_QUEUED_TIMEOUT_SECONDS", 30),
		CompensationPayTimeoutSeconds:    getenvInt("ORDER_COMPENSATION_PAY_TIMEOUT_SECONDS", 900),
		RateLimitEnabled:                 getenvBool("ORDER_RATE_LIMIT_ENABLED", false),
		RateLimitRPS:                     getenvFloat("ORDER_RATE_LIMIT_RPS", 20),
		RateLimitBurst:                   getenvInt("ORDER_RATE_LIMIT_BURST", 40),
	}
	return cfg
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvFloat(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
