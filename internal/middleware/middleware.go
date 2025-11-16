package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/akaza21/hotel-reservation/internal/config"
	"github.com/akaza21/hotel-reservation/internal/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func RequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		requestID := uuid.New().String()
		c.Locals("request_id", requestID)

		ctx := context.WithValue(c.Context(), logger.RequestIDKey, requestID)
		c.SetUserContext(ctx)

		logger.LogRequest(c)

		err := c.Next()

		logger.LogResponse(c, err)

		return err
	}
}

func Recovery() fiber.Handler {
	return func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				logger.WithFields(map[string]interface{}{
					"panic":      r,
					"request_id": c.Locals("request_id"),
					"method":     c.Method(),
					"path":       c.Path(),
				}).Error("Panic recovered")

				c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error":      "Internal server error",
					"request_id": c.Locals("request_id"),
				})
			}
		}()

		return c.Next()
	}
}

func CORS() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With")

		if c.Method() == "OPTIONS" {
			return c.SendStatus(fiber.StatusNoContent)
		}

		return c.Next()
	}
}

func RateLimiter(cfg config.RateLimitConfig, redisClient *redis.Client) fiber.Handler {
	if !cfg.Enabled || redisClient == nil {
		logger.Warn("Rate limiting disabled (missing Redis or configuration)")
		return func(c *fiber.Ctx) error {
			return c.Next()
		}
	}

	logger.WithFields(map[string]interface{}{
		"default_limit":  cfg.Requests,
		"default_window": cfg.Window,
		"critical_paths": cfg.CriticalPaths,
	}).Info("Rate limiting middleware enabled")

	return func(c *fiber.Ctx) error {
		key := fmt.Sprintf("rate:%s", c.IP())
		limit := cfg.Requests
		window := cfg.Window

		if isCriticalPath(c.Path(), cfg.CriticalPaths) {
			limit = cfg.CriticalRequests
			window = cfg.CriticalWindow
			key = fmt.Sprintf("rate:critical:%s:%s", c.IP(), c.Route().Path)
		}

		allowed, remaining, retryAfter, err := allowRequest(c.Context(), redisClient, key, limit, window)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Error("Rate limiter fallback to allow due to Redis error")
			return c.Next()
		}

		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Set("X-RateLimit-Reset", fmt.Sprintf("%d", int(retryAfter)))

		if !allowed {
			logger.WithFields(map[string]interface{}{
				"ip":     c.IP(),
				"path":   c.Path(),
				"limit":  limit,
				"window": window,
			}).Warn("Rate limit exceeded")

			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many requests. Please try again later.",
			})
		}

		return c.Next()
	}
}

func allowRequest(ctx context.Context, client *redis.Client, key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	pipe := client.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return true, limit, 0, err
	}

	count := int(incr.Val())
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}

	ttl, err := client.TTL(ctx, key).Result()
	if err != nil || ttl < 0 {
		ttl = window
	}

	return count <= limit, remaining, ttl, nil
}

func isCriticalPath(path string, critical []string) bool {
	for _, p := range critical {
		if p == "" {
			continue
		}
		if strings.HasSuffix(p, "*") {
			prefix := strings.TrimSuffix(p, "*")
			if strings.HasPrefix(path, prefix) {
				return true
			}
			continue
		}
		if p == path {
			return true
		}
	}
	return false
}

func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Set("Content-Security-Policy", "default-src 'self'")

		return c.Next()
	}
}

func Timeout(duration time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), duration)
		defer cancel()

		c.SetUserContext(ctx)

		return c.Next()
	}
}
