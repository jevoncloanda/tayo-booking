package service

import (
	"context"
	"tayo-booking/internal/models"
	"tayo-booking/internal/repository"
	"time"

	"github.com/google/uuid"
)

type UserService struct {
	Repo *repository.UserRepository
}

func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{Repo: repo}
}

func (s *UserService) RegisterUser(ctx context.Context, name, email string) (*models.User, error) {
	user := &models.User{
		ID:        uuid.New(),
		Name:      &name,
		Email:     email,
		CreatedAt: time.Now(),
	}
	err := s.Repo.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}
	return user, nil
}
