package db

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const DBNAME = "hotel-reservation"
const MongoDBNameEnvName = "MONGO_DB_NAME"
const DBURIFORMAT = "mongodb://%s:%s@%s"

type Map map[string]interface{}

type Pagination struct {
	Limit int64
	Page  int64
}

type Store struct {
	User    UserStore
	Hotel   HotelStore
	Room    RoomStore
	Booking BookingStore
}

type MongoInstance struct {
	Client *mongo.Client
	DB     *mongo.Database
}

var instance *MongoInstance

func Connect(config ...string) (*MongoInstance, error) {
	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}

	db := client.Database(DBNAME)

	instance = &MongoInstance{
		Client: client,
		DB:     db,
	}

	return instance, nil
}