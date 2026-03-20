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

	// Wire up dependencies for user registration
	userRepo := repository.NewUserRepository(conn)
	userService := service.NewUserService(userRepo)
	userHandler := handlers.NewUserHandler(userService)
	helloHandler := &handlers.HelloHandler{}

	r := routes.SetupRouter(helloHandler, userHandler)
	log.Println("Server running on :8080")
	r.Run(":8080")
}
