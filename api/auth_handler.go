package api

import (
	"errors"
	"time"

	"github.com/akaza21/hotel-reservation/db"
	"github.com/akaza21/hotel-reservation/internal/config"
	"github.com/akaza21/hotel-reservation/internal/logger"
	"github.com/akaza21/hotel-reservation/internal/metrics"
	"github.com/akaza21/hotel-reservation/internal/validator"
	"github.com/akaza21/hotel-reservation/types"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/mongo"
)

type AuthHandler struct {
	userStore db.UserStore
	config    *config.Config
}

func NewAuthHandler(userStore db.UserStore, config *config.Config) *AuthHandler {
	return &AuthHandler{
		userStore: userStore,
		config:    config,
	}
}

type AuthParams struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

type AuthResponse struct {
	User  *types.User `json:"user"`
	Token string      `json:"token"`
}

func (h *AuthHandler) HandleAuthenticate(c *fiber.Ctx) error {
	requestID := c.Locals("request_id").(string)

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"ip":         c.IP(),
		"user_agent": c.Get("User-Agent"),
	}).Info("Authentication attempt started")

	var params AuthParams
	if err := c.BodyParser(&params); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Failed to parse authentication request body")
		return ErrBadRequest()
	}

	if err := validator.ValidateStruct(params); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"email":      params.Email,
		}).Warn("Authentication request validation failed")
		metrics.IncrementAuthFailure()
		return NewValidationError(err.Error(), "")
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"email":      params.Email,
	}).Debug("Attempting to authenticate user")

	user, err := h.userStore.GetUserByEmail(c.Context(), params.Email)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"email":      params.Email,
			}).Warn("Authentication failed - user not found")
			metrics.IncrementAuthFailure()
			return ErrInvalidCredentials()
		}

		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"email":      params.Email,
			"error":      err.Error(),
		}).Error("Database error during authentication")
		return NewDatabaseError("user lookup", err)
	}

	if !types.IsValidPassword(user.EncryptedPassword, params.Password) {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"email":      params.Email,
			"user_id":    user.ID,
		}).Warn("Authentication failed - invalid password")
		metrics.IncrementAuthFailure()
		return ErrInvalidCredentials()
	}

	token, err := h.CreateTokenFromUser(user)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    user.ID,
			"error":      err.Error(),
		}).Error("Failed to create JWT token")
		return NewInternalError("Token creation failed", err.Error())
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    user.ID,
		"email":      user.Email,
	}).Info("Authentication successful")
	metrics.IncrementAuthSuccess()

	resp := AuthResponse{
		User:  user,
		Token: token,
	}

	return SuccessJSON(c, resp, "Authentication successful")
}

func (h *AuthHandler) CreateTokenFromUser(user *types.User) (string, error) {
	now := time.Now()
	expires := now.Add(time.Hour * time.Duration(h.config.JWT.ExpirationHours)).Unix()

	claims := jwt.MapClaims{
		"id":      user.ID.Hex(),
		"email":   user.Email,
		"expires": expires,
		"iat":     now.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(h.config.JWT.Secret))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": user.ID,
			"error":   err.Error(),
		}).Error("Failed to sign JWT token")
		return "", err
	}

	return tokenStr, nil
}
