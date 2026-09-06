package handlers

import (
	"net/http"
	"tayo-booking/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RouteHandler struct {
	Service *service.RouteService
}

func NewRouteHandler(svc *service.RouteService) *RouteHandler {
	return &RouteHandler{Service: svc}
}

type CreateRouteRequest struct {
	Name string `json:"name" binding:"required"`
}

func (h *RouteHandler) Create(c *gin.Context) {
	var req CreateRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}

	ctx := c.Request.Context()
	route, err := h.Service.CreateRoute(ctx, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, route)
}

type CreateRouteStopRequest struct {
	RouteID   string `json:"route_id"   binding:"required,uuid"`
	StopID    string `json:"stop_id"    binding:"required,uuid"`
	StopOrder int    `json:"stop_order" binding:"required,min=1"`
}

func (h *RouteHandler) CreateRouteStop(c *gin.Context) {
	var req CreateRouteStopRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}

	routeID, _ := uuid.Parse(req.RouteID)
	stopID, _ := uuid.Parse(req.StopID)

	ctx := c.Request.Context()
	rs, err := h.Service.CreateRouteStop(ctx, routeID, stopID, req.StopOrder)
	if err != nil {
		if err.Error() == "duplicate route stop" {
			c.JSON(http.StatusConflict, gin.H{"error": "Duplicate stop_order or stop already exists on this route"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rs)
}

func (h *RouteHandler) List(c *gin.Context) {
	routes, err := h.Service.ListRoutes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, routes)
}
