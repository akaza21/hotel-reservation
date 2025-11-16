package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/akaza21/hotel-reservation/api"
	"github.com/akaza21/hotel-reservation/db"
	"github.com/akaza21/hotel-reservation/db/fixtures"
	"github.com/akaza21/hotel-reservation/internal/config"
	"github.com/akaza21/hotel-reservation/internal/logger"
	"github.com/akaza21/hotel-reservation/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type SeedData struct {
	Users    []*types.User
	Hotels   []*types.Hotel
	Rooms    []*types.Room
	Bookings []*types.Booking
	Tokens   map[string]string
}

func main() {
	// Initialize logger
	logger.InitLogger()
	logger.Info("Starting comprehensive database seeding")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}

	// Connect to MongoDB
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.Database.URI))
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}
	defer client.Disconnect(ctx)

	// Initialize stores
	store := &db.Store{
		User:    db.NewMongoUserStore(client),
		Hotel:   db.NewMongoHotelStore(client),
		Room:    db.NewMongoRoomStore(client, db.NewMongoHotelStore(client)),
		Booking: db.NewMongoBookingStore(client),
	}

	// Create auth handler for token generation
	authHandler := api.NewAuthHandler(store.User, cfg)

	seedData := &SeedData{
		Users:    make([]*types.User, 0),
		Hotels:   make([]*types.Hotel, 0),
		Rooms:    make([]*types.Room, 0),
		Bookings: make([]*types.Booking, 0),
		Tokens:   make(map[string]string),
	}

	logger.Info("Creating users...")
	createUsers(store, authHandler, seedData)

	logger.Info("Creating hotels and rooms...")
	createHotelsAndRooms(store, seedData)

	logger.Info("Creating bookings...")
	createBookings(store, seedData)

	logger.Info("Seeding completed successfully!")
	printSeedSummary(seedData)
}

func createUsers(store *db.Store, authHandler *api.AuthHandler, seedData *SeedData) {
	userConfigs := []struct {
		firstName string
		lastName  string
		email     string
		password  string
		isAdmin   bool
	}{
		// Admin users
		{"Super", "Admin", "admin@hotel.com", "admin123", true},
		{"Manager", "One", "manager1@hotel.com", "manager123", true},

		// Regular users - Business travelers
		{"James", "Wilson", "james.wilson@business.com", "password123", false},
		{"Sarah", "Johnson", "sarah.johnson@corp.com", "password123", false},
		{"Michael", "Brown", "michael.brown@company.com", "password123", false},
		{"Emily", "Davis", "emily.davis@enterprise.com", "password123", false},
		{"David", "Miller", "david.miller@business.org", "password123", false},

		// Regular users - Tourists
		{"Alice", "Smith", "alice.smith@gmail.com", "password123", false},
		{"Bob", "Taylor", "bob.taylor@yahoo.com", "password123", false},
		{"Carol", "Anderson", "carol.anderson@outlook.com", "password123", false},
		{"Daniel", "Moore", "daniel.moore@gmail.com", "password123", false},
		{"Emma", "Jackson", "emma.jackson@hotmail.com", "password123", false},
		{"Frank", "White", "frank.white@gmail.com", "password123", false},
		{"Grace", "Harris", "grace.harris@yahoo.com", "password123", false},
		{"Henry", "Martin", "henry.martin@outlook.com", "password123", false},
		{"Ivy", "Thompson", "ivy.thompson@gmail.com", "password123", false},
		{"Jack", "Garcia", "jack.garcia@yahoo.com", "password123", false},
		{"Kate", "Martinez", "kate.martinez@gmail.com", "password123", false},
		{"Leo", "Robinson", "leo.robinson@outlook.com", "password123", false},
		{"Mia", "Clark", "mia.clark@gmail.com", "password123", false},
		{"Noah", "Rodriguez", "noah.rodriguez@yahoo.com", "password123", false},
		{"Olivia", "Lewis", "olivia.lewis@gmail.com", "password123", false},
	}

	for _, config := range userConfigs {
		user := fixtures.AddUserWithCredentials(store, config.firstName, config.lastName, config.email, config.password, config.isAdmin)
		seedData.Users = append(seedData.Users, user)

		// Generate token for each user
		token, err := authHandler.CreateTokenFromUser(user)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"user_email": user.Email,
				"error":      err.Error(),
			}).Error("Failed to create token for user")
			continue
		}
		seedData.Tokens[user.Email] = token

		logger.WithFields(map[string]interface{}{
			"user_id":  user.ID,
			"email":    user.Email,
			"is_admin": user.IsAdmin,
		}).Info("Created user")
	}
}

func createHotelsAndRooms(store *db.Store, seedData *SeedData) {
	hotelConfigs := []struct {
		name     string
		location string
		rating   int
		rooms    []struct {
			size    string
			seaside bool
			price   float64
			count   int
		}
	}{
		{
			name:     "Grand Palace Hotel",
			location: "New York City, NY",
			rating:   5,
			rooms: []struct {
				size    string
				seaside bool
				price   float64
				count   int
			}{
				{"small", false, 299.99, 5},
				{"normal", false, 399.99, 8},
				{"kingsize", false, 599.99, 3},
				{"suite", false, 899.99, 2},
			},
		},
		{
			name:     "Ocean View Resort",
			location: "Miami Beach, FL",
			rating:   5,
			rooms: []struct {
				size    string
				seaside bool
				price   float64
				count   int
			}{
				{"small", true, 399.99, 6},
				{"normal", true, 499.99, 10},
				{"kingsize", true, 699.99, 4},
				{"suite", true, 1199.99, 2},
			},
		},
		{
			name:     "Mountain Lodge",
			location: "Aspen, CO",
			rating:   4,
			rooms: []struct {
				size    string
				seaside bool
				price   float64
				count   int
			}{
				{"small", false, 199.99, 8},
				{"normal", false, 299.99, 12},
				{"kingsize", false, 449.99, 6},
				{"cabin", false, 599.99, 4},
			},
		},
		{
			name:     "Business Central Hotel",
			location: "Chicago, IL",
			rating:   4,
			rooms: []struct {
				size    string
				seaside bool
				price   float64
				count   int
			}{
				{"small", false, 179.99, 15},
				{"normal", false, 249.99, 20},
				{"kingsize", false, 349.99, 8},
				{"executive", false, 499.99, 5},
			},
		},
		{
			name:     "Desert Oasis Resort",
			location: "Las Vegas, NV",
			rating:   5,
			rooms: []struct {
				size    string
				seaside bool
				price   float64
				count   int
			}{
				{"small", false, 249.99, 12},
				{"normal", false, 349.99, 18},
				{"kingsize", false, 549.99, 10},
				{"penthouse", false, 1499.99, 3},
			},
		},
		{
			name:     "Coastal Retreat",
			location: "San Diego, CA",
			rating:   4,
			rooms: []struct {
				size    string
				seaside bool
				price   float64
				count   int
			}{
				{"small", true, 279.99, 10},
				{"normal", true, 379.99, 14},
				{"kingsize", true, 499.99, 8},
				{"villa", true, 799.99, 4},
			},
		},
		{
			name:     "Historic Downtown Inn",
			location: "Boston, MA",
			rating:   3,
			rooms: []struct {
				size    string
				seaside bool
				price   float64
				count   int
			}{
				{"small", false, 149.99, 12},
				{"normal", false, 199.99, 16},
				{"kingsize", false, 279.99, 6},
			},
		},
		{
			name:     "Luxury Sky Tower",
			location: "Seattle, WA",
			rating:   5,
			rooms: []struct {
				size    string
				seaside bool
				price   float64
				count   int
			}{
				{"normal", false, 329.99, 20},
				{"kingsize", false, 449.99, 15},
				{"suite", false, 699.99, 8},
				{"presidential", false, 1299.99, 2},
			},
		},
	}

	for _, config := range hotelConfigs {
		hotel := fixtures.AddHotel(store, config.name, config.location, config.rating, nil)
		seedData.Hotels = append(seedData.Hotels, hotel)

		logger.WithFields(map[string]interface{}{
			"hotel_id": hotel.ID,
			"name":     hotel.Name,
			"location": hotel.Location,
			"rating":   hotel.Rating,
		}).Info("Created hotel")

		// Create rooms for this hotel
		for _, roomConfig := range config.rooms {
			for i := 0; i < roomConfig.count; i++ {
				room := fixtures.AddRoom(store, roomConfig.size, roomConfig.seaside, roomConfig.price, hotel.ID)
				seedData.Rooms = append(seedData.Rooms, room)

				logger.WithFields(map[string]interface{}{
					"room_id":  room.ID,
					"hotel_id": hotel.ID,
					"size":     room.Size,
					"price":    room.Price,
					"seaside":  room.Seaside,
				}).Debug("Created room")
			}
		}
	}
}

func createBookings(store *db.Store, seedData *SeedData) {
	if len(seedData.Users) == 0 || len(seedData.Rooms) == 0 {
		log.Println("No users or rooms available for booking creation")
		return
	}

	// Create bookings with realistic patterns
	bookingScenarios := []struct {
		userEmail     string
		daysFromNow   int
		duration      int
		numPersons    int
		preferSeaside bool
		cancelled     bool
	}{
		// Recent bookings (past)
		{"james.wilson@business.com", -30, 3, 1, false, false},
		{"sarah.johnson@corp.com", -25, 2, 2, false, false},
		{"alice.smith@gmail.com", -20, 5, 4, true, false},
		{"bob.taylor@yahoo.com", -15, 1, 1, false, true}, // Cancelled
		{"michael.brown@company.com", -10, 4, 2, false, false},

		// Current bookings (ongoing)
		{"emily.davis@enterprise.com", -2, 5, 1, false, false},
		{"carol.anderson@outlook.com", -1, 3, 2, true, false},

		// Upcoming bookings (future)
		{"daniel.moore@gmail.com", 5, 3, 2, false, false},
		{"emma.jackson@hotmail.com", 10, 7, 4, true, false},
		{"frank.white@gmail.com", 15, 2, 1, false, false},
		{"grace.harris@yahoo.com", 20, 4, 3, true, false},
		{"henry.martin@outlook.com", 25, 6, 2, false, false},
		{"ivy.thompson@gmail.com", 30, 3, 2, true, false},
		{"jack.garcia@yahoo.com", 35, 5, 1, false, false},
		{"kate.martinez@gmail.com", 40, 2, 4, false, false},
		{"leo.robinson@outlook.com", 45, 8, 2, true, false},
		{"david.miller@business.org", 50, 3, 1, false, false},
		{"mia.clark@gmail.com", 60, 4, 3, true, false},
		{"noah.rodriguez@yahoo.com", 70, 2, 2, false, false},
		{"olivia.lewis@gmail.com", 80, 6, 4, true, false},
	}

	for _, scenario := range bookingScenarios {
		// Find user by email
		var user *types.User
		for _, u := range seedData.Users {
			if u.Email == scenario.userEmail {
				user = u
				break
			}
		}
		if user == nil {
			continue
		}

		// Find suitable room based on preferences
		var selectedRoom *types.Room
		for _, room := range seedData.Rooms {
			if scenario.preferSeaside && !room.Seaside {
				continue
			}
			// Prefer normal+ rooms for multiple persons
			if scenario.numPersons > 1 && room.Size == "small" {
				continue
			}
			selectedRoom = room
			break
		}
		if selectedRoom == nil {
			selectedRoom = seedData.Rooms[rand.Intn(len(seedData.Rooms))]
		}

		// Calculate dates
		fromDate := time.Now().AddDate(0, 0, scenario.daysFromNow)
		tillDate := fromDate.AddDate(0, 0, scenario.duration)

		booking := fixtures.AddBooking(store, user.ID, selectedRoom.ID, fromDate, tillDate)
		booking.NumPersons = scenario.numPersons
		booking.Canceled = scenario.cancelled

		// Update the booking with additional fields
		update := bson.M{
			"$set": bson.M{
				"numPersons": scenario.numPersons,
				"canceled":   scenario.cancelled,
			},
		}
		if err := store.Booking.UpdateBooking(context.Background(), booking.ID.Hex(), update); err != nil {
			logger.WithFields(map[string]interface{}{
				"booking_id": booking.ID,
				"error":      err.Error(),
			}).Error("Failed to update booking")
		}

		seedData.Bookings = append(seedData.Bookings, booking)

		logger.WithFields(map[string]interface{}{
			"booking_id":  booking.ID,
			"user_email":  user.Email,
			"room_id":     selectedRoom.ID,
			"from_date":   fromDate,
			"till_date":   tillDate,
			"num_persons": scenario.numPersons,
			"cancelled":   scenario.cancelled,
		}).Info("Created booking")
	}
}

func printSeedSummary(seedData *SeedData) {
	divider := strings.Repeat("=", 60)
	fmt.Println("\n" + divider)
	fmt.Println("COMPREHENSIVE SEED DATA SUMMARY")
	fmt.Println(divider)

	fmt.Printf("Users Created: %d\n", len(seedData.Users))
	fmt.Printf("   Admins: %d\n", countAdmins(seedData.Users))
	fmt.Printf("   Regular Users: %d\n", len(seedData.Users)-countAdmins(seedData.Users))

	fmt.Printf("\nHotels Created: %d\n", len(seedData.Hotels))
	for _, hotel := range seedData.Hotels {
		roomCount := countRoomsForHotel(seedData.Rooms, hotel.ID.Hex())
		fmt.Printf("   %s (%s) - %d stars - %d rooms\n",
			hotel.Name, hotel.Location, hotel.Rating, roomCount)
	}

	fmt.Printf("\nTotal Rooms Created: %d\n", len(seedData.Rooms))
	roomTypes := make(map[string]int)
	for _, room := range seedData.Rooms {
		roomTypes[room.Size]++
	}
	for roomType, count := range roomTypes {
		fmt.Printf("   %s: %d rooms\n", roomType, count)
	}

	fmt.Printf("\nBookings Created: %d\n", len(seedData.Bookings))
	activeBookings := 0
	cancelledBookings := 0
	for _, booking := range seedData.Bookings {
		if booking.Canceled {
			cancelledBookings++
		} else {
			activeBookings++
		}
	}
	fmt.Printf("   Active: %d\n", activeBookings)
	fmt.Printf("   Cancelled: %d\n", cancelledBookings)

	fmt.Println("\nSample API Tokens (first five):")
	tokenCount := 0
	for email, token := range seedData.Tokens {
		if tokenCount >= 5 {
			break
		}
		fmt.Printf("   %s:\n   %s\n\n", email, token)
		tokenCount++
	}

	fmt.Println(divider)
	fmt.Println("Database seeding completed.")
	fmt.Println(divider)
}

func countAdmins(users []*types.User) int {
	count := 0
	for _, user := range users {
		if user.IsAdmin {
			count++
		}
	}
	return count
}

func countRoomsForHotel(rooms []*types.Room, hotelID string) int {
	count := 0
	for _, room := range rooms {
		if room.HotelID.Hex() == hotelID {
			count++
		}
	}
	return count
}
