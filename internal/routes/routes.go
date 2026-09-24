package routes

import (
	"net/http"
	"os"

	"tayo-booking/internal/handlers"

	"github.com/gin-gonic/gin"
)

func corsMiddleware() gin.HandlerFunc {
	allowedOrigin := os.Getenv("FRONTEND_URL")
	if allowedOrigin == "" {
		allowedOrigin = "http://localhost:3000"
	}

	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", allowedOrigin)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func SetupRouter(
	helloHandler *handlers.HelloHandler,
	authHandler *handlers.AuthHandler,
	tripHandler *handlers.TripHandler,
	busHandler *handlers.BusHandler,
	stopHandler *handlers.StopHandler,
	routeHandler *handlers.RouteHandler,
	bookingHandler *handlers.BookingHandler,
) *gin.Engine {
	r := gin.Default()
	r.Use(corsMiddleware())

	r.GET("/hello", helloHandler.Handle)

	// Auth
	r.POST("/auth/register", authHandler.Register)
	r.POST("/auth/login", authHandler.Login)
	r.POST("/auth/refresh", authHandler.Refresh)
	r.POST("/auth/logout", authHandler.Logout)
	r.GET("/me", handlers.AuthMiddleware(), authHandler.Me)

	// Public trip routes
	r.GET("/trips", tripHandler.Search)
	r.GET("/trips/:id", tripHandler.GetByID)
	r.GET("/trips/:id/seats", tripHandler.GetSeats)

	// Public stop routes
	r.GET("/stops", stopHandler.GetAll)

	// User booking routes (authenticated)
	bookings := r.Group("/bookings")
	bookings.Use(handlers.AuthMiddleware())
	{
		bookings.POST("", bookingHandler.Create)
		bookings.GET("", bookingHandler.List)
		bookings.GET("/:id", bookingHandler.GetByID)
		bookings.DELETE("/:id", bookingHandler.Cancel)
	}

	// Admin routes
	admin := r.Group("/admin")
	admin.Use(handlers.AuthMiddleware(), handlers.AdminMiddleware())
	{
		// Trips
		admin.POST("/trips", tripHandler.Create)
		admin.PATCH("/trips/:id", tripHandler.Update)

		// Buses & seats
		admin.GET("/buses", busHandler.List)
		admin.POST("/buses", busHandler.Create)
		admin.PATCH("/seats/:id", busHandler.UpdateSeat)

		// Stops
		admin.POST("/stops", stopHandler.Create)

		// Routes
		admin.GET("/routes", routeHandler.List)
		admin.POST("/routes", routeHandler.Create)
		admin.POST("/route-stops", routeHandler.CreateRouteStop)

		// Bookings
		admin.GET("/bookings", bookingHandler.AdminList)
		admin.PATCH("/bookings/:id", bookingHandler.AdminUpdateStatus)
	}

	return r
}
