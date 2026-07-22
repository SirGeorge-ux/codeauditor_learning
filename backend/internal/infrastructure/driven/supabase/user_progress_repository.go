package supabase

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// Compile-time check: SupabaseProgressRepository implements ports.UserProgressRepository.
var _ ports.UserProgressRepository = (*SupabaseProgressRepository)(nil)

// SupabaseProgressRepository is the Supabase/PostgreSQL adapter for the
// UserProgressRepository port. It owns all SQL and JSONB marshaling concerns.
type SupabaseProgressRepository struct {
	db *sql.DB
}

// NewSupabaseProgressRepository constructs the adapter backed by the given
// database/sql connection.
func NewSupabaseProgressRepository(db *sql.DB) *SupabaseProgressRepository {
	return &SupabaseProgressRepository{db: db}
}

// languageProgressColumns lists the columns returned/inserted by this adapter,
// excluding ultima_actualizacion which the DB sets automatically (or via trigger).
const languageProgressColumns = `user_id, language,
	rango, puntos,
	challenges_completados, challenges_intentados, tasa_exito,
	topics_dominados, completed_challenge_ids,
	rango_code_health, puntos_code_health,
	repos_analizados, issues_encontrados,
	ultimo_completado, ultima_actualizacion`

// scanLanguageProgress maps a SQL row into a models.LanguageProgress.
func scanLanguageProgress(scanner interface{ Scan(dest ...any) error }) (*models.LanguageProgress, error) {
	var (
		p                   models.LanguageProgress
		topicsBytes         []byte
		completedBytes      []byte
		ultimoCompletado    sql.NullTime
		ultimaActualizacion sql.NullTime
	)
	err := scanner.Scan(
		&p.UserID, &p.Language,
		&p.Rango, &p.Puntos,
		&p.ChallengesCompletados, &p.ChallengesIntentados, &p.TasaExito,
		&topicsBytes, &completedBytes,
		&p.RangoCodeHealth, &p.PuntosCodeHealth,
		&p.ReposAnalizados, &p.IssuesEncontrados,
		&ultimoCompletado, &ultimaActualizacion,
	)
	if err != nil {
		return nil, err
	}

	if topicsBytes != nil {
		if err := json.Unmarshal(topicsBytes, &p.TopicsDominados); err != nil {
			return nil, fmt.Errorf("unmarshal topics_dominados: %w", err)
		}
	}
	if p.TopicsDominados == nil {
		p.TopicsDominados = []string{}
	}

	if completedBytes != nil {
		if err := json.Unmarshal(completedBytes, &p.CompletedChallengeIDs); err != nil {
			return nil, fmt.Errorf("unmarshal completed_challenge_ids: %w", err)
		}
	}
	if p.CompletedChallengeIDs == nil {
		p.CompletedChallengeIDs = []string{}
	}

	if ultimoCompletado.Valid {
		t := ultimoCompletado.Time
		p.UltimoCompletado = &t
	}
	if ultimaActualizacion.Valid {
		p.UltimaActualizacion = ultimaActualizacion.Time
	}

	return &p, nil
}

// ErrSentinelNotFound is a sentinel used internally; database/sql.ErrNoRows is
// used directly in code paths below.
var ErrSentinelNotFound = errors.New("not found")

// GetLanguageProgress returns the progress row for (userID, language),
// upserting a default row when the pair does not exist yet.
func (r *SupabaseProgressRepository) GetLanguageProgress(ctx context.Context, userID, language string) (*models.LanguageProgress, error) {
	const query = `SELECT ` + languageProgressColumns + `
		FROM public.user_language_progress
		WHERE user_id = $1 AND language = $2`

	row := r.db.QueryRowContext(ctx, query, userID, language)
	p, err := scanLanguageProgress(row)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get language progress: %w", err)
	}

	// Row missing — upsert a default row and re-read.
	defaulted := defaultLanguageProgress(userID, language)
	if err := r.UpdateLanguageProgress(ctx, defaulted); err != nil {
		return nil, fmt.Errorf("upsert default language progress: %w", err)
	}

	row = r.db.QueryRowContext(ctx, query, userID, language)
	p, err = scanLanguageProgress(row)
	if err != nil {
		return nil, fmt.Errorf("re-read default language progress: %w", err)
	}
	return p, nil
}

// UpdateLanguageProgress upserts the given progress row by (user_id, language).
func (r *SupabaseProgressRepository) UpdateLanguageProgress(ctx context.Context, p *models.LanguageProgress) error {
	topicsJSON, err := json.Marshal(p.TopicsDominados)
	if err != nil {
		return fmt.Errorf("marshal topics_dominados: %w", err)
	}
	completedJSON, err := json.Marshal(p.CompletedChallengeIDs)
	if err != nil {
		return fmt.Errorf("marshal completed_challenge_ids: %w", err)
	}

	const query = `INSERT INTO public.user_language_progress (
		user_id, language,
		rango, puntos,
		challenges_completados, challenges_intentados, tasa_exito,
		topics_dominados, completed_challenge_ids,
		rango_code_health, puntos_code_health,
		repos_analizados, issues_encontrados,
		ultimo_completado, ultima_actualizacion
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,NOW())
	ON CONFLICT (user_id, language) DO UPDATE SET
		rango = EXCLUDED.rango,
		puntos = EXCLUDED.puntos,
		challenges_completados = EXCLUDED.challenges_completados,
		challenges_intentados = EXCLUDED.challenges_intentados,
		tasa_exito = EXCLUDED.tasa_exito,
		topics_dominados = EXCLUDED.topics_dominados,
		completed_challenge_ids = EXCLUDED.completed_challenge_ids,
		rango_code_health = EXCLUDED.rango_code_health,
		puntos_code_health = EXCLUDED.puntos_code_health,
		repos_analizados = EXCLUDED.repos_analizados,
		issues_encontrados = EXCLUDED.issues_encontrados,
		ultimo_completado = EXCLUDED.ultimo_completado,
		ultima_actualizacion = NOW()`

	var ultimoCompletado any
	if p.UltimoCompletado != nil {
		ultimoCompletado = *p.UltimoCompletado
	} else {
		ultimoCompletado = nil
	}

	_, err = r.db.ExecContext(
		ctx, query,
		p.UserID, p.Language,
		p.Rango, p.Puntos,
		p.ChallengesCompletados, p.ChallengesIntentados, p.TasaExito,
		topicsJSON, completedJSON,
		p.RangoCodeHealth, p.PuntosCodeHealth,
		p.ReposAnalizados, p.IssuesEncontrados,
		ultimoCompletado,
	)
	if err != nil {
		return fmt.Errorf("upsert language progress: %w", err)
	}
	return nil
}

// GetAllLanguageProgress returns all language progress rows for the given user.
func (r *SupabaseProgressRepository) GetAllLanguageProgress(ctx context.Context, userID string) ([]*models.LanguageProgress, error) {
	const query = `SELECT ` + languageProgressColumns + `
		FROM public.user_language_progress
		WHERE user_id = $1
		ORDER BY language ASC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get all language progress: %w", err)
	}
	defer rows.Close()

	var out []*models.LanguageProgress
	for rows.Next() {
		p, err := scanLanguageProgress(rows)
		if err != nil {
			return nil, fmt.Errorf("scan language progress: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate language progress rows: %w", err)
	}
	return out, nil
}

// GetLearningProfile returns the learning profile for userID, creating a
// default one if none exists.
func (r *SupabaseProgressRepository) GetLearningProfile(ctx context.Context, userID string) (*models.LearningProfile, error) {
	const query = `SELECT user_id, preferencias, estilo_aprendizaje, ultima_actualizacion
		FROM public.user_learning_profile
		WHERE user_id = $1`

	var (
		userIDOut           string
		preferenciasBytes   []byte
		estiloBytes         []byte
		ultimaActualizacion sql.NullTime
	)
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&userIDOut, &preferenciasBytes, &estiloBytes, &ultimaActualizacion,
	)
	if err == nil {
		p := models.DefaultLearningProfile(userID)
		if preferenciasBytes != nil {
			if err := json.Unmarshal(preferenciasBytes, &p.Preferencias); err != nil {
				return nil, fmt.Errorf("unmarshal preferencias: %w", err)
			}
		}
		if estiloBytes != nil {
			if err := json.Unmarshal(estiloBytes, &p.EstiloAprendizaje); err != nil {
				return nil, fmt.Errorf("unmarshal estilo_aprendizaje: %w", err)
			}
		}
		if ultimaActualizacion.Valid {
			p.UltimaActualizacion = ultimaActualizacion.Time
		}
		return p, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get learning profile: %w", err)
	}

	// Row missing — upsert default.
	defaulted := models.DefaultLearningProfile(userID)
	if err := r.UpdateLearningProfile(ctx, defaulted); err != nil {
		return nil, fmt.Errorf("upsert default learning profile: %w", err)
	}
	return defaulted, nil
}

// UpdateLearningProfile upserts the given learning profile by user_id.
func (r *SupabaseProgressRepository) UpdateLearningProfile(ctx context.Context, p *models.LearningProfile) error {
	preferenciasJSON, err := json.Marshal(p.Preferencias)
	if err != nil {
		return fmt.Errorf("marshal preferencias: %w", err)
	}
	estiloJSON, err := json.Marshal(p.EstiloAprendizaje)
	if err != nil {
		return fmt.Errorf("marshal estilo_aprendizaje: %w", err)
	}

	const query = `INSERT INTO public.user_learning_profile (user_id, preferencias, estilo_aprendizaje, ultima_actualizacion)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			preferencias = EXCLUDED.preferencias,
			estilo_aprendizaje = EXCLUDED.estilo_aprendizaje,
			ultima_actualizacion = NOW()`

	_, err = r.db.ExecContext(ctx, query, p.UserID, preferenciasJSON, estiloJSON)
	if err != nil {
		return fmt.Errorf("upsert learning profile: %w", err)
	}
	return nil
}

// defaultLanguageProgress builds an in-memory default LanguageProgress row that
// matches the DB defaults (rango="F", rangoCodeHealth="Junior", puntos=0).
func defaultLanguageProgress(userID, language string) *models.LanguageProgress {
	return &models.LanguageProgress{
		UserID:                userID,
		Language:              language,
		Rango:                 "F",
		Puntos:                0,
		ChallengesCompletados: 0,
		ChallengesIntentados:  0,
		TasaExito:             0,
		TopicsDominados:       []string{},
		CompletedChallengeIDs: []string{},
		RangoCodeHealth:       "Junior",
		PuntosCodeHealth:      0,
		ReposAnalizados:       0,
		IssuesEncontrados:     0,
	}
}
