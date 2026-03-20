package repository

import (
	"context"
	"tayo-booking/internal/models"

	"github.com/jackc/pgx/v5"
)

type UserRepository struct {
	DB *pgx.Conn
}

func NewUserRepository(db *pgx.Conn) *UserRepository {
	return &UserRepository{DB: db}
}

func (r *UserRepository) CreateUser(ctx context.Context, user *models.User) error {
	query := `INSERT INTO users (id, name, email, created_at) VALUES ($1, $2, $3, $4)`
	_, err := r.DB.Exec(ctx, query, user.ID, user.Name, user.Email, user.CreatedAt)
	return err
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `SELECT id, name, email, created_at FROM users WHERE email = $1`
	row := r.DB.QueryRow(ctx, query, email)
	var user models.User
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
