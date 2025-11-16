package metrics

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP metrics
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "The total number of processed HTTP requests",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "The duration of HTTP requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint", "status_code"},
	)

	// Business metrics
	bookingsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hotel_bookings_total",
			Help: "The total number of hotel bookings",
		},
		[]string{"status"}, // created, cancelled
	)

	activeBookings = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hotel_active_bookings",
			Help: "The current number of active bookings",
		},
	)

	userRegistrations = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "hotel_user_registrations_total",
			Help: "The total number of user registrations",
		},
	)

	authAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hotel_auth_attempts_total",
			Help: "The total number of authentication attempts",
		},
		[]string{"result"}, // success, failure
	)

	// Database metrics
	dbOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "database_operations_total",
			Help: "The total number of database operations",
		},
		[]string{"operation", "collection", "status"},
	)

	dbOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "database_operation_duration_seconds",
			Help:    "The duration of database operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "collection"},
	)
)

// Middleware for collecting HTTP metrics
func PrometheusMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		
		// Continue to next handler
		err := c.Next()
		
		// Collect metrics after request completion
		duration := time.Since(start).Seconds()
		statusCode := strconv.Itoa(c.Response().StatusCode())
		method := c.Method()
		endpoint := c.Route().Path
		
		if endpoint == "" {
			endpoint = c.Path()
		}

		// Increment request counter
		httpRequestsTotal.WithLabelValues(method, endpoint, statusCode).Inc()
		
		// Record request duration
		httpRequestDuration.WithLabelValues(method, endpoint, statusCode).Observe(duration)
		
		return err
	}
}

// Business metrics functions
func IncrementBookingCreated() {
	bookingsTotal.WithLabelValues("created").Inc()
}

func IncrementBookingCancelled() {
	bookingsTotal.WithLabelValues("cancelled").Inc()
}

func SetActiveBookings(count float64) {
	activeBookings.Set(count)
}

func IncrementUserRegistration() {
	userRegistrations.Inc()
}

func IncrementAuthSuccess() {
	authAttempts.WithLabelValues("success").Inc()
}

func IncrementAuthFailure() {
	authAttempts.WithLabelValues("failure").Inc()
}

// Database metrics functions
func IncrementDBOperation(operation, collection, status string) {
	dbOperations.WithLabelValues(operation, collection, status).Inc()
}

func RecordDBOperationDuration(operation, collection string, duration time.Duration) {
	dbOperationDuration.WithLabelValues(operation, collection).Observe(duration.Seconds())
}

// Helper function to time database operations
func TimeDBOperation(operation, collection string, fn func() error) error {
	start := time.Now()
	err := fn()
	duration := time.Since(start)
	
	status := "success"
	if err != nil {
		status = "error"
	}
	
	IncrementDBOperation(operation, collection, status)
	RecordDBOperationDuration(operation, collection, duration)
	
	return err
}