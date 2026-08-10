package service

import (
	"context"
	"errors"
	"os"
	"time"

	"tayo-booking/internal/models"
	"tayo-booking/internal/repository"

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

// role is now embedded in the JWT so AdminMiddleware
// never needs a DB round-trip on every request.
func generateJWT(userID, role string) (string, error) {
	jwtSecret := []byte(os.Getenv("JWT_SECRET"))
	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     time.Now().Add(time.Hour * 1).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func (s *AuthService) LoginUser(ctx context.Context, email, password, userAgent, ip string) (*TokenPair, error) {
	match, err := s.UserRepo.VerifyPassword(ctx, email, password)
	if err != nil || !match {
		return nil, errors.New("invalid email or password")
	}

	user, err := s.UserRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, errors.New("user not found")
	}

	accessToken, err := generateJWT(user.ID.String(), user.Role)
	if err != nil {
		return nil, errors.New("failed to generate token")
	}

	refreshToken := uuid.New().String()
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

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
	userID, err := s.RefreshTokenRepo.ValidateRefreshToken(ctx, refreshToken, userAgent, ip)
	if err != nil {
		return nil, errors.New("invalid or expired refresh token")
	}

	// Fetch user to get current role — role may have changed since last login
	user, err := s.UserRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	accessToken, err := generateJWT(userID.String(), user.Role)
	if err != nil {
		return nil, errors.New("failed to generate access token")
	}

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
