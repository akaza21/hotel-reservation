package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/akaza21/hotel-reservation/internal/logger"
	"github.com/gofiber/fiber/v2"
)

type ErrorCode string

const (
	ErrCodeValidation     ErrorCode = "VALIDATION_ERROR"
	ErrCodeAuthentication ErrorCode = "AUTHENTICATION_ERROR"
	ErrCodeAuthorization  ErrorCode = "AUTHORIZATION_ERROR"
	ErrCodeNotFound       ErrorCode = "NOT_FOUND"
	ErrCodeConflict       ErrorCode = "CONFLICT"
	ErrCodeInternal       ErrorCode = "INTERNAL_ERROR"
	ErrCodeRateLimit      ErrorCode = "RATE_LIMIT_ERROR"
	ErrCodeDatabase       ErrorCode = "DATABASE_ERROR"
)

type Error struct {
	Code      int       `json:"code"`
	ErrorCode ErrorCode `json:"error_code"`
	Message   string    `json:"message"`
	Details   string    `json:"details,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type ValidationError struct {
	Field   string `json:"field"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error Error `json:"error"`
}

type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

func (e Error) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.ErrorCode, e.Message, e.Details)
}

func NewError(code int, errorCode ErrorCode, message, details string) Error {
	return Error{
		Code:      code,
		ErrorCode: errorCode,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

func NewValidationError(message string, details string) Error {
	return NewError(http.StatusBadRequest, ErrCodeValidation, message, details)
}

func NewAuthenticationError(message string) Error {
	return NewError(http.StatusUnauthorized, ErrCodeAuthentication, message, "")
}

func NewAuthorizationError(message string) Error {
	return NewError(http.StatusForbidden, ErrCodeAuthorization, message, "")
}

func NewNotFoundError(resource string) Error {
	return NewError(http.StatusNotFound, ErrCodeNotFound, 
		fmt.Sprintf("%s not found", resource), "")
}

func NewConflictError(message string, details string) Error {
	return NewError(http.StatusConflict, ErrCodeConflict, message, details)
}

func NewInternalError(message string, details string) Error {
	return NewError(http.StatusInternalServerError, ErrCodeInternal, message, details)
}

func NewDatabaseError(operation string, err error) Error {
	return NewError(http.StatusInternalServerError, ErrCodeDatabase,
		fmt.Sprintf("Database %s failed", operation),
		err.Error())
}

func ErrInvalidID() Error {
	return NewValidationError("Invalid ID format", "The provided ID is not a valid ObjectID")
}

func ErrBadRequest() Error {
	return NewValidationError("Invalid request format", "Request body contains invalid JSON")
}

func ErrNotResourceNotFound(res string) Error {
	return NewNotFoundError(res)
}

func ErrUnAuthorized() Error {
	return NewAuthenticationError("Authentication required")
}

func ErrTokenExpired() Error {
	return NewAuthenticationError("Token has expired")
}

func ErrInvalidCredentials() Error {
	return NewAuthenticationError("Invalid email or password")
}

func ErrForbidden(message string) Error {
	return NewAuthorizationError(message)
}

func SuccessJSON(c *fiber.Ctx, data interface{}, message string) error {
	return c.JSON(SuccessResponse{
		Success: true,
		Data:    data,
		Message: message,
	})
}

func ErrorHandler(c *fiber.Ctx, err error) error {
	requestID := ""
	if id := c.Locals("request_id"); id != nil {
		requestID = id.(string)
	}

	var apiError Error
	
	switch e := err.(type) {
	case Error:
		apiError = e
		apiError.RequestID = requestID
	case *fiber.Error:
		apiError = NewError(e.Code, ErrCodeInternal, e.Message, "")
		apiError.RequestID = requestID
	default:
		apiError = NewInternalError("An unexpected error occurred", err.Error())
		apiError.RequestID = requestID
		
		logger.WithFields(map[string]interface{}{
			"error":      err.Error(),
			"request_id": requestID,
			"method":     c.Method(),
			"path":       c.Path(),
			"user_agent": c.Get("User-Agent"),
			"ip":         c.IP(),
		}).Error("Unhandled error occurred")
	}

	logger.WithFields(map[string]interface{}{
		"error_code": apiError.ErrorCode,
		"message":    apiError.Message,
		"details":    apiError.Details,
		"request_id": requestID,
		"status":     apiError.Code,
	}).Warn("API error response")

	return c.Status(apiError.Code).JSON(ErrorResponse{Error: apiError})
}