package main

import (
    "context"
    "log"

    "github.com/joho/godotenv"
    "tayo-booking/internal/database"
    "tayo-booking/internal/routes"
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

    r := routes.SetupRouter()
    log.Println("Server running on :8080")
    r.Run(":8080")
}