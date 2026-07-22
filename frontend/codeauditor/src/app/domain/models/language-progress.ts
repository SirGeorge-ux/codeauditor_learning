// LanguageProgress — tracks per-language mastery for a user.
//
// Two range systems coexist:
//   - rango / puntos: F-S ranges for free practice mode
//   - rangoCodeHealth / puntosCodeHealth: Junior-Architect for code health mode
//
// Matches the Go struct in backend/internal/core/domain/models/language_progress.go.
// All JSON keys are camelCase (Go json tags already use camelCase).
// Zero framework imports. Pure TypeScript domain model.
export interface LanguageProgress {
  userId: string;
  language: string;
  rango: string; // F-S
  puntos: number;
  challengesCompletados: number;
  challengesIntentados: number;
  tasaExito: number; // 0-100
  topicsDominados: string[];
  rangoCodeHealth: string; // Junior-Mid-Senior-Architect
  puntosCodeHealth: number;
  reposAnalizados: number;
  issuesEncontrados: number;
  ultimoCompletado?: string | null; // ISO 8601 or null
  ultimaActualizacion: string; // ISO 8601
}

// Response shape for GET /api/v1/users/{id}/progress.
export interface AllProgressResponse {
  rangoGlobal: string;
  languages: LanguageProgress[];
}