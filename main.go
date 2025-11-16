package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/akaza21/hotel-reservation/api"
	"github.com/akaza21/hotel-reservation/db"
	"github.com/akaza21/hotel-reservation/internal/config"
	"github.com/akaza21/hotel-reservation/internal/events"
	"github.com/akaza21/hotel-reservation/internal/logger"
	"github.com/akaza21/hotel-reservation/internal/metrics"
	"github.com/akaza21/hotel-reservation/internal/middleware"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration:", err)
	}

	// Initialize logger
	logger.InitLogger()
	logger.Info("Starting Hotel Reservation API Server")
	logger.WithFields(map[string]interface{}{
		"environment": cfg.Server.Environment,
		"port":        cfg.Server.Port,
		"log_level":   cfg.Logger.Level,
	}).Info("Server configuration loaded")

	// Parse command line flags
	listenAddr := flag.String("listenAddr", ":"+cfg.Server.Port, "The listen address of the API Server")
	flag.Parse()

	// Connect to MongoDB with timeout and better error handling
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Database.Timeout)
	defer cancel()

	clientOptions := options.Client().
		ApplyURI(cfg.Database.URI).
		SetMaxPoolSize(uint64(cfg.Database.MaxConns)).
		SetConnectTimeout(cfg.Database.Timeout)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"uri":   cfg.Database.URI,
			"error": err.Error(),
		}).Fatal("Failed to connect to MongoDB")
	}
	defer client.Disconnect(context.Background())

	// Ping the database to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Fatal("Failed to ping MongoDB")
	}
	logger.Info("Successfully connected to MongoDB")

	// Initialize stores
	userStore := db.NewMongoUserStore(client)
	hotelStore := db.NewMongoHotelStore(client)
	roomStore := db.NewMongoRoomStore(client, hotelStore)
	bookingStore := db.NewMongoBookingStore(client)

	store := &db.Store{
		User:    userStore,
		Hotel:   hotelStore,
		Room:    roomStore,
		Booking: bookingStore,
	}

	logger.Info("Database stores initialized successfully")

	redisURL := cfg.Redis.URL
	redisAddr := "localhost:6379"
	if strings.Contains(redisURL, "://") {
		// Extract address from redis://host:port format
		parts := strings.Split(redisURL, "://")
		if len(parts) > 1 {
			redisAddr = parts[1]
		}
	} else {
		redisAddr = redisURL
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test Redis connection
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to connect to Redis - events will be disabled")
		redisClient = nil // Disable events if Redis is not available
	} else {
		logger.Info("Connected to Redis for event bus")
	}

	// Create event bus
	var eventBus *events.EventBus
	if redisClient != nil {
		eventBus = events.NewEventBus(redisClient)
		logger.Info("Event bus initialized")
	}

	// Initialize handlers with config and events
	userHandler := api.NewUserHandler(userStore)
	hotelHandler := api.NewHotelHandler(store)
	authHandler := api.NewAuthHandler(userStore, cfg)
	roomHandler := api.NewRoomHandler(store)

	// Initialize handlers - use event handlers if available
	var bookingHandler *api.BookingHandler
	var simpleEventBookingHandler *api.SimpleEventBookingHandler

	if eventBus != nil {
		simpleEventBookingHandler = api.NewSimpleEventBookingHandler(store, eventBus)
		logger.Info("Event-driven booking handler enabled")
	} else {
		bookingHandler = api.NewBookingHandler(store)
		logger.Info("Using standard booking handler (events disabled)")
	}

	// Create Fiber app with enhanced configuration
	fiberConfig := fiber.Config{
		ErrorHandler:         api.ErrorHandler,
		ReadTimeout:          cfg.Server.ReadTimeout,
		WriteTimeout:         cfg.Server.WriteTimeout,
		ServerHeader:         "Hotel-Reservation-API",
		DisableKeepalive:     cfg.Server.Environment == "production",
		CaseSensitive:        true,
		StrictRouting:        true,
		UnescapePath:         false,
		BodyLimit:            4 * 1024 * 1024, // 4MB
		CompressedFileSuffix: ".gz",
	}

	app := fiber.New(fiberConfig)

	// Apply global middleware
	app.Use(middleware.Recovery())
	app.Use(middleware.RequestLogger())
	app.Use(middleware.SecurityHeaders())
	app.Use(middleware.CORS())
	app.Use(middleware.RateLimiter(cfg.RateLimit, redisClient))
	app.Use(metrics.PrometheusMiddleware())
	app.Use(middleware.Timeout(30 * time.Second))

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "healthy",
			"timestamp": time.Now().UTC(),
			"version":   "1.0.0",
		})
	})
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	// API v1 group
	apiv1 := app.Group("/api/v1")

	// Auth routes (public)
	auth := apiv1.Group("/auth")
	auth.Post("/", authHandler.HandleAuthenticate)

	// User routes
	users := apiv1.Group("/user")
	users.Get("/", userHandler.HandleGetUsers)
	users.Get("/:id", userHandler.HandleGetUser)
	users.Post("/", userHandler.HandlePostUser)
	users.Delete("/:id", userHandler.HandleDeleteUser)
	users.Put("/:id", userHandler.HandlePutUser)

	// Hotel routes (public read access)
	hotels := apiv1.Group("/hotel")
	hotels.Get("/", hotelHandler.HandleGetHotels)
	hotels.Get("/:id", hotelHandler.HandleGetHotel)
	hotels.Get("/:id/rooms", hotelHandler.HandleGetRooms)

	// Room routes (protected)
	rooms := apiv1.Group("/room")
	if simpleEventBookingHandler != nil {
		// Use event-driven booking
		rooms.Post("/:id/book", api.JWTAuthentication(userStore, cfg), simpleEventBookingHandler.HandleBookRoom)
	} else {
		// Use standard booking
		rooms.Post("/:id/book", api.JWTAuthentication(userStore, cfg), roomHandler.HandleBookRoom)
	}

	// Booking routes (protected)
	bookings := apiv1.Group("/booking", api.JWTAuthentication(userStore, cfg))
	if simpleEventBookingHandler != nil {
		// Use event-driven handlers
		bookings.Get("/", simpleEventBookingHandler.HandleGetBookings)
		bookings.Get("/:id", simpleEventBookingHandler.HandleGetBooking)
		bookings.Post("/:id/cancel", simpleEventBookingHandler.HandleCancelBooking)
	} else {
		// Use standard handlers
		bookings.Get("/", bookingHandler.HandleGetBookings)
		bookings.Get("/:id", bookingHandler.HandleGetBooking)
		bookings.Post("/:id/cancel", bookingHandler.HandleCancelBooking)
	}

	// Admin routes (protected + admin only)
	admin := apiv1.Group("/admin", api.JWTAuthentication(userStore, cfg), api.AdminAuth)
	if simpleEventBookingHandler != nil {
		admin.Get("/booking", simpleEventBookingHandler.HandleGetBookings)
	} else {
		admin.Get("/booking", bookingHandler.HandleGetBookings)
	}

	// Log all registered routes
	logger.Info("Registered routes:")
	for _, route := range app.GetRoutes() {
		logger.WithFields(map[string]interface{}{
			"method": route.Method,
			"path":   route.Path,
		}).Debug("Route registered")
	}

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		logger.Info("Gracefully shutting down server...")
		app.Shutdown()
	}()

	// Start server
	logger.WithFields(map[string]interface{}{
		"address": *listenAddr,
	}).Info("Server starting...")

	if err := app.Listen(*listenAddr); err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Fatal("Failed to start server")
	}

	logger.Info("Server stopped")
}
