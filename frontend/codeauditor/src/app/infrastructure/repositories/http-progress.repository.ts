// HttpProgressRepository — infrastructure adapter that fetches language
// progress and learning profile data from the backend API.
//
// Uses native fetch with manual Authorization header (consistent with
// HttpChallengeRepository pattern). Implements ProgressRepository port.
//
// The backend serializes the LanguageProgress and LearningProfile structs
// with camelCase JSON keys (Go json tags already use camelCase). This
// adapter maps the raw API responses to the TS domain interfaces,
// defaulting missing/null arrays to [].
//
// API endpoints consumed:
//   GET  /api/v1/users/{id}/progress           → AllProgressResponse
//   GET  /api/v1/users/{id}/progress/{lang}     → LanguageProgress
//   GET  /api/v1/users/{id}/learning-profile    → LearningProfile
//   PUT  /api/v1/users/{id}/learning-profile    → LearningProfile (partial update)
import { LanguageProgress, AllProgressResponse } from '../../domain/models/language-progress';
import {
  LearningProfile,
  Preferencias,
  EstiloAprendizaje,
} from '../../domain/models/learning-profile';
import { ProgressRepository } from '../../domain/ports/progress-repository.port';

export interface TokenProvider {
  getToken(): string | null;
  getUserId(): string | null;
}

export class HttpProgressRepository implements ProgressRepository {
  private readonly baseUrl: string;
  private readonly tokenProvider: TokenProvider;

  constructor(tokenProvider: TokenProvider, baseUrl: string = '/api/v1') {
    this.baseUrl = baseUrl;
    this.tokenProvider = tokenProvider;
  }

  private userPath(): string {
    const id = this.tokenProvider.getUserId();
    return id ? `/users/${encodeURIComponent(id)}` : '/users/me';
  }

  async getAllLanguageProgress(): Promise<AllProgressResponse> {
    try {
      const token = this.tokenProvider.getToken();
      if (!token) return { rangoGlobal: 'F', languages: [] };

      const resp = await fetch(`${this.baseUrl}${this.userPath()}/progress`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!resp.ok) return { rangoGlobal: 'F', languages: [] };
      const data = await resp.json();
      return mapAllProgressResponse(data);
    } catch {
      return { rangoGlobal: 'F', languages: [] };
    }
  }

  async getLanguageProgress(lang: string): Promise<LanguageProgress | null> {
    try {
      const token = this.tokenProvider.getToken();
      if (!token) return null;

      const resp = await fetch(
        `${this.baseUrl}${this.userPath()}/progress/${encodeURIComponent(lang)}`,
        { headers: { Authorization: `Bearer ${token}` } },
      );
      if (!resp.ok) return null;
      const data = await resp.json();
      return mapLanguageProgress(data);
    } catch {
      return null;
    }
  }

  async getLearningProfile(): Promise<LearningProfile | null> {
    try {
      const token = this.tokenProvider.getToken();
      if (!token) return null;

      const resp = await fetch(`${this.baseUrl}${this.userPath()}/learning-profile`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!resp.ok) return null;
      const data = await resp.json();
      return mapLearningProfile(data);
    } catch {
      return null;
    }
  }

  async updateLearningProfile(
    preferencias?: Partial<Preferencias>,
    estiloAprendizaje?: Partial<EstiloAprendizaje>,
  ): Promise<LearningProfile | null> {
    try {
      const token = this.tokenProvider.getToken();
      if (!token) return null;

      const body: Record<string, unknown> = {};
      if (preferencias) body['preferencias'] = preferencias;
      if (estiloAprendizaje) body['estiloAprendizaje'] = estiloAprendizaje;

      const resp = await fetch(`${this.baseUrl}${this.userPath()}/learning-profile`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(body),
      });
      if (!resp.ok) return null;
      const data = await resp.json();
      return mapLearningProfile(data);
    } catch {
      return null;
    }
  }
}

// ─── Mappers ───────────────────────────────────────────────────────────────

function mapAllProgressResponse(obj: Record<string, unknown>): AllProgressResponse {
  const languages = Array.isArray(obj['languages']) ? obj['languages'] : [];
  return {
    rangoGlobal: typeof obj['rangoGlobal'] === 'string' ? obj['rangoGlobal'] : 'F',
    languages: languages.map((raw) => mapLanguageProgress(raw as Record<string, unknown>)),
  };
}

function mapLanguageProgress(obj: Record<string, unknown>): LanguageProgress {
  return {
    userId: String(obj['userId'] ?? ''),
    language: String(obj['language'] ?? ''),
    rango: String(obj['rango'] ?? 'F'),
    puntos: Number(obj['puntos'] ?? 0),
    challengesCompletados: Number(obj['challengesCompletados'] ?? 0),
    challengesIntentados: Number(obj['challengesIntentados'] ?? 0),
    tasaExito: Number(obj['tasaExito'] ?? 0),
    topicsDominados: Array.isArray(obj['topicsDominados'])
      ? obj['topicsDominados'].map((x: unknown) => String(x))
      : [],
    rangoCodeHealth: String(obj['rangoCodeHealth'] ?? 'Junior'),
    puntosCodeHealth: Number(obj['puntosCodeHealth'] ?? 0),
    reposAnalizados: Number(obj['reposAnalizados'] ?? 0),
    issuesEncontrados: Number(obj['issuesEncontrados'] ?? 0),
    ultimoCompletado:
      obj['ultimoCompletado'] === null
        ? null
        : typeof obj['ultimoCompletado'] === 'string'
          ? obj['ultimoCompletado']
          : undefined,
    ultimaActualizacion: String(obj['ultimaActualizacion'] ?? ''),
  };
}

function mapLearningProfile(obj: Record<string, unknown>): LearningProfile {
  const pref = (obj['preferencias'] as Record<string, unknown>) ?? {};
  const estilo = (obj['estiloAprendizaje'] as Record<string, unknown>) ?? {};
  return {
    userId: String(obj['userId'] ?? ''),
    preferencias: {
      idioma: String(pref['idioma'] ?? 'es'),
      nivelSocratismo: Number(pref['nivelSocratismo'] ?? 2),
      longitudMaximaMsg: Number(pref['longitudMaximaMsg'] ?? 200),
      bloqueosTipicos: Array.isArray(pref['bloqueosTipicos'])
        ? pref['bloqueosTipicos'].map((x: unknown) => String(x))
        : [],
      topicsQueLeCuestan: Array.isArray(pref['topicsQueLeCuestan'])
        ? pref['topicsQueLeCuestan'].map((x: unknown) => String(x))
        : [],
      resumen: String(pref['resumen'] ?? ''),
    },
    estiloAprendizaje: {
      aprendeMejorCon: String(estilo['aprendeMejorCon'] ?? 'ejemplos'),
    },
    ultimaActualizacion: String(obj['ultimaActualizacion'] ?? ''),
  };
}