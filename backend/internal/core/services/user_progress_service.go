package services

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// UserProgressService manages user streaks, mastery points, and ranking.
type UserProgressService struct {
	db *sql.DB
}

// Rank thresholds
const (
	RankJunior    = "Junior"
	RankMid       = "Mid"
	RankSenior    = "Senior"
	RankArchitect = "Architect"
)

// RankThresholds maps minimum points to rank.
var RankThresholds = []struct {
	MinPoints int
	Rank      string
}{
	{750, RankArchitect},
	{300, RankSenior},
	{100, RankMid},
	{0, RankJunior},
}

// NewUserProgressService creates a new UserProgressService.
func NewUserProgressService(db *sql.DB) *UserProgressService {
	return &UserProgressService{db: db}
}

// RecordAuditAttempt updates user progress after a successful audit.
// It increments mastery points (+10), updates the daily streak,
// and recalculates rank based on total points.
func (s *UserProgressService) RecordAuditAttempt(ctx context.Context, userID string) error {
	now := time.Now().UTC()

	// Fetch current state
	var (
		rachaDias      int
		puntosMaestria int
		ultimoIntento  sql.NullTime
	)

	err := s.db.QueryRowContext(
		ctx,
		`SELECT racha_dias, puntos_maestria, ultimo_intento_valido
		 FROM public.usuarios WHERE id = $1`, userID,
	).Scan(&rachaDias, &puntosMaestria, &ultimoIntento)

	if err == sql.ErrNoRows {
		log.Printf("User %s not found in usuarios table", userID)
		return nil
	}
	if err != nil {
		return err
	}

	// Streak logic: +1 if last attempt was yesterday or today, reset if gap > 1 day
	newRacha := 1
	if ultimoIntento.Valid {
		lastY, lastM, lastD := ultimoIntento.Time.UTC().Date()
		todayY, todayM, todayD := now.Date()
		if lastY == todayY && lastM == todayM {
			if lastD == todayD {
				newRacha = rachaDias // same day, don't increment
			} else if lastD == todayD-1 {
				newRacha = rachaDias + 1 // consecutive day
			}
		} else if lastY == todayY && lastM == todayM-1 && todayD == 1 {
			// Month boundary: last day of previous month → first day of current month
			lastDayOfMonth := time.Date(lastY, lastM+1, 0, 0, 0, 0, 0, time.UTC).Day()
			if lastD == lastDayOfMonth {
				newRacha = rachaDias + 1
			}
		} else if lastY == todayY-1 && lastM == 12 && todayM == 1 && lastD == 31 && todayD == 1 {
			// Year boundary
			newRacha = rachaDias + 1
		}
	}

	// Mastery points: +10 per audit
	newPuntos := puntosMaestria + 10

	// Rank calculation
	newRango := calculateRank(newPuntos)

	_, err = s.db.ExecContext(
		ctx,
		`UPDATE public.usuarios
		 SET racha_dias = $1, puntos_maestria = $2, rango_actual = $3,
		     ultimo_intento_valido = $4, updated_at = NOW()
		 WHERE id = $5`,
		newRacha, newPuntos, newRango, now, userID,
	)
	if err != nil {
		return err
	}

	log.Printf("User %s progress: streak=%d, points=%d, rank=%s", userID, newRacha, newPuntos, newRango)
	return nil
}

// calculateRank returns the rank based on total mastery points.
func calculateRank(points int) string {
	for _, t := range RankThresholds {
		if points >= t.MinPoints {
			return t.Rank
		}
	}
	return RankJunior
}
