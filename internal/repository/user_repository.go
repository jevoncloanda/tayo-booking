package repository

import (
	"context"
	"log"
	"tayo-booking/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UserRepository struct {
	DB *pgx.Conn
}

func NewUserRepository(db *pgx.Conn) *UserRepository {
	return &UserRepository{DB: db}
}

func (r *UserRepository) CreateUserWithAuth(ctx context.Context, user *models.User, password string) error {
	authQuery := `
        insert into auth.users (id, email, encrypted_password)
        values ($1, $2, crypt($3, gen_salt('bf')))
        returning id
    `
	var authUserID string
	err := r.DB.QueryRow(ctx, authQuery, user.ID, user.Email, password).Scan(&authUserID)
	if err != nil {
		log.Println("[CreateUserWithAuth] error inserting into auth.users:", err)
		return err
	}

	userQuery := `insert into users (id, name, email, created_at) values ($1, $2, $3, $4)`
	_, err = r.DB.Exec(ctx, userQuery, user.ID, user.Name, user.Email, user.CreatedAt)
	if err != nil {
		log.Println("[CreateUserWithAuth] error inserting into users:", err)
	}
	return err
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `SELECT id, name, email, role, created_at FROM users WHERE email = $1`
	row := r.DB.QueryRow(ctx, query, email)
	var user models.User
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.CreatedAt)
	if err != nil {
		log.Println("[GetUserByEmail] error scanning user:", err)
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query := `SELECT id, name, email, role, created_at FROM users WHERE id = $1`
	row := r.DB.QueryRow(ctx, query, id)
	var user models.User
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.CreatedAt)
	if err != nil {
		log.Println("[GetUserByID] error scanning user:", err)
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) VerifyPassword(ctx context.Context, email, password string) (bool, error) {
	query := `
        SELECT (a.encrypted_password = crypt($2, a.encrypted_password)) AS match
        FROM auth.users a
        JOIN users u ON u.id = a.id
        WHERE u.email = $1
    `
	var match bool
	err := r.DB.QueryRow(ctx, query, email, password).Scan(&match)
	if err != nil {
		log.Println("[VerifyPassword] error verifying password:", err)
	}
	return match, err
}
