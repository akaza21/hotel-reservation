package logger

import (
	"context"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

var Log *logrus.Logger

type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	UserIDKey    contextKey = "user_id"
)

func InitLogger() {
	Log = logrus.New()
	Log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})

	level := os.Getenv("LOG_LEVEL")
	switch level {
	case "debug":
		Log.SetLevel(logrus.DebugLevel)
	case "warn":
		Log.SetLevel(logrus.WarnLevel)
	case "error":
		Log.SetLevel(logrus.ErrorLevel)
	default:
		Log.SetLevel(logrus.InfoLevel)
	}

	Log.SetOutput(os.Stdout)
}

func WithContext(ctx context.Context) *logrus.Entry {
	entry := Log.WithContext(ctx)
	
	if requestID := ctx.Value(RequestIDKey); requestID != nil {
		entry = entry.WithField("request_id", requestID)
	}
	
	if userID := ctx.Value(UserIDKey); userID != nil {
		entry = entry.WithField("user_id", userID)
	}
	
	return entry
}

func WithFields(fields logrus.Fields) *logrus.Entry {
	return Log.WithFields(fields)
}

func Info(args ...interface{}) {
	Log.Info(args...)
}

func Debug(args ...interface{}) {
	Log.Debug(args...)
}

func Warn(args ...interface{}) {
	Log.Warn(args...)
}

func Error(args ...interface{}) {
	Log.Error(args...)
}

func Fatal(args ...interface{}) {
	Log.Fatal(args...)
}

func LogRequest(c *fiber.Ctx) {
	start := time.Now()
	
	Log.WithFields(logrus.Fields{
		"method":     c.Method(),
		"path":       c.Path(),
		"ip":         c.IP(),
		"user_agent": c.Get("User-Agent"),
		"request_id": c.Locals("request_id"),
	}).Info("Request started")

	c.Locals("start_time", start)
}

func LogResponse(c *fiber.Ctx, err error) {
	start, ok := c.Locals("start_time").(time.Time)
	if !ok {
		start = time.Now()
	}
	
	duration := time.Since(start)
	
	fields := logrus.Fields{
		"method":      c.Method(),
		"path":        c.Path(),
		"status_code": c.Response().StatusCode(),
		"duration_ms": duration.Milliseconds(),
		"request_id":  c.Locals("request_id"),
	}

	if err != nil {
		fields["error"] = err.Error()
		Log.WithFields(fields).Error("Request completed with error")
	} else {
		Log.WithFields(fields).Info("Request completed successfully")
	}
}