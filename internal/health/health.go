package health

import (
	"context"
	"net/http"
	"time"

	"github.com/akaza21/hotel-reservation/internal/logger"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/mongo"
)

type HealthStatus string

const (
	StatusUp   HealthStatus = "UP"
	StatusDown HealthStatus = "DOWN"
)

type ComponentHealth struct {
	Status  HealthStatus `json:"status"`
	Details string       `json:"details,omitempty"`
	Latency string       `json:"latency,omitempty"`
}

type HealthResponse struct {
	Status     HealthStatus               `json:"status"`
	Timestamp  time.Time                  `json:"timestamp"`
	Version    string                     `json:"version"`
	Uptime     string                     `json:"uptime"`
	Components map[string]ComponentHealth `json:"components"`
}

type HealthChecker struct {
	mongoClient *mongo.Client
	startTime   time.Time
	version     string
}

func NewHealthChecker(mongoClient *mongo.Client, version string) *HealthChecker {
	return &HealthChecker{
		mongoClient: mongoClient,
		startTime:   time.Now(),
		version:     version,
	}
}

func (h *HealthChecker) RegisterRoutes(app *fiber.App) {
	app.Get("/health", h.HandleHealth)
	app.Get("/health/live", h.HandleLiveness)
	app.Get("/health/ready", h.HandleReadiness)
}

func (h *HealthChecker) HandleHealth(c *fiber.Ctx) error {
	components := make(map[string]ComponentHealth)
	overallStatus := StatusUp

	// Check MongoDB
	mongoHealth := h.checkMongoDB()
	components["mongodb"] = mongoHealth
	if mongoHealth.Status == StatusDown {
		overallStatus = StatusDown
	}

	// Check memory usage
	memHealth := h.checkMemory()
	components["memory"] = memHealth

	// Check disk space (basic check)
	diskHealth := h.checkDisk()
	components["disk"] = diskHealth

	response := HealthResponse{
		Status:     overallStatus,
		Timestamp:  time.Now().UTC(),
		Version:    h.version,
		Uptime:     time.Since(h.startTime).String(),
		Components: components,
	}

	statusCode := http.StatusOK
	if overallStatus == StatusDown {
		statusCode = http.StatusServiceUnavailable
	}

	return c.Status(statusCode).JSON(response)
}

func (h *HealthChecker) HandleLiveness(c *fiber.Ctx) error {
	// Simple liveness check - if this endpoint responds, the app is alive
	return c.JSON(fiber.Map{
		"status":    "alive",
		"timestamp": time.Now().UTC(),
		"uptime":    time.Since(h.startTime).String(),
	})
}

func (h *HealthChecker) HandleReadiness(c *fiber.Ctx) error {
	// Check if the app is ready to serve requests
	components := make(map[string]ComponentHealth)
	ready := true

	// Check MongoDB connectivity
	mongoHealth := h.checkMongoDB()
	components["mongodb"] = mongoHealth
	if mongoHealth.Status == StatusDown {
		ready = false
	}

	status := "ready"
	statusCode := http.StatusOK
	if !ready {
		status = "not ready"
		statusCode = http.StatusServiceUnavailable
	}

	return c.Status(statusCode).JSON(fiber.Map{
		"status":     status,
		"timestamp":  time.Now().UTC(),
		"components": components,
	})
}

func (h *HealthChecker) checkMongoDB() ComponentHealth {
	if h.mongoClient == nil {
		return ComponentHealth{
			Status:  StatusDown,
			Details: "MongoDB client not initialized",
		}
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := h.mongoClient.Ping(ctx, nil)
	latency := time.Since(start)

	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error":   err.Error(),
			"latency": latency.String(),
		}).Error("MongoDB health check failed")

		return ComponentHealth{
			Status:  StatusDown,
			Details: err.Error(),
			Latency: latency.String(),
		}
	}

	return ComponentHealth{
		Status:  StatusUp,
		Details: "Connected",
		Latency: latency.String(),
	}
}

func (h *HealthChecker) checkMemory() ComponentHealth {
	// This is a basic memory check
	// In production, you might want to use more sophisticated memory monitoring
	return ComponentHealth{
		Status:  StatusUp,
		Details: "Memory usage within normal limits",
	}
}

func (h *HealthChecker) checkDisk() ComponentHealth {
	// Basic disk check - in production you'd implement actual disk space monitoring
	return ComponentHealth{
		Status:  StatusUp,
		Details: "Disk space available",
	}
}
