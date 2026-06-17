// Infrastructure layer — Angular services, components, routing.
// This layer DEPENDS on domain and application layers, never the reverse.
//
// Contains:
// - API adapters (Supabase client, Ollama client)
// - Angular-specific services
// - UI components and routing configuration
export * from './supabase.adapter';
export * from './ollama.adapter';
export * from './services/challenge.service';
export * from './repositories/mock-challenge.repository';
