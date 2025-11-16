package gateway

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/akaza21/hotel-reservation/internal/events"
	"github.com/akaza21/hotel-reservation/internal/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/proxy"
)

// API Gateway - Single entry point for all client requests
type APIGateway struct {
	services map[string]*ServiceConfig
	eventBus *events.EventBus
}

type ServiceConfig struct {
	Name        string
	BaseURL     string
	Prefix      string
	Timeout     time.Duration
	HealthCheck string
}

func NewAPIGateway(eventBus *events.EventBus) *APIGateway {
	return &APIGateway{
		services: make(map[string]*ServiceConfig),
		eventBus: eventBus,
	}
}

// Register microservices
func (gw *APIGateway) RegisterServices() {
	gw.services["user"] = &ServiceConfig{
		Name:        "user-service",
		BaseURL:     "http://user-service:8080",
		Prefix:      "/api/v1/user",
		Timeout:     30 * time.Second,
		HealthCheck: "/health",
	}

	gw.services["hotel"] = &ServiceConfig{
		Name:        "hotel-service",
		BaseURL:     "http://hotel-service:8080",
		Prefix:      "/api/v1/hotel",
		Timeout:     30 * time.Second,
		HealthCheck: "/health",
	}

	gw.services["booking"] = &ServiceConfig{
		Name:        "booking-service",
		BaseURL:     "http://booking-service:8080",
		Prefix:      "/api/v1/booking",
		Timeout:     30 * time.Second,
		HealthCheck: "/health",
	}

	gw.services["notification"] = &ServiceConfig{
		Name:        "notification-service",
		BaseURL:     "http://notification-service:8080",
		Prefix:      "/api/v1/notification",
		Timeout:     10 * time.Second,
		HealthCheck: "/health",
	}
}

func (gw *APIGateway) SetupRoutes(app *fiber.App) {
	// API Gateway middleware
	app.Use(gw.requestLoggingMiddleware())
	app.Use(gw.rateLimitingMiddleware())
	app.Use(gw.authenticationMiddleware())

	// Health check for gateway itself
	app.Get("/gateway/health", gw.healthCheck)

	// Route requests to appropriate services
	for _, service := range gw.services {
		gw.setupServiceProxy(app, service)
	}
}

func (gw *APIGateway) setupServiceProxy(app *fiber.App, service *ServiceConfig) {
	app.All(service.Prefix+"/*", func(c *fiber.Ctx) error {
		// Remove service prefix from URL
		targetPath := strings.TrimPrefix(c.Path(), service.Prefix)
		targetURL := service.BaseURL + targetPath

		// Add query parameters
		if c.Context().QueryArgs().Len() > 0 {
			targetURL += "?" + string(c.Context().QueryArgs().QueryString())
		}

		// Proxy the request
		return proxy.Do(c, targetURL)
	})
}

// Middleware functions
func (gw *APIGateway) requestLoggingMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Generate request ID
		requestID := fmt.Sprintf("gw-%d", time.Now().UnixNano())
		c.Set("X-Request-ID", requestID)

		err := c.Next()

		// Log request through gateway
		duration := time.Since(start)

		// Publish gateway metrics event
		gatewayEvent := events.Event{
			Type: events.MetricRecorded,
			Data: map[string]interface{}{
				"component":  "api-gateway",
				"method":     c.Method(),
				"path":       c.Path(),
				"status":     c.Response().StatusCode(),
				"duration":   duration.Milliseconds(),
				"request_id": requestID,
			},
			Source: "api-gateway",
		}

		go func() {
			if err := gw.eventBus.Publish(context.Background(), "gateway-metrics", gatewayEvent); err != nil {
				logger.WithFields(map[string]interface{}{
					"request_id": requestID,
					"error":      err.Error(),
				}).Error("Failed to publish gateway metrics")
			}
		}()

		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"method":     c.Method(),
			"path":       c.Path(),
			"status":     c.Response().StatusCode(),
			"duration":   duration,
		}).Info("Gateway request processed")

		return err
	}
}

func (gw *APIGateway) rateLimitingMiddleware() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        100,
		Expiration: 1 * time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			// Log rate limit exceeded
			logger.WithFields(map[string]interface{}{
				"client_ip": c.IP(),
				"path":      c.Path(),
			}).Warn("Rate limit exceeded")

			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":       "Rate limit exceeded",
				"retry_after": "60s",
			})
		},
	})
}

func (gw *APIGateway) authenticationMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip auth for health checks and public endpoints
		if strings.HasSuffix(c.Path(), "/health") ||
			strings.HasPrefix(c.Path(), "/api/v1/auth") ||
			strings.HasPrefix(c.Path(), "/api/v1/hotel") {
			return c.Next()
		}

		// Extract JWT token from Authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authorization header required",
			})
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Bearer token required",
			})
		}

		// Here you would validate the JWT token
		// For now, just pass it through to the service

		return c.Next()
	}
}

func (gw *APIGateway) healthCheck(c *fiber.Ctx) error {
	// Check health of all registered services
	serviceHealth := make(map[string]string)

	for name, service := range gw.services {
		health := gw.checkServiceHealth(service)
		serviceHealth[name] = health
	}

	return c.JSON(fiber.Map{
		"gateway":   "healthy",
		"timestamp": time.Now().UTC(),
		"services":  serviceHealth,
	})
}

func (gw *APIGateway) checkServiceHealth(service *ServiceConfig) string {
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(service.BaseURL + service.HealthCheck)
	if err != nil {
		return "unhealthy"
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return "healthy"
	}

	return "unhealthy"
}
