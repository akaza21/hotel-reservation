package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/akaza21/hotel-reservation/internal/logger"
	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	client *redis.Client
}

func NewRedisClient(addr, password string, db int) *RedisClient {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		PoolTimeout:  30 * time.Second,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
			"addr":  addr,
		}).Error("Failed to connect to Redis")
		return nil
	}

	logger.WithFields(map[string]interface{}{
		"addr": addr,
		"db":   db,
	}).Info("Connected to Redis successfully")

	return &RedisClient{client: rdb}
}

func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	jsonData, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	return r.client.Set(ctx, key, jsonData, expiration).Err()
}

func (r *RedisClient) Get(ctx context.Context, key string, dest interface{}) error {
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(val), dest)
}

func (r *RedisClient) Delete(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

func (r *RedisClient) Exists(ctx context.Context, key string) (bool, error) {
	result, err := r.client.Exists(ctx, key).Result()
	return result > 0, err
}

func (r *RedisClient) Increment(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

func (r *RedisClient) SetExpiration(ctx context.Context, key string, expiration time.Duration) error {
	return r.client.Expire(ctx, key, expiration).Err()
}

// Rate limiting functions
func (r *RedisClient) IsRateLimited(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	pipe := r.client.TxPipeline()
	
	// Increment the counter
	incrCmd := pipe.Incr(ctx, key)
	// Set expiration if it's a new key
	pipe.Expire(ctx, key, window)
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}

	count := incrCmd.Val()
	return count > limit, nil
}

// Hotel-specific cache functions
func (r *RedisClient) CacheHotel(ctx context.Context, hotelID string, hotel interface{}, ttl time.Duration) error {
	key := fmt.Sprintf("hotel:%s", hotelID)
	return r.Set(ctx, key, hotel, ttl)
}

func (r *RedisClient) GetCachedHotel(ctx context.Context, hotelID string, dest interface{}) error {
	key := fmt.Sprintf("hotel:%s", hotelID)
	return r.Get(ctx, key, dest)
}

func (r *RedisClient) InvalidateHotel(ctx context.Context, hotelID string) error {
	key := fmt.Sprintf("hotel:%s", hotelID)
	return r.Delete(ctx, key)
}

func (r *RedisClient) CacheHotelList(ctx context.Context, filters string, hotels interface{}, ttl time.Duration) error {
	key := fmt.Sprintf("hotels:list:%s", filters)
	return r.Set(ctx, key, hotels, ttl)
}

func (r *RedisClient) GetCachedHotelList(ctx context.Context, filters string, dest interface{}) error {
	key := fmt.Sprintf("hotels:list:%s", filters)
	return r.Get(ctx, key, dest)
}

// Room availability cache
func (r *RedisClient) CacheRoomAvailability(ctx context.Context, roomID string, fromDate, tillDate string, available bool, ttl time.Duration) error {
	key := fmt.Sprintf("room:availability:%s:%s:%s", roomID, fromDate, tillDate)
	return r.Set(ctx, key, map[string]bool{"available": available}, ttl)
}

func (r *RedisClient) GetCachedRoomAvailability(ctx context.Context, roomID string, fromDate, tillDate string) (bool, error) {
	key := fmt.Sprintf("room:availability:%s:%s:%s", roomID, fromDate, tillDate)
	var result map[string]bool
	err := r.Get(ctx, key, &result)
	if err != nil {
		return false, err
	}
	return result["available"], nil
}

// User session cache
func (r *RedisClient) CacheUserSession(ctx context.Context, token string, userID string, ttl time.Duration) error {
	key := fmt.Sprintf("session:%s", token)
	return r.Set(ctx, key, map[string]string{"user_id": userID}, ttl)
}

func (r *RedisClient) GetCachedUserSession(ctx context.Context, token string) (string, error) {
	key := fmt.Sprintf("session:%s", token)
	var result map[string]string
	err := r.Get(ctx, key, &result)
	if err != nil {
		return "", err
	}
	return result["user_id"], nil
}

func (r *RedisClient) InvalidateUserSession(ctx context.Context, token string) error {
	key := fmt.Sprintf("session:%s", token)
	return r.Delete(ctx, key)
}

// Health check
func (r *RedisClient) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}