// LearningProfile — per-user persistent learning preferences consumed by the
// socratic tutor chat.
//
// Matches the Go struct in backend/internal/core/domain/models/learning_profile.go.
// All JSON keys are camelCase (Go json tags already use camelCase).
// Zero framework imports. Pure TypeScript domain model.
export interface Preferencias {
  idioma: string; // es | en | fr | de
  nivelSocratismo: number; // 0-3
  longitudMaximaMsg: number; // max chars per tutor message
  bloqueosTipicos: string[];
  topicsQueLeCuestan: string[];
  resumen: string;
}

export interface EstiloAprendizaje {
  aprendeMejorCon: string; // ejemplos | teoria | practica | ...
}

export interface LearningProfile {
  userId: string;
  preferencias: Preferencias;
  estiloAprendizaje: EstiloAprendizaje;
  ultimaActualizacion: string; // ISO 8601
}