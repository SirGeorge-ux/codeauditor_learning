package models

import "time"

// AuthResponse represents the authentication response from Supabase.
type AuthResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	User         UserData `json:"user"`
}

// UserData represents user data from Supabase auth.
type UserData struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// UserProfile represents the user profile from public.usuarios.
type UserProfile struct {
	ID             string     `json:"id"`
	Email          string     `json:"email"`
	DisplayName    string     `json:"display_name,omitempty"`
	RachaDias      int        `json:"racha_dias"`
	PuntosMaestria int        `json:"puntos_maestria"`
	RangoActual    string     `json:"rango_actual"`
	UltimoIntento  *time.Time `json:"ultimo_intento_valido,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
