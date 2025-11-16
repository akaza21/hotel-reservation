package api

import (
	"context"
	"fmt"
	"time"

	"github.com/akaza21/hotel-reservation/db"
	"github.com/akaza21/hotel-reservation/internal/logger"
	"github.com/akaza21/hotel-reservation/internal/metrics"
	"github.com/akaza21/hotel-reservation/internal/validator"
	"github.com/akaza21/hotel-reservation/types"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RoomHandler struct {
	store *db.Store
}

func NewRoomHandler(store *db.Store) *RoomHandler {
	return &RoomHandler{
		store: store,
	}
}

type BookRoomParams struct {
	FromDate   time.Time `json:"fromDate" validate:"required"`
	TillDate   time.Time `json:"tillDate" validate:"required"`
	NumPersons int       `json:"numPersons" validate:"required,min=1,max=10"`
}

func (p BookRoomParams) validate() error {
	now := time.Now()
	if now.After(p.FromDate) || now.After(p.TillDate) {
		return fmt.Errorf("cannot book a room in the past")
	}
	if p.FromDate.After(p.TillDate) {
		return fmt.Errorf("check-in date must be before check-out date")
	}
	return nil
}

func (h *RoomHandler) HandleBookRoom(c *fiber.Ctx) error {
	requestID := c.Locals("request_id").(string)
	roomIDParam := c.Params("id")

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"room_id":    roomIDParam,
	}).Info("Room booking attempt started")

	var params BookRoomParams
	if err := c.BodyParser(&params); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Failed to parse booking request body")
		return ErrBadRequest()
	}

	if err := validator.ValidateStruct(params); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"params":     params,
		}).Warn("Booking request validation failed")
		return NewValidationError(err.Error(), "")
	}

	if err := params.validate(); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"params":     params,
		}).Warn("Booking date validation failed")
		return NewValidationError("Invalid booking dates", err.Error())
	}

	if err := validator.ValidateObjectID(roomIDParam); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"room_id":    roomIDParam,
		}).Warn("Invalid room ID provided")
		return ErrInvalidID()
	}

	roomID, err := primitive.ObjectIDFromHex(roomIDParam)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"room_id":    roomIDParam,
			"error":      err.Error(),
		}).Error("Failed to parse room ID")
		return ErrInvalidID()
	}

	user, ok := c.Context().UserValue("user").(*types.User)
	if !ok {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
		}).Error("Failed to get user from request context")
		return NewInternalError("Failed to get user from context", "User not found in request context")
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    user.ID,
		"room_id":    roomIDParam,
		"from_date":  params.FromDate,
		"till_date":  params.TillDate,
		"persons":    params.NumPersons,
	}).Debug("Checking room availability")

	available, err := h.isRoomAvailable(c.Context(), roomID, params)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"room_id":    roomIDParam,
			"error":      err.Error(),
		}).Error("Failed to check room availability")
		return NewDatabaseError("room availability check", err)
	}

	if !available {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"room_id":    roomIDParam,
			"from_date":  params.FromDate,
			"till_date":  params.TillDate,
		}).Warn("Room not available for requested dates")
		return NewConflictError("Room unavailable", fmt.Sprintf("Room %s is already booked for the selected dates", roomIDParam))
	}

	booking := types.Booking{
		UserID:     user.ID,
		RoomID:     roomID,
		FromDate:   params.FromDate,
		TillDate:   params.TillDate,
		NumPersons: params.NumPersons,
	}

	inserted, err := h.store.Booking.InsertBooking(c.Context(), &booking)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    user.ID,
			"room_id":    roomIDParam,
			"error":      err.Error(),
		}).Error("Failed to create booking")
		return NewDatabaseError("booking creation", err)
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"booking_id": inserted.ID,
		"user_id":    user.ID,
		"room_id":    roomIDParam,
	}).Info("Room booking created successfully")
	metrics.IncrementBookingCreated()

	return SuccessJSON(c, inserted, "Room booked successfully")
}

func (h *RoomHandler) isRoomAvailable(ctx context.Context, roomID primitive.ObjectID, params BookRoomParams) (bool, error) {
	where := bson.M{
		"roomID": roomID,
		"fromDate": bson.M{
			"$lt": params.TillDate,
		},
		"tillDate": bson.M{
			"$gt": params.FromDate,
		},
		"canceled": bson.M{"$ne": true},
	}
	bookings, err := h.store.Booking.GetBookings(ctx, where)
	if err != nil {
		return false, err
	}
	ok := len(bookings) == 0
	return ok, nil
}
