// ProgressRepository — port interface for language progress and learning
// profile data access.
//
// Implemented by the infrastructure layer (HTTP adapter).
// Zero framework imports. Pure TypeScript interface.
import { LanguageProgress, AllProgressResponse } from '../models/language-progress';
import { LearningProfile, Preferencias, EstiloAprendizaje } from '../models/learning-profile';

export interface ProgressRepository {
  getAllLanguageProgress(): Promise<AllProgressResponse>;
  getLanguageProgress(lang: string): Promise<LanguageProgress | null>;
  getLearningProfile(): Promise<LearningProfile | null>;
  updateLearningProfile(
    preferencias?: Partial<Preferencias>,
    estiloAprendizaje?: Partial<EstiloAprendizaje>,
  ): Promise<LearningProfile | null>;
}