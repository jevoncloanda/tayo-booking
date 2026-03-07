package database

import (
	"context"
	"errors"
	"os"

	"github.com/jackc/pgx/v5"
)

func Connect() (*pgx.Conn, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is not set")
	}

	conn, err := pgx.Connect(context.Background(), databaseURL)
	if err != nil {
		return nil, err
	}

	return conn, nil
}