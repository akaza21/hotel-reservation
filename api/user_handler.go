package api

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/akaza21/hotel-reservation/db"
	"github.com/akaza21/hotel-reservation/internal/logger"
	"github.com/akaza21/hotel-reservation/internal/metrics"
	"github.com/akaza21/hotel-reservation/internal/validator"
	"github.com/akaza21/hotel-reservation/types"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserHandler struct {
	userStore db.UserStore
}

func NewUserHandler(userStore db.UserStore) *UserHandler {
	return &UserHandler{
		userStore: userStore,
	}
}

func (h *UserHandler) HandlePutUser(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	userID := c.Params("id")

	if err := validator.ValidateObjectID(userID); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
		}).Warn("Invalid user ID provided for update")
		return ErrInvalidID()
	}

	var params types.UpdateUserParams
	if err := c.BodyParser(&params); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Failed to parse update user payload")
		return ErrBadRequest()
	}

	if len(params.ToBSON()) == 0 {
		return NewValidationError("No fields to update", "Provide at least one of firstName or lastName")
	}

	oid, _ := primitive.ObjectIDFromHex(userID)
	filter := bson.M{"_id": oid}
	if err := h.userStore.UpdateUser(c.Context(), filter, params); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNotResourceNotFound("user")
		}
		return NewDatabaseError("user update", err)
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
	}).Info("User updated successfully")

	return SuccessJSON(c, fiber.Map{"id": userID}, "User updated successfully")
}

func (h *UserHandler) HandlePostUser(c *fiber.Ctx) error {
	requestID := getRequestID(c)

	var params types.CreateUserParams
	if err := c.BodyParser(&params); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Failed to parse create user payload")
		return ErrBadRequest()
	}

	if validationErrors := params.Validate(); len(validationErrors) > 0 {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"errors":     validationErrors,
		}).Warn("Create user validation failed")
		return NewValidationError("Invalid user data", formatValidationErrors(validationErrors))
	}

	user, err := types.NewUserFromParams(params)
	if err != nil {
		return NewInternalError("Failed to hash password", err.Error())
	}

	insertedUser, err := h.userStore.InsertUser(c.Context(), user)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return NewConflictError("Email already exists", fmt.Sprintf("user with email %s already exists", params.Email))
		}
		return NewDatabaseError("user insertion", err)
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    insertedUser.ID.Hex(),
		"email":      insertedUser.Email,
	}).Info("User created successfully")
	metrics.IncrementUserRegistration()

	return SuccessJSON(c, insertedUser, "User created successfully")
}

func (h *UserHandler) HandleGetUser(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	id := c.Params("id")

	if err := validator.ValidateObjectID(id); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    id,
		}).Warn("Invalid user ID provided for lookup")
		return ErrInvalidID()
	}

	user, err := h.userStore.GetUserByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNotResourceNotFound("user")
		}
		return NewDatabaseError("user lookup", err)
	}

	return SuccessJSON(c, user, "User retrieved successfully")
}

func (h *UserHandler) HandleGetUsers(c *fiber.Ctx) error {
	users, err := h.userStore.GetUsers(c.Context())
	if err != nil {
		return NewDatabaseError("users lookup", err)
	}
	return SuccessJSON(c, users, "Users retrieved successfully")
}

func (h *UserHandler) HandleDeleteUser(c *fiber.Ctx) error {
	requestID := getRequestID(c)
	userID := c.Params("id")

	if err := validator.ValidateObjectID(userID); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
		}).Warn("Invalid user ID provided for deletion")
		return ErrInvalidID()
	}

	if err := h.userStore.DeleteUser(c.Context(), userID); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNotResourceNotFound("user")
		}
		return NewDatabaseError("user deletion", err)
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
	}).Info("User deleted successfully")

	return SuccessJSON(c, fiber.Map{"id": userID}, "User deleted successfully")
}

func getRequestID(c *fiber.Ctx) string {
	if id, ok := c.Locals("request_id").(string); ok {
		return id
	}
	return ""
}

func formatValidationErrors(errs map[string]string) string {
	if len(errs) == 0 {
		return ""
	}

	keys := make([]string, 0, len(errs))
	for field := range errs {
		keys = append(keys, field)
	}
	sort.Strings(keys)

	messages := make([]string, 0, len(keys))
	for _, field := range keys {
		messages = append(messages, fmt.Sprintf("%s: %s", field, errs[field]))
	}

	return strings.Join(messages, "; ")
}
