package api

import (
	"context"
	"os"
	"testing"

	"github.com/akaza21/hotel-reservation/db"
	"github.com/akaza21/hotel-reservation/internal/logger"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type testdb struct {
	client *mongo.Client
	*db.Store
}

func (tdb *testdb) teardown(t *testing.T) {
	// Use test database name or fallback to default
	dbname := os.Getenv("MONGO_DB_NAME_TEST")
	if dbname == "" {
		dbname = "hotel-reservation-test"
	}
	
	logger.WithFields(map[string]interface{}{
		"database": dbname,
	}).Debug("Tearing down test database")
	
	if err := tdb.client.Database(dbname).Drop(context.TODO()); err != nil {
		t.Fatal(err)
	}
}

func setup(t *testing.T) *testdb {
	// Initialize logger for tests
	logger.InitLogger()
	
	// Try to load .env file, but don't fail if it doesn't exist
	if err := godotenv.Load("../.env"); err != nil {
		t.Logf("Warning: Could not load .env file: %v", err)
	}
	
	// Get test database URI or use default
	dburi := os.Getenv("MONGO_DB_URL_TEST")
	if dburi == "" {
		dburi = os.Getenv("MONGO_DB_URL")
		if dburi == "" {
			dburi = "mongodb://localhost:27017"
		}
	}
	
	logger.WithFields(map[string]interface{}{
		"test_name": t.Name(),
		"db_uri":    dburi,
	}).Debug("Setting up test database")
	
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(dburi))
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	
	// Ping database to verify connection
	if err := client.Ping(context.TODO(), nil); err != nil {
		t.Fatalf("Failed to ping test database: %v", err)
	}
	
	// Initialize stores
	hotelStore := db.NewMongoHotelStore(client)
	userStore := db.NewMongoUserStore(client)
	roomStore := db.NewMongoRoomStore(client, hotelStore)
	bookingStore := db.NewMongoBookingStore(client)
	
	store := &db.Store{
		Hotel:   hotelStore,
		User:    userStore,
		Room:    roomStore,
		Booking: bookingStore,
	}
	
	logger.WithFields(map[string]interface{}{
		"test_name": t.Name(),
	}).Debug("Test database setup completed")
	
	return &testdb{
		client: client,
		Store:  store,
	}
}
