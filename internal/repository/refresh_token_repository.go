package repository

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type RefreshTokenRepository struct {
	DB *pgx.Conn
}

func NewRefreshTokenRepository(db *pgx.Conn) *RefreshTokenRepository {
	return &RefreshTokenRepository{DB: db}
}

func (r *RefreshTokenRepository) StoreRefreshToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time, userAgent, ip string) error {
	query := `
        INSERT INTO refresh_tokens (user_id, token, expires_at, user_agent, ip_address)
        VALUES ($1, $2, $3, $4, $5)
    `
	_, err := r.DB.Exec(ctx, query, userID, token, expiresAt, userAgent, ip)
	if err != nil {
		log.Println("[StoreRefreshToken] error storing refresh token:", err)
	}
	return err
}

func (r *RefreshTokenRepository) ValidateRefreshToken(ctx context.Context, token, userAgent, ip string) (uuid.UUID, error) {
	query := `
        SELECT user_id FROM refresh_tokens
        WHERE token = $1 AND revoked = false AND expires_at > now()
    `
	var userID uuid.UUID
	err := r.DB.QueryRow(ctx, query, token).Scan(&userID)
	if err != nil {
		log.Println("[ValidateRefreshToken] error:", err)
		return uuid.Nil, err
	}
	return userID, nil
}

func (r *RefreshTokenRepository) RotateRefreshToken(ctx context.Context, oldToken, newToken string, expiresAt time.Time, userAgent, ip string) error {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		log.Println("[RotateRefreshToken] begin tx error:", err)
		return err
	}
	defer tx.Rollback(ctx)

	// Revoke old token
	_, err = tx.Exec(ctx, `UPDATE refresh_tokens SET revoked = true WHERE token = $1`, oldToken)
	if err != nil {
		log.Println("[RotateRefreshToken] revoke error:", err)
		return err
	}

	// Get user_id from old token
	var userID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT user_id FROM refresh_tokens WHERE token = $1`, oldToken).Scan(&userID)
	if err != nil {
		log.Println("[RotateRefreshToken] get user_id error:", err)
		return err
	}

	// Insert new token
	_, err = tx.Exec(ctx, `
        INSERT INTO refresh_tokens (user_id, token, expires_at, user_agent, ip_address)
        VALUES ($1, $2, $3, $4, $5)
    `, userID, newToken, expiresAt, userAgent, ip)
	if err != nil {
		log.Println("[RotateRefreshToken] insert new token error:", err)
		return err
	}

	return tx.Commit(ctx)
}

func (r *RefreshTokenRepository) RevokeRefreshToken(ctx context.Context, token string) error {
	_, err := r.DB.Exec(ctx, `UPDATE refresh_tokens SET revoked = true WHERE token = $1`, token)
	if err != nil {
		log.Println("[RevokeRefreshToken] error revoking token:", err)
	}
	return err
}
