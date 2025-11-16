package api

import (
	"context"
	"time"

	"github.com/akaza21/hotel-reservation/db"
	"github.com/akaza21/hotel-reservation/internal/events"
	"github.com/akaza21/hotel-reservation/internal/logger"
	"github.com/akaza21/hotel-reservation/internal/metrics"
	"github.com/akaza21/hotel-reservation/types"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SimpleEventBookingHandler - simplified version that works with existing code
type SimpleEventBookingHandler struct {
	store    *db.Store
	eventBus *events.EventBus
}

func NewSimpleEventBookingHandler(store *db.Store, eventBus *events.EventBus) *SimpleEventBookingHandler {
	return &SimpleEventBookingHandler{
		store:    store,
		eventBus: eventBus,
	}
}

// HandleBookRoom - enhanced version that publishes events (simplified)
func (h *SimpleEventBookingHandler) HandleBookRoom(c *fiber.Ctx) error {
	var params BookRoomParams
	if err := c.BodyParser(&params); err != nil {
		return ErrBadRequest()
	}

	// Existing validation
	if err := params.validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	roomID, _ := primitive.ObjectIDFromHex(c.Params("id"))
	user, err := getAuthUser(c)
	if err != nil {
		return ErrUnAuthorized()
	}

	// Check availability (existing logic)
	ok, err := h.isRoomAvailableForBooking(roomID, params)
	if err != nil {
		return err
	}
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"msg": "room already booked",
		})
	}

	// Create booking (existing logic)
	booking := &types.Booking{
		UserID:     user.ID,
		RoomID:     roomID,
		FromDate:   params.FromDate,
		TillDate:   params.TillDate,
		NumPersons: params.NumPersons,
	}

	insertedBooking, err := h.store.Booking.InsertBooking(c.Context(), booking)
	if err != nil {
		return err
	}

	go h.publishBookingCreatedEvent(c.Context(), insertedBooking, user)

	logger.WithFields(map[string]interface{}{
		"booking_id": insertedBooking.ID.Hex(),
		"user_id":    user.ID.Hex(),
		"room_id":    roomID.Hex(),
	}).Info("Booking created with events")
	metrics.IncrementBookingCreated()

	return c.JSON(insertedBooking)
}

// HandleCancelBooking - enhanced version that publishes events
func (h *SimpleEventBookingHandler) HandleCancelBooking(c *fiber.Ctx) error {
	id := c.Params("id")
	booking, err := h.store.Booking.GetBookingByID(c.Context(), id)
	if err != nil {
		return ErrNotResourceNotFound("booking")
	}

	user, err := getAuthUser(c)
	if err != nil {
		return ErrUnAuthorized()
	}

	if booking.UserID != user.ID {
		return ErrUnAuthorized()
	}

	// Update booking (existing logic)
	if err := h.store.Booking.UpdateBooking(c.Context(), id, bson.M{"$set": bson.M{"canceled": true}}); err != nil {
		return err
	}

	go h.publishBookingCancelledEvent(c.Context(), booking, user)

	logger.WithFields(map[string]interface{}{
		"booking_id": booking.ID.Hex(),
		"user_id":    user.ID.Hex(),
	}).Info("Booking cancelled with events")
	metrics.IncrementBookingCancelled()

	return SuccessJSON(c, nil, "Booking canceled successfully")
}

// HandleGetBookings - with user behavior tracking
func (h *SimpleEventBookingHandler) HandleGetBookings(c *fiber.Ctx) error {
	user, err := getAuthUser(c)
	if err != nil {
		return ErrUnAuthorized()
	}

	go h.publishUserBehaviorEvent(c.Context(), user.ID.Hex(), "view_bookings")

	filter := bson.M{"userID": user.ID}
	bookings, err := h.store.Booking.GetBookings(c.Context(), filter)
	if err != nil {
		return ErrNotResourceNotFound("bookings")
	}

	return c.JSON(bookings)
}

func (h *SimpleEventBookingHandler) HandleGetBooking(c *fiber.Ctx) error {
	id := c.Params("id")
	booking, err := h.store.Booking.GetBookingByID(c.Context(), id)
	if err != nil {
		return ErrNotResourceNotFound("booking")
	}

	user, err := getAuthUser(c)
	if err != nil {
		return ErrUnAuthorized()
	}

	if booking.UserID != user.ID {
		return ErrUnAuthorized()
	}

	go h.publishUserBehaviorEvent(c.Context(), user.ID.Hex(), "view_booking_details")

	return c.JSON(booking)
}

// Event publishing methods (simplified)

func (h *SimpleEventBookingHandler) publishBookingCreatedEvent(ctx context.Context, booking *types.Booking, user *types.User) {
	// Calculate a basic booking amount (simplified)
	nights := booking.TillDate.Sub(booking.FromDate).Hours() / 24
	estimatedAmount := nights * 100.0 // $100 per night estimate

	// Create booking created event
	bookingEvent := events.Event{
		Type:   events.BookingCreated,
		UserID: user.ID.Hex(),
		Data: events.BookingCreatedData{
			BookingID: booking.ID.Hex(),
			HotelID:   "hotel-" + booking.RoomID.Hex()[:6], // Simplified hotel ID
			RoomID:    booking.RoomID.Hex(),
			CheckIn:   booking.FromDate,
			CheckOut:  booking.TillDate,
			Amount:    estimatedAmount,
		},
		Source: "booking-service",
	}

	// Publish to multiple topics
	topics := []string{"bookings", "business-metrics", "audit-logs"}

	for _, topic := range topics {
		if err := h.eventBus.Publish(ctx, topic, bookingEvent); err != nil {
			logger.WithFields(map[string]interface{}{
				"booking_id": booking.ID.Hex(),
				"topic":      topic,
				"error":      err.Error(),
			}).Error("Failed to publish booking event")
		}
	}

	logger.WithFields(map[string]interface{}{
		"booking_id": booking.ID.Hex(),
		"amount":     estimatedAmount,
		"topics":     len(topics),
	}).Info("Booking created events published")
}

func (h *SimpleEventBookingHandler) publishBookingCancelledEvent(ctx context.Context, booking *types.Booking, user *types.User) {
	cancellationEvent := events.Event{
		Type:   events.BookingCancelled,
		UserID: user.ID.Hex(),
		Data: map[string]interface{}{
			"booking_id":          booking.ID.Hex(),
			"room_id":             booking.RoomID.Hex(),
			"cancelled_at":        time.Now(),
			"original_check_in":   booking.FromDate,
			"original_check_out":  booking.TillDate,
			"cancellation_reason": "user_requested",
		},
		Source: "booking-service",
	}

	// Publish to relevant topics
	topics := []string{"bookings", "business-metrics", "audit-logs"}

	for _, topic := range topics {
		if err := h.eventBus.Publish(ctx, topic, cancellationEvent); err != nil {
			logger.WithFields(map[string]interface{}{
				"booking_id": booking.ID.Hex(),
				"topic":      topic,
				"error":      err.Error(),
			}).Error("Failed to publish cancellation event")
		}
	}

	logger.WithFields(map[string]interface{}{
		"booking_id": booking.ID.Hex(),
		"topics":     len(topics),
	}).Info("Booking cancellation events published")
}

func (h *SimpleEventBookingHandler) publishUserBehaviorEvent(ctx context.Context, userID, action string) {
	behaviorEvent := events.Event{
		Type:   events.MetricRecorded,
		UserID: userID,
		Data: map[string]interface{}{
			"action":    action,
			"timestamp": time.Now(),
			"source":    "booking-handler",
		},
		Source: "booking-service",
	}

	if err := h.eventBus.Publish(ctx, "user-behavior", behaviorEvent); err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"action":  action,
			"error":   err.Error(),
		}).Error("Failed to publish behavior event")
	}
}

// Helper method (existing logic)
func (h *SimpleEventBookingHandler) isRoomAvailableForBooking(roomID primitive.ObjectID, params BookRoomParams) (bool, error) {
	filter := bson.M{
		"roomID": roomID,
		"fromDate": bson.M{
			"$lt": params.TillDate,
		},
		"tillDate": bson.M{
			"$gt": params.FromDate,
		},
		"canceled": bson.M{"$ne": true},
	}

	bookings, err := h.store.Booking.GetBookings(context.Background(), filter)
	if err != nil {
		return false, err
	}

	return len(bookings) == 0, nil
}
