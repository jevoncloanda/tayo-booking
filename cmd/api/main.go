package main

import (
	"context"
	"log"

	"tayo-booking/internal/database"
	"tayo-booking/internal/handlers"
	"tayo-booking/internal/repository"
	"tayo-booking/internal/routes"
	"tayo-booking/internal/service"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading it")
	}

	conn, err := database.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	defer conn.Close(context.Background())

	log.Println("DB Connected")

	// Auth
	userRepo := repository.NewUserRepository(conn)
	refreshTokenRepo := repository.NewRefreshTokenRepository(conn)
	authService := service.NewAuthService(userRepo, refreshTokenRepo)
	authHandler := handlers.NewAuthHandler(authService)

	// Trips
	tripRepo := repository.NewTripRepository(conn)
	tripService := service.NewTripService(tripRepo)
	tripHandler := handlers.NewTripHandler(tripService)

	helloHandler := &handlers.HelloHandler{}

	r := routes.SetupRouter(helloHandler, authHandler, tripHandler)
	log.Println("Server running on :8080")
	r.Run(":8080")
}
