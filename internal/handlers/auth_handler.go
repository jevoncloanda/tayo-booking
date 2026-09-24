package handlers

import (
	"net/http"
	"tayo-booking/internal/service"

	"os"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type AuthHandler struct {
	Service *service.AuthService
}

func NewAuthHandler(service *service.AuthService) *AuthHandler {
	return &AuthHandler{Service: service}
}

type RegisterUserRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Custom validation error formatting
		details := make(map[string]string)
		if verrs, ok := err.(validator.ValidationErrors); ok {
			for _, verr := range verrs {
				field := verr.Field()
				switch field {
				case "Email":
					details["email"] = "Invalid email format"
				case "Password":
					if verr.Tag() == "min" {
						details["password"] = "Password must be at least 8 characters"
					} else {
						details["password"] = "Invalid password"
					}
				case "Name":
					details["name"] = "Name is required"
				}
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Validation failed",
				"details": details,
			})
			return
		}
		// fallback for other errors
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	user, err := h.Service.RegisterUserWithPassword(ctx, req.Name, req.Email, req.Password)
	if err != nil {
		// A taken email is a client-correctable conflict, not a server fault, so
		// the client can point at the email field instead of showing a generic
		// "try again" that will never succeed.
		if err.Error() == "email already registered" {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "Email already registered",
				"details": gin.H{"email": "An account with this email already exists"},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, user)
}

type LoginUserRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		details := make(map[string]string)
		if verrs, ok := err.(validator.ValidationErrors); ok {
			for _, verr := range verrs {
				field := verr.Field()
				switch field {
				case "Email":
					details["email"] = "Invalid email format"
				case "Password":
					details["password"] = "Password is required"
				}
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Validation failed",
				"details": details,
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tokens, err := h.Service.LoginUser(ctx, req.Email, req.Password, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Set access_token cookie (expires in 1 hour)
	c.SetCookie(
		"access_token",
		tokens.AccessToken,
		3600, // 1 hour in seconds
		"/",
		"",
		true, // secure (set to true in production)
		true, // httpOnly
	)

	// Set refresh_token cookie (expires in 7 days)
	c.SetCookie(
		"refresh_token",
		tokens.RefreshToken,
		7*24*3600, // 7 days in seconds
		"/",
		"",
		true, // secure (set to true in production)
		true, // httpOnly
	)

	c.JSON(http.StatusOK, gin.H{"message": "Login successful"})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	idStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Returns the full user (including role) so clients can tell an admin from
	// a regular user without a second round-trip.
	user, err := h.Service.UserRepo.GetUserByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing refresh token"})
		return
	}

	ctx := c.Request.Context()
	tokens, err := h.Service.RefreshTokens(ctx, refreshToken, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Set new cookies
	c.SetCookie("access_token", tokens.AccessToken, 3600, "/", "", true, true)
	c.SetCookie("refresh_token", tokens.RefreshToken, 7*24*3600, "/", "", true, true)
	c.JSON(http.StatusOK, gin.H{"message": "Token refreshed"})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing refresh token"})
		return
	}

	ctx := c.Request.Context()
	err = h.Service.Logout(ctx, refreshToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	// Clear cookies
	c.SetCookie("access_token", "", -1, "/", "", true, true)
	c.SetCookie("refresh_token", "", -1, "/", "", true, true)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		jwtSecret := []byte(os.Getenv("JWT_SECRET"))
		tokenStr, err := c.Cookie("access_token")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing access token"})
			return
		}
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}
		c.Set("user_id", claims["user_id"])
		c.Set("role", claims["role"])
		c.Next()
	}
}

func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		if role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
			return
		}
		c.Next()
	}
}
