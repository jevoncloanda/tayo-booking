package service

import (
	"context"
	"errors"
	"tayo-booking/internal/models"
	"tayo-booking/internal/repository"
	"time"

	"os"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type AuthService struct {
	UserRepo         *repository.UserRepository
	RefreshTokenRepo *repository.RefreshTokenRepository
}

func NewAuthService(userRepo *repository.UserRepository, refreshTokenRepo *repository.RefreshTokenRepository) *AuthService {
	return &AuthService{
		UserRepo:         userRepo,
		RefreshTokenRepo: refreshTokenRepo,
	}
}

func (s *AuthService) RegisterUserWithPassword(ctx context.Context, name, email, password string) (*models.User, error) {
	// Check for duplicate email
	existing, err := s.UserRepo.GetUserByEmail(ctx, email)
	if err == nil && existing != nil {
		return nil, errors.New("email already registered")
	}

	user := &models.User{
		ID:        uuid.New(),
		Name:      &name,
		Email:     email,
		CreatedAt: time.Now(),
	}
	err = s.UserRepo.CreateUserWithAuth(ctx, user, password)
	if err != nil {
		return nil, err
	}
	return user, nil
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Generate JWT
func generateJWT(userID string) (string, error) {
	jwtSecret := []byte(os.Getenv("JWT_SECRET"))
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour * 1).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func (s *AuthService) LoginUser(ctx context.Context, email, password, userAgent, ip string) (*TokenPair, error) {
	// Verify password using PostgreSQL's crypt
	match, err := s.UserRepo.VerifyPassword(ctx, email, password)
	if err != nil || !match {
		return nil, errors.New("invalid email or password")
	}

	user, err := s.UserRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Generate JWT
	accessToken, err := generateJWT(user.ID.String())
	if err != nil {
		return nil, errors.New("failed to generate token")
	}

	// Generate refresh token (random UUID)
	refreshToken := uuid.New().String()
	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7 days

	// Store refresh token
	err = s.RefreshTokenRepo.StoreRefreshToken(ctx, user.ID, refreshToken, expiresAt, userAgent, ip)
	if err != nil {
		return nil, errors.New("failed to store refresh token")
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *AuthService) RefreshTokens(ctx context.Context, refreshToken, userAgent, ip string) (*TokenPair, error) {
	// Validate refresh token
	userID, err := s.RefreshTokenRepo.ValidateRefreshToken(ctx, refreshToken, userAgent, ip)
	if err != nil {
		return nil, errors.New("invalid or expired refresh token")
	}

	// Generate new access token
	accessToken, err := generateJWT(userID.String())
	if err != nil {
		return nil, errors.New("failed to generate access token")
	}

	// Rotate refresh token: create new, revoke old
	newRefreshToken := uuid.New().String()
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	err = s.RefreshTokenRepo.RotateRefreshToken(ctx, refreshToken, newRefreshToken, expiresAt, userAgent, ip)
	if err != nil {
		return nil, errors.New("failed to rotate refresh token")
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	return s.RefreshTokenRepo.RevokeRefreshToken(ctx, refreshToken)
}
