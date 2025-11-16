package db

import (
	"context"

	"github.com/akaza21/hotel-reservation/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const roomColl = "rooms"

type RoomStore interface {
	InsertRoom(context.Context, *types.Room) (*types.Room, error)
	GetRooms(context.Context, bson.M) ([]*types.Room, error)
	GetRoomByID(context.Context, string) (*types.Room, error)
}

type MongoRoomStore struct {
	client *mongo.Client
	coll   *mongo.Collection
	HotelStore
}

func NewMongoRoomStore(client *mongo.Client, hotelStore HotelStore) *MongoRoomStore {
	return &MongoRoomStore{
		client:     client,
		coll:       client.Database(DBNAME).Collection(roomColl),
		HotelStore: hotelStore,
	}
}

func (s *MongoRoomStore) GetRooms(ctx context.Context, filter bson.M) ([]*types.Room, error) {
	resp, err := s.coll.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var rooms []*types.Room
	if err := resp.All(ctx, &rooms); err != nil {
		return nil, err
	}
	return rooms, nil
}

func (s *MongoRoomStore) InsertRoom(ctx context.Context, room *types.Room) (*types.Room, error) {
	resp, err := s.coll.InsertOne(ctx, room)
	if err != nil {
		return nil, err
	}
	room.ID = resp.InsertedID.(primitive.ObjectID)

	// update the hotel with this room ID
	filter := bson.M{"_id": room.HotelID}
	update := bson.M{"$push": bson.M{"rooms": room.ID}}
	if err := s.HotelStore.Update(ctx, filter, update); err != nil {
		return nil, err
	}

	return room, nil
}

func (s *MongoRoomStore) GetRoomByID(ctx context.Context, id string) (*types.Room, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	
	var room types.Room
	if err := s.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&room); err != nil {
		return nil, err
	}
	return &room, nil
}