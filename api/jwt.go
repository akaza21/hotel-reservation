package api

import (
	"context"
	"strings"
	"time"

	"github.com/akaza21/hotel-reservation/db"
	"github.com/akaza21/hotel-reservation/internal/config"
	"github.com/akaza21/hotel-reservation/internal/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func JWTAuthentication(userStore db.UserStore, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		requestID := c.Locals("request_id").(string)
		
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"method":     c.Method(),
			"path":       c.Path(),
		}).Debug("JWT authentication started")

		authHeader := c.Get("Authorization")
		if authHeader == "" {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"ip":         c.IP(),
			}).Warn("Authentication attempt without Authorization header")
			return ErrUnAuthorized()
		}
		
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenStr == authHeader {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"ip":         c.IP(),
			}).Warn("Authentication attempt with invalid header format")
			return ErrUnAuthorized()
		}

		claims, err := validateToken(tokenStr, cfg.JWT.Secret)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"error":      err.Error(),
				"ip":         c.IP(),
			}).Warn("Token validation failed")
			return err
		}

		if err := validateTokenExpiration(claims); err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"ip":         c.IP(),
				"expires":    claims["expires"],
			}).Warn("Token expired")
			return err
		}

		userID, ok := claims["id"].(string)
		if !ok {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"claims":     claims,
			}).Error("Invalid user ID in token claims")
			return ErrUnAuthorized()
		}

		user, err := userStore.GetUserByID(c.Context(), userID)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"user_id":    userID,
				"error":      err.Error(),
			}).Error("Failed to fetch user from database")
			return ErrUnAuthorized()
		}

		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"user_email": user.Email,
		}).Debug("Authentication successful")

		ctx := context.WithValue(c.Context(), logger.UserIDKey, userID)
		c.SetUserContext(ctx)
		c.Context().SetUserValue("user", user)
		
		return c.Next()
	}
}

func validateToken(tokenStr, secret string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			logger.WithFields(map[string]interface{}{
				"algorithm": token.Header["alg"],
			}).Error("Invalid JWT signing method")
			return nil, ErrUnAuthorized()
		}
		return []byte(secret), nil
	})
	
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to parse JWT token")
		return nil, ErrUnAuthorized()
	}

	if !token.Valid {
		logger.Error("Invalid JWT token")
		return nil, ErrUnAuthorized()
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		logger.Error("Failed to extract JWT claims")
		return nil, ErrUnAuthorized()
	}

	return claims, nil
}

func validateTokenExpiration(claims jwt.MapClaims) error {
	expiresFloat, ok := claims["expires"].(float64)
	if !ok {
		logger.Error("Invalid expires claim in token")
		return ErrUnAuthorized()
	}

	expires := int64(expiresFloat)
	if time.Now().Unix() > expires {
		return ErrTokenExpired()
	}

	return nil
}