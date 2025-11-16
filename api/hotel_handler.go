package api

import (
	"github.com/akaza21/hotel-reservation/db"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type HotelHandler struct {
	store *db.Store
}

func NewHotelHandler(store *db.Store) *HotelHandler {
	return &HotelHandler{
		store: store,
	}
}

func (h *HotelHandler) HandleGetRooms(c *fiber.Ctx) error {
	id := c.Params("id")
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return ErrInvalidID()
	}

	filter := bson.M{"hotelID": oid}
	rooms, err := h.store.Room.GetRooms(c.Context(), filter)
	if err != nil {
		return ErrNotResourceNotFound("hotel")
	}
	return SuccessJSON(c, rooms, "Rooms retrieved successfully")
}

func (h *HotelHandler) HandleGetHotel(c *fiber.Ctx) error {
	id := c.Params("id")
	hotel, err := h.store.Hotel.GetHotelByID(c.Context(), id)
	if err != nil {
		return ErrNotResourceNotFound("hotel")
	}
	return SuccessJSON(c, hotel, "Hotel retrieved successfully")
}

type ResourceResp struct {
	Results int `json:"results"`
	Data    any `json:"data"`
	Page    int `json:"page"`
}

type HotelQueryParams struct {
	db.Pagination
	Rating int
}

func (h *HotelHandler) HandleGetHotels(c *fiber.Ctx) error {
	var params HotelQueryParams
	if err := c.QueryParser(&params); err != nil {
		return ErrBadRequest()
	}
	filter := bson.M{}
	if params.Rating > 0 {
		filter["rating"] = params.Rating
	}
	if params.Page == 0 {
		params.Page = 1
	}
	if params.Limit == 0 {
		params.Limit = 50
	}
	hotels, err := h.store.Hotel.GetHotels(c.Context(), filter, &params.Pagination)
	if err != nil {
		return ErrNotResourceNotFound("hotels")
	}
	resp := ResourceResp{
		Data:    hotels,
		Results: len(hotels),
		Page:    int(params.Page),
	}
	return SuccessJSON(c, resp, "Hotels retrieved successfully")
}
