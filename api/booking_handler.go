package api

import (
	"errors"

	"github.com/akaza21/hotel-reservation/db"
	"github.com/akaza21/hotel-reservation/internal/metrics"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type BookingHandler struct {
	store *db.Store
}

func NewBookingHandler(store *db.Store) *BookingHandler {
	return &BookingHandler{
		store: store,
	}
}

func (h *BookingHandler) HandleCancelBooking(c *fiber.Ctx) error {
	id := c.Params("id")
	booking, err := h.store.Booking.GetBookingByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNotResourceNotFound("booking")
		}
		return NewDatabaseError("booking lookup", err)
	}
	user, err := getAuthUser(c)
	if err != nil {
		return ErrUnAuthorized()
	}
	if booking.UserID != user.ID {
		return ErrUnAuthorized()
	}
	if err := h.store.Booking.UpdateBooking(c.Context(), c.Params("id"), bson.M{"$set": bson.M{"canceled": true}}); err != nil {
		return err
	}
	metrics.IncrementBookingCancelled()
	return SuccessJSON(c, nil, "Booking canceled successfully")
}

func (h *BookingHandler) HandleGetBookings(c *fiber.Ctx) error {
	user, err := getAuthUser(c)
	if err != nil {
		return ErrUnAuthorized()
	}

	filter := bson.M{"userID": user.ID}
	bookings, err := h.store.Booking.GetBookings(c.Context(), filter)
	if err != nil {
		return NewDatabaseError("bookings lookup", err)
	}
	return SuccessJSON(c, bookings, "Bookings retrieved successfully")
}

func (h *BookingHandler) HandleGetBooking(c *fiber.Ctx) error {
	id := c.Params("id")
	booking, err := h.store.Booking.GetBookingByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNotResourceNotFound("booking")
		}
		return NewDatabaseError("booking lookup", err)
	}
	user, err := getAuthUser(c)
	if err != nil {
		return ErrUnAuthorized()
	}
	if booking.UserID != user.ID {
		return ErrUnAuthorized()
	}
	return SuccessJSON(c, booking, "Booking retrieved successfully")
}
