package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/akaza21/hotel-reservation/internal/config"
	"github.com/akaza21/hotel-reservation/internal/events"
	"github.com/akaza21/hotel-reservation/internal/logger"
	"github.com/gofiber/fiber/v2"
)

type EnhancedNotificationService struct {
	eventBus *events.EventBus
	config   *config.Config
	app      *fiber.App
}

func NewEnhancedNotificationService(eventBus *events.EventBus, cfg *config.Config) *EnhancedNotificationService {
	service := &EnhancedNotificationService{
		eventBus: eventBus,
		config:   cfg,
		app:      fiber.New(fiber.Config{ServerHeader: "Notification-Service"}),
	}

	service.setupRoutes()
	return service
}

func (ns *EnhancedNotificationService) setupRoutes() {
	// Health check endpoint
	ns.app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "healthy",
			"service":   "notification-service",
			"timestamp": time.Now().UTC(),
			"version":   "1.0.0",
		})
	})

	// Metrics endpoint
	ns.app.Get("/metrics", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"service":            "notification-service",
			"emails_sent":        0, // In real implementation, track these
			"sms_sent":           0,
			"push_notifications": 0,
			"failed_deliveries":  0,
		})
	})
}

// Start both event consumers and HTTP server
func (ns *EnhancedNotificationService) Start(ctx context.Context) error {
	logger.Info("Starting Enhanced Notification Service")

	// Start event consumers
	go ns.startEventConsumers(ctx)

	// Start HTTP server for health checks
	go func() {
		logger.WithFields(map[string]interface{}{
			"port": "8080",
		}).Info("Starting notification service HTTP server")

		if err := ns.app.Listen(":8080"); err != nil {
			logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Error("Failed to start HTTP server")
		}
	}()

	return nil
}

func (ns *EnhancedNotificationService) startEventConsumers(ctx context.Context) {
	// Subscribe to booking events
	go ns.subscribeToBookingEvents(ctx)

	// Subscribe to user events
	go ns.subscribeToUserEvents(ctx)

	// Subscribe to general notification requests
	go ns.subscribeToNotificationEvents(ctx)
}

func (ns *EnhancedNotificationService) subscribeToBookingEvents(ctx context.Context) {
	logger.Info("Subscribing to booking events")

	err := ns.eventBus.Subscribe(ctx, "bookings", "notification-service-bookings", func(event events.Event) error {
		logger.WithFields(map[string]interface{}{
			"event_type": event.Type,
			"event_id":   event.ID,
			"user_id":    event.UserID,
		}).Info("Processing booking event")

		switch event.Type {
		case events.BookingCreated:
			return ns.handleBookingCreated(event)
		case events.BookingCancelled:
			return ns.handleBookingCancelled(event)
		case events.BookingConfirmed:
			return ns.handleBookingConfirmed(event)
		default:
			logger.WithFields(map[string]interface{}{
				"event_type": event.Type,
			}).Warn("Unknown booking event type")
		}
		return nil
	})

	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to subscribe to booking events")
	}
}

func (ns *EnhancedNotificationService) subscribeToUserEvents(ctx context.Context) {
	logger.Info("Subscribing to user events")

	err := ns.eventBus.Subscribe(ctx, "users", "notification-service-users", func(event events.Event) error {
		logger.WithFields(map[string]interface{}{
			"event_type": event.Type,
			"event_id":   event.ID,
			"user_id":    event.UserID,
		}).Info("Processing user event")

		switch event.Type {
		case events.UserRegistered:
			return ns.handleUserRegistered(event)
		case events.UserLoggedIn:
			return ns.handleUserLoggedIn(event)
		default:
			logger.WithFields(map[string]interface{}{
				"event_type": event.Type,
			}).Warn("Unknown user event type")
		}
		return nil
	})

	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to subscribe to user events")
	}
}

func (ns *EnhancedNotificationService) subscribeToNotificationEvents(ctx context.Context) {
	logger.Info("Subscribing to notification events")

	err := ns.eventBus.Subscribe(ctx, "notifications", "notification-service-direct", func(event events.Event) error {
		logger.WithFields(map[string]interface{}{
			"event_type": event.Type,
			"event_id":   event.ID,
		}).Info("Processing direct notification event")

		// Handle direct notification requests
		return ns.handleDirectNotification(event)
	})

	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to subscribe to notification events")
	}
}

// Event Handlers

func (ns *EnhancedNotificationService) handleBookingCreated(event events.Event) error {
	var data events.BookingCreatedData
	if err := ns.parseEventData(event.Data, &data); err != nil {
		return err
	}

	logger.WithFields(map[string]interface{}{
		"booking_id": data.BookingID,
		"user_id":    event.UserID,
		"amount":     data.Amount,
	}).Info("🎉 Handling booking created event")

	// Send multiple notifications
	notifications := []func() error{
		func() error { return ns.sendBookingConfirmationEmail(event.UserID, data) },
		func() error { return ns.sendBookingConfirmationSMS(event.UserID, data) },
		func() error { return ns.sendBookingPushNotification(event.UserID, data) },
	}

	for _, notify := range notifications {
		if err := notify(); err != nil {
			logger.WithFields(map[string]interface{}{
				"booking_id": data.BookingID,
				"error":      err.Error(),
			}).Error("Failed to send booking notification")
		}
	}

	return nil
}

func (ns *EnhancedNotificationService) handleBookingCancelled(event events.Event) error {
	logger.WithFields(map[string]interface{}{
		"user_id":  event.UserID,
		"event_id": event.ID,
	}).Info("Handling booking cancellation event")

	// Extract booking ID from event data
	eventData := event.Data.(map[string]interface{})
	bookingID := eventData["booking_id"].(string)

	notifications := []func() error{
		func() error { return ns.sendCancellationEmail(event.UserID, bookingID) },
		func() error { return ns.sendCancellationSMS(event.UserID, bookingID) },
	}

	for _, notify := range notifications {
		if err := notify(); err != nil {
			logger.WithFields(map[string]interface{}{
				"booking_id": bookingID,
				"error":      err.Error(),
			}).Error("Failed to send cancellation notification")
		}
	}

	return nil
}

func (ns *EnhancedNotificationService) handleBookingConfirmed(event events.Event) error {
	logger.WithFields(map[string]interface{}{
		"user_id":  event.UserID,
		"event_id": event.ID,
	}).Info("Handling booking confirmation event")

	return ns.sendBookingReminderEmail(event.UserID)
}

func (ns *EnhancedNotificationService) handleUserRegistered(event events.Event) error {
	var data events.UserRegisteredData
	if err := ns.parseEventData(event.Data, &data); err != nil {
		return err
	}

	logger.WithFields(map[string]interface{}{
		"user_id": data.UserID,
		"email":   data.Email,
	}).Info("Handling user registration event")

	notifications := []func() error{
		func() error { return ns.sendWelcomeEmail(data) },
		func() error { return ns.sendWelcomePushNotification(data.UserID) },
	}

	for _, notify := range notifications {
		if err := notify(); err != nil {
			logger.WithFields(map[string]interface{}{
				"user_id": data.UserID,
				"error":   err.Error(),
			}).Error("Failed to send welcome notification")
		}
	}

	return nil
}

func (ns *EnhancedNotificationService) handleUserLoggedIn(event events.Event) error {
	eventData := event.Data.(map[string]interface{})
	clientIP := eventData["client_ip"].(string)

	// Only send security notification for suspicious logins
	if ns.isSuspiciousLogin(clientIP) {
		logger.WithFields(map[string]interface{}{
			"user_id":   event.UserID,
			"client_ip": clientIP,
		}).Info("🔐 Sending security notification for suspicious login")

		return ns.sendSecurityNotification(event.UserID, clientIP)
	}

	return nil
}

func (ns *EnhancedNotificationService) handleDirectNotification(event events.Event) error {
	// Handle direct notification requests from other services
	logger.WithFields(map[string]interface{}{
		"event_id": event.ID,
	}).Info("📮 Handling direct notification request")

	return nil
}

// Notification Methods

func (ns *EnhancedNotificationService) sendBookingConfirmationEmail(userID string, data events.BookingCreatedData) error {
	subject := fmt.Sprintf("Booking Confirmed - ID: %s", data.BookingID)
	body := fmt.Sprintf(`
Dear Guest,

Your hotel reservation has been confirmed!

Booking Details:
- Booking ID: %s
- Hotel ID: %s
- Room ID: %s
- Check-in: %s
- Check-out: %s
- Total Amount: $%.2f

Thank you for choosing our hotel!

Best regards,
Hotel Reservation Team
	`, data.BookingID, data.HotelID, data.RoomID,
		data.CheckIn.Format("Jan 2, 2006"),
		data.CheckOut.Format("Jan 2, 2006"),
		data.Amount)

	return ns.sendEmail(userID, subject, body)
}

func (ns *EnhancedNotificationService) sendBookingConfirmationSMS(userID string, data events.BookingCreatedData) error {
	message := fmt.Sprintf("Hotel booking confirmed! ID: %s, Check-in: %s, Amount: $%.2f",
		data.BookingID, data.CheckIn.Format("Jan 2"), data.Amount)

	return ns.sendSMS(userID, message)
}

func (ns *EnhancedNotificationService) sendBookingPushNotification(userID string, data events.BookingCreatedData) error {
	message := fmt.Sprintf("Booking confirmed for $%.2f! Check-in: %s", data.Amount, data.CheckIn.Format("Jan 2"))
	return ns.sendPushNotification(userID, "Booking Confirmed", message)
}

func (ns *EnhancedNotificationService) sendCancellationEmail(userID, bookingID string) error {
	subject := fmt.Sprintf("Booking Cancelled - ID: %s", bookingID)
	body := fmt.Sprintf(`
Dear Guest,

Your hotel reservation has been cancelled.

Booking ID: %s

If you have any questions, please contact our support team.

Best regards,
Hotel Reservation Team
	`, bookingID)

	return ns.sendEmail(userID, subject, body)
}

func (ns *EnhancedNotificationService) sendCancellationSMS(userID, bookingID string) error {
	message := fmt.Sprintf("Hotel booking cancelled: %s. Refund processed.", bookingID)
	return ns.sendSMS(userID, message)
}

func (ns *EnhancedNotificationService) sendBookingReminderEmail(userID string) error {
	subject := "Upcoming Hotel Stay Reminder"
	body := "Don't forget about your upcoming hotel stay! Have a great trip!"
	return ns.sendEmail(userID, subject, body)
}

func (ns *EnhancedNotificationService) sendWelcomeEmail(data events.UserRegisteredData) error {
	subject := "Welcome to Hotel Reservation System!"
	body := fmt.Sprintf(`
Dear %s,

Welcome to our Hotel Reservation System!

We're excited to have you on board. You can now:
- Browse hotels worldwide
- Make instant reservations
- Manage your bookings
- Earn rewards points

Start exploring amazing hotels at great prices!

Best regards,
Hotel Reservation Team
	`, data.FirstName)

	return ns.sendEmailToAddress(data.Email, subject, body)
}

func (ns *EnhancedNotificationService) sendWelcomePushNotification(userID string) error {
	return ns.sendPushNotification(userID, "Welcome!", "Welcome to Hotel Reservation System! Start exploring amazing hotels.")
}

func (ns *EnhancedNotificationService) sendSecurityNotification(userID, clientIP string) error {
	subject := "Security Alert - New Login"
	body := fmt.Sprintf(`
Security Alert,

A new login was detected from IP address: %s

If this wasn't you, please change your password immediately.

Best regards,
Security Team
	`, clientIP)

	return ns.sendEmail(userID, subject, body)
}

// Core notification delivery methods

func (ns *EnhancedNotificationService) sendEmail(userID, subject, body string) error {
	// In a real implementation, you would:
	// 1. Get user email from user service
	// 2. Use actual SMTP configuration
	// 3. Handle delivery failures and retries

	logger.WithFields(map[string]interface{}{
		"user_id": userID,
		"subject": subject,
		"type":    "email",
	}).Info("📧 Email sent")

	// Mock implementation - in production, use real SMTP
	return ns.sendEmailToAddress("user@example.com", subject, body)
}

func (ns *EnhancedNotificationService) sendEmailToAddress(email, subject, body string) error {
	// Mock SMTP implementation
	logger.WithFields(map[string]interface{}{
		"email":   email,
		"subject": subject,
		"type":    "email",
	}).Info("📧 Email sent to address")

	// In production, use real SMTP:
	/*
		auth := smtp.PlainAuth("", ns.config.SMTP.Username, ns.config.SMTP.Password, ns.config.SMTP.Host)
		msg := []byte("To: " + email + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"\r\n" + body + "\r\n")

		err := smtp.SendMail(ns.config.SMTP.Host+":"+ns.config.SMTP.Port, auth, ns.config.SMTP.Username, []string{email}, msg)
		return err
	*/

	return nil
}

func (ns *EnhancedNotificationService) sendSMS(userID, message string) error {
	// In a real implementation, integrate with Twilio, AWS SNS, etc.
	logger.WithFields(map[string]interface{}{
		"user_id": userID,
		"message": message,
		"type":    "sms",
	}).Info("📱 SMS sent")

	return nil
}

func (ns *EnhancedNotificationService) sendPushNotification(userID, title, message string) error {
	// In a real implementation, integrate with Firebase Cloud Messaging, etc.
	logger.WithFields(map[string]interface{}{
		"user_id": userID,
		"title":   title,
		"message": message,
		"type":    "push",
	}).Info("Push notification sent")

	return nil
}

// Helper methods

func (ns *EnhancedNotificationService) parseEventData(data interface{}, target interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, target)
}

func (ns *EnhancedNotificationService) isSuspiciousLogin(clientIP string) bool {
	// Mock implementation - in production, check against threat intelligence
	suspiciousIPs := []string{"192.168.1.1", "10.0.0.1"} // Example
	for _, ip := range suspiciousIPs {
		if ip == clientIP {
			return true
		}
	}
	return false
}

func (ns *EnhancedNotificationService) Stop() error {
	logger.Info("Stopping notification service HTTP server")
	return ns.app.Shutdown()
}
