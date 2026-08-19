package routes

import (
	"tayo-booking/internal/handlers"

	"github.com/gin-gonic/gin"
)

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
		admin.POST("/buses", busHandler.Create)
		admin.PATCH("/seats/:id", busHandler.UpdateSeat)

		// Stops
		admin.POST("/stops", stopHandler.Create)

		// Routes
		admin.POST("/routes", routeHandler.Create)
		admin.POST("/route-stops", routeHandler.CreateRouteStop)

		// Bookings
		admin.GET("/bookings", bookingHandler.AdminList)
		admin.PATCH("/bookings/:id", bookingHandler.AdminUpdateStatus)
	}

	return r
}
