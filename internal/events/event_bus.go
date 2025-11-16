package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/akaza21/hotel-reservation/internal/logger"
	"github.com/redis/go-redis/v9"
)

type EventType string

const (
	// User Events
	UserRegistered EventType = "user.registered"
	UserLoggedIn   EventType = "user.logged_in"
	
	// Booking Events
	BookingCreated   EventType = "booking.created"
	BookingCancelled EventType = "booking.cancelled"
	BookingConfirmed EventType = "booking.confirmed"
	
	// Hotel Events
	HotelViewed EventType = "hotel.viewed"
	RoomSearched EventType = "room.searched"
	
	// System Events
	MetricRecorded EventType = "metric.recorded"
	LogGenerated   EventType = "log.generated"
)

type Event struct {
	ID        string      `json:"id"`
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	UserID    string      `json:"user_id,omitempty"`
	Data      interface{} `json:"data"`
	Source    string      `json:"source"`
}

type EventBus struct {
	redis *redis.Client
}

func NewEventBus(redisClient *redis.Client) *EventBus {
	return &EventBus{
		redis: redisClient,
	}
}

// Publish event to specific topic
func (eb *EventBus) Publish(ctx context.Context, topic string, event Event) error {
	event.Timestamp = time.Now()
	event.ID = fmt.Sprintf("%s-%d", event.Type, time.Now().UnixNano())
	
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	
	// Using Redis Streams for pub/sub
	err = eb.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: fmt.Sprintf("events:%s", topic),
		Values: map[string]interface{}{
			"event": string(data),
		},
	}).Err()
	
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"topic":      topic,
			"event_type": event.Type,
			"error":      err.Error(),
		}).Error("Failed to publish event")
		return err
	}
	
	logger.WithFields(map[string]interface{}{
		"topic":      topic,
		"event_type": event.Type,
		"event_id":   event.ID,
	}).Info("Event published")
	
	return nil
}

// Subscribe to topic and process events
func (eb *EventBus) Subscribe(ctx context.Context, topic string, consumerGroup string, handler func(Event) error) error {
	streamName := fmt.Sprintf("events:%s", topic)
	
	// Create consumer group if it doesn't exist
	eb.redis.XGroupCreateMkStream(ctx, streamName, consumerGroup, "0").Err()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			streams, err := eb.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    consumerGroup,
				Consumer: "consumer-1",
				Streams:  []string{streamName, ">"},
				Count:    1,
				Block:    1 * time.Second,
			}).Result()
			
			if err != nil {
				if err == redis.Nil {
					continue // No new messages
				}
				logger.WithFields(map[string]interface{}{
					"error": err.Error(),
				}).Error("Failed to read from stream")
				continue
			}
			
			for _, stream := range streams {
				for _, message := range stream.Messages {
					var event Event
					eventData := message.Values["event"].(string)
					
					if err := json.Unmarshal([]byte(eventData), &event); err != nil {
						logger.WithFields(map[string]interface{}{
							"error": err.Error(),
						}).Error("Failed to unmarshal event")
						continue
					}
					
					// Process event
					if err := handler(event); err != nil {
						logger.WithFields(map[string]interface{}{
							"event_id": event.ID,
							"error":    err.Error(),
						}).Error("Failed to handle event")
						continue
					}
					
					// Acknowledge message
					eb.redis.XAck(ctx, streamName, consumerGroup, message.ID)
				}
			}
		}
	}
}

// Event Data Structures
type BookingCreatedData struct {
	BookingID string    `json:"booking_id"`
	HotelID   string    `json:"hotel_id"`
	RoomID    string    `json:"room_id"`
	CheckIn   time.Time `json:"check_in"`
	CheckOut  time.Time `json:"check_out"`
	Amount    float64   `json:"amount"`
}

type UserRegisteredData struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type MetricData struct {
	Name   string                 `json:"name"`
	Value  float64               `json:"value"`
	Labels map[string]string     `json:"labels"`
}