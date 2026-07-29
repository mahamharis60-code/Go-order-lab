package service

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go-order-lab/internal/model"
)

type RedisStockStore struct {
	client *redis.Client
}

var reserveStockScript = redis.NewScript(`
local stockKey = KEYS[1]
local usersKey = KEYS[2]
local initialStock = tonumber(ARGV[1])
local userID = ARGV[2]
local ttl = tonumber(ARGV[3])

if redis.call("EXISTS", stockKey) == 0 then
  redis.call("SET", stockKey, initialStock, "EX", ttl)
end
redis.call("EXPIRE", usersKey, ttl)

if redis.call("SISMEMBER", usersKey, userID) == 1 then
  return {-1, tonumber(redis.call("GET", stockKey) or "0")}
end

local stock = tonumber(redis.call("GET", stockKey) or "0")
if stock <= 0 then
  return {-2, stock}
end

local left = redis.call("DECR", stockKey)
redis.call("SADD", usersKey, userID)
redis.call("EXPIRE", usersKey, ttl)
return {0, left}
`)

var releaseStockScript = redis.NewScript(`
local stockKey = KEYS[1]
local usersKey = KEYS[2]
local userID = ARGV[1]
local removed = redis.call("SREM", usersKey, userID)
if removed == 1 then
  redis.call("INCR", stockKey)
end
return removed
`)

func NewRedisStockStore(addr, password string, db int) (*RedisStockStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &RedisStockStore{client: client}, nil
}

func (s *RedisStockStore) Reserve(ctx context.Context, activity model.Activity, userID uint) (int, error) {
	keys := []string{stockKey(activity.ID), usersKey(activity.ID)}
	result, err := reserveStockScript.Run(ctx, s.client, keys, activity.Stock, fmt.Sprintf("%d", userID), reserveTTL(activity.EndAt)).Result()
	if err != nil {
		return 0, err
	}
	values, ok := result.([]interface{})
	if !ok || len(values) != 2 {
		return 0, fmt.Errorf("unexpected redis reserve result: %v", result)
	}
	code := toInt(values[0])
	stockLeft := toInt(values[1])
	switch code {
	case 0:
		return stockLeft, nil
	case -1:
		return stockLeft, ErrDuplicateOrder
	case -2:
		return stockLeft, ErrSoldOut
	default:
		return stockLeft, fmt.Errorf("unexpected redis reserve code: %d", code)
	}
}

func (s *RedisStockStore) Prewarm(ctx context.Context, activity model.Activity) error {
	return s.SetStock(ctx, activity.ID, activity.Stock, activity.EndAt)
}

func (s *RedisStockStore) SetStock(ctx context.Context, activityID uint, stock int, endAt time.Time) error {
	ttl := reserveTTL(endAt)
	pipe := s.client.Pipeline()
	pipe.Set(ctx, stockKey(activityID), stock, time.Duration(ttl)*time.Second)
	pipe.Expire(ctx, usersKey(activityID), time.Duration(ttl)*time.Second)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStockStore) GetStock(ctx context.Context, activityID uint) (int, bool, error) {
	value, err := s.client.Get(ctx, stockKey(activityID)).Result()
	if err == redis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	var stock int
	if _, err := fmt.Sscanf(value, "%d", &stock); err != nil {
		return 0, true, fmt.Errorf("invalid redis stock value %q: %w", value, err)
	}
	return stock, true, nil
}

func (s *RedisStockStore) Release(ctx context.Context, activityID uint, userID uint) error {
	_, err := releaseStockScript.Run(ctx, s.client, []string{stockKey(activityID), usersKey(activityID)}, fmt.Sprintf("%d", userID)).Result()
	return err
}

func (s *RedisStockStore) Close() error {
	return s.client.Close()
}

func stockKey(activityID uint) string {
	return fmt.Sprintf("order:activity:%d:stock", activityID)
}

func usersKey(activityID uint) string {
	return fmt.Sprintf("order:activity:%d:users", activityID)
}

func reserveTTL(endAt time.Time) int64 {
	ttl := time.Until(endAt) + time.Hour
	if ttl < time.Hour {
		ttl = time.Hour
	}
	return int64(ttl.Seconds())
}

func toInt(value interface{}) int {
	switch v := value.(type) {
	case int64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}
