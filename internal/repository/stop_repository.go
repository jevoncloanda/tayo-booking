package repository

import (
	"context"
	"log"
	"tayo-booking/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type StopRepository struct {
	DB *pgxpool.Pool
}

func NewStopRepository(db *pgxpool.Pool) *StopRepository {
	return &StopRepository{DB: db}
}

func (r *StopRepository) CreateStop(ctx context.Context, stop *models.Stop) error {
	query := `
		insert into stops (id, name, city, created_at)
		values ($1, $2, $3, $4)
	`
	_, err := r.DB.Exec(ctx, query, stop.ID, stop.Name, stop.City, stop.CreatedAt)
	if err != nil {
		log.Println("[CreateStop] error:", err)
	}
	return err
}

func (r *StopRepository) GetAllStops(ctx context.Context) ([]models.Stop, error) {
	query := `select id, name, city, created_at from stops order by city, name`
	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		log.Println("[GetAllStops] error:", err)
		return nil, err
	}
	defer rows.Close()

	var stops []models.Stop
	for rows.Next() {
		var s models.Stop
		if err := rows.Scan(&s.ID, &s.Name, &s.City, &s.CreatedAt); err != nil {
			return nil, err
		}
		stops = append(stops, s)
	}
	return stops, rows.Err()
}

func (r *StopRepository) GetStopByNameAndCity(ctx context.Context, name, city string) (*models.Stop, error) {
	query := `select id, name, city, created_at from stops where name = $1 and city = $2`
	row := r.DB.QueryRow(ctx, query, name, city)
	var s models.Stop
	err := row.Scan(&s.ID, &s.Name, &s.City, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
