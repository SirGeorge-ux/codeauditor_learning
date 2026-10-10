package models

import (
	"encoding/json"
	"testing"
	"time"
)

// TestSocraticLevel_Constants cubre la base: 4 constantes con los valores
// 0..3 que el spec del tutor exige, mas el predicado Valid().
//
// Mientras el spec / design llama a los niveles "0..3", el codigo Go usa
// constantes con nombre para que los call-sites documenten la eleccion.
// Si una constante futura cambia su valor numeric, esta prueba rompe
// primero y obliga a actualizar los adapters que asumen [0..3].
func TestSocraticLevel_Constants(t *testing.T) {
	cases := []struct {
		name string
		got  SocraticLevel
		want int
	}{
		{"Direct", SocraticDirect, 0},
		{"Guided", SocraticGuided, 1},
		{"Pure", SocraticPure, 2},
		{"Discovery", SocraticDiscovery, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if int(c.got) != c.want {
				t.Errorf("%s = %d, want %d", c.name, int(c.got), c.want)
			}
		})
	}
}

// TestSocraticLevel_Valid_Boundaries triangula el predicate Valid() con
// cada valor canonico, cada valor fuera de rango, y el valor zero.
// El predicado DEBE aceptar exactamente 0..3; cualquier valor fuera del
// rango es rechazable silenciosamente por el auto-escalation state
// machine (PR 2).
func TestSocraticLevel_Valid_Boundaries(t *testing.T) {
	cases := []struct {
		name  string
		level SocraticLevel
		want  bool
	}{
		{"direct 0", SocraticDirect, true},
		{"guided 1", SocraticGuided, true},
		{"pure 2", SocraticPure, true},
		{"discovery 3", SocraticDiscovery, true},
		{"zero value", SocraticLevel(0), true},
		{"negative", SocraticLevel(-1), false},
		{"above range", SocraticLevel(4), false},
		{"absurd", SocraticLevel(999), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.level.Valid(); got != c.want {
				t.Errorf("Valid(%d) = %v, want %v", int(c.level), got, c.want)
			}
		})
	}
}

// TestSocraticLevel_String verifica que cada constante tiene una
// etiqueta canonica estable (usada para logs y el event `status:
// escalated`). Si alguien agrega un nivel extra en el futuro, la
// default branch DEBE ser un valor seguro que no se confunda con
// una de las 4 etiquetas conocidas.
func TestSocraticLevel_String(t *testing.T) {
	cases := []struct {
		level SocraticLevel
		want  string
	}{
		{SocraticDirect, "direct"},
		{SocraticGuided, "guided"},
		{SocraticPure, "socratic"},
		{SocraticDiscovery, "discovery"},
		{SocraticLevel(7), "unknown"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			if got := c.level.String(); got != c.want {
				t.Errorf("String(%d) = %q, want %q", int(c.level), got, c.want)
			}
		})
	}
}

// TestSocraticLevel_JSONMarshal confirma el contrato de serializacion:
// el level se serializa como entero (alineado con el JSON tag de
// Preferencias.NivelSocratismo que ya existe en learning_profile.go).
// Si el contrato cambia a string en el futuro, esta prueba rompe.
func TestSocraticLevel_JSONMarshal(t *testing.T) {
	out, err := json.Marshal(SocraticDiscovery)
	if err != nil {
		t.Fatalf("Marshal(SocraticDiscovery) error: %v", err)
	}
	if string(out) != "3" {
		t.Errorf("Marshal(SocraticDiscovery) = %s, want %q", out, "3")
	}
}

// TestChatRole_Constants verifica que los 3 roles canónicos del chat
// son los unicos literales que un ChatMessage.Role puede tener
// (validado por el MCP tool layer en update_learning_profile).
func TestChatRole_Constants(t *testing.T) {
	cases := []struct {
		got  ChatRole
		want string
	}{
		{ChatRoleUser, "user"},
		{ChatRoleTutor, "tutor"},
		{ChatRoleSystem, "system"},
	}
	for _, c := range cases {
		t.Run(string(c.got), func(t *testing.T) {
			if string(c.got) != c.want {
				t.Errorf("ChatRole = %q, want %q", c.got, c.want)
			}
		})
	}
}

// TestChatMessage_JSONRoundtrip verifica el shape JSON que el frontend
// Angular consume y que el SSE streamer serializa. Si un JSON tag
// cambia (eg. timestamp), el frontend deja de parsear — esta prueba
// rompe en Go en lugar de en runtime del browser.
func TestChatMessage_JSONRoundtrip(t *testing.T) {
	ts := time.Date(2026, 10, 10, 12, 30, 0, 0, time.UTC)
	level := SocraticPure
	original := ChatMessage{
		Role:      ChatRoleTutor,
		Content:   "¿Qué te dice esta línea sobre el control de entrada?",
		Level:     &level,
		Timestamp: ts,
	}
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var decoded ChatMessage
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.Role != ChatRoleTutor {
		t.Errorf("Role round-trip = %q, want %q", decoded.Role, ChatRoleTutor)
	}
	if decoded.Content != original.Content {
		t.Errorf("Content round-trip = %q, want %q", decoded.Content, original.Content)
	}
	if decoded.Level == nil || *decoded.Level != SocraticPure {
		t.Errorf("Level round-trip = %v, want pointer to Pure (2)", decoded.Level)
	}
	if !decoded.Timestamp.Equal(ts) {
		t.Errorf("Timestamp round-trip = %v, want %v", decoded.Timestamp, ts)
	}
}

// TestChatMessage_OmitsLevelWhenZero confirma el contrato opcional del
// campo Level: solo mensajes del tutor lo setean, mensajes del user no.
// Un JSON que incluye `"level": 0` en mensajes user es informacion
// redundante que contamina el LLM prompt.
func TestChatMessage_OmitsLevelWhenZero(t *testing.T) {
	msg := ChatMessage{
		Role:      ChatRoleUser,
		Content:   "no entiendo la línea 12",
		Timestamp: time.Now(),
	}
	encoded, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	// Decode generico para chequear presencia del campo.
	var generic map[string]any
	if err := json.Unmarshal(encoded, &generic); err != nil {
		t.Fatalf("Unmarshal into map error: %v", err)
	}
	if _, present := generic["level"]; present {
		t.Errorf("level field should be omitted when nil, got %s", encoded)
	}
}

// TestChatContext_ZeroValueIsPublishable verifica que el zero value de
// ChatContext no produce JSON con campos nulos que rompan al LLM (en
// particular, que Challenge sea un *pointer que pueda ser nil sin
// panic en MarshalJSON).
func TestChatContext_ZeroValueIsPublishable(t *testing.T) {
	var ctx ChatContext // zero value
	encoded, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("Marshal zero ChatContext error: %v", err)
	}
	// Validar que el output sea JSON valido y contega las claves esperadas.
	var generic map[string]any
	if err := json.Unmarshal(encoded, &generic); err != nil {
		t.Fatalf("Unmarshal back into map error: %v", err)
	}
	for _, required := range []string{"user", "language", "location", "history", "preferences"} {
		if _, ok := generic[required]; !ok {
			t.Errorf("zero ChatContext JSON missing required key %q", required)
		}
	}
}

// TestChatContext_FullPayloadRoundtrip verifica que un ChatContext
// completamente poblado sobrevive un Marshal/Unmarshal sin perder
// informacion. Esto es el contrato que el backend y el frontend usan
// para reconstruir el estado cuando el SSE streamer manda el
// contexto al cliente (PR 2 feature).
func TestChatContext_FullPayloadRoundtrip(t *testing.T) {
	level := SocraticGuided
	original := ChatContext{
		User: ChatUserContext{
			UserID:     "ggogsmic",
			GlobalRank: "C",
			StreakDays: 7,
		},
		Language: ChatLanguageContext{
			Language:    "typescript",
			Rank:        "C",
			Points:      450,
			SuccessRate: 0.62,
		},
		Location: ChatLocationContext{
			Route: "/dojo/ch-sqli",
			Challenge: &ChallengeLocationState{
				ChallengeID:   "ch-sqli",
				Code:          "app.get('/u/:id', (req) => db.query('SELECT * FROM u WHERE id = ' + req.params.id))",
				SandboxOutput: "ok",
			},
		},
		History: ChatHistoryContext{
			RecentHints:     []string{"first hint", "second hint"},
			TypicalBlockers: []string{"OFF-BY-ONE", "mutable default arg"},
		},
		Preferences: ChatPreferencesContext{
			Language:         "es",
			SocraticLevel:    level,
			MaxMessageLength: 200,
		},
	}
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var decoded ChatContext
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.User.UserID != original.User.UserID {
		t.Errorf("User.UserID round-trip = %q, want %q", decoded.User.UserID, original.User.UserID)
	}
	if decoded.Language.Points != original.Language.Points {
		t.Errorf("Language.Points round-trip = %d, want %d", decoded.Language.Points, original.Language.Points)
	}
	if decoded.Location.Challenge == nil || decoded.Location.Challenge.ChallengeID != "ch-sqli" {
		t.Errorf("Location.Challenge.ChallengeID round-trip failed: got %+v", decoded.Location.Challenge)
	}
	if decoded.Preferences.SocraticLevel != SocraticGuided {
		t.Errorf("Preferences.SocraticLevel round-trip = %d, want %d", decoded.Preferences.SocraticLevel, SocraticGuided)
	}
	if !decoded.Preferences.SocraticLevel.Valid() {
		t.Errorf("decoded SocraticLevel failed Valid() predicate")
	}
}
