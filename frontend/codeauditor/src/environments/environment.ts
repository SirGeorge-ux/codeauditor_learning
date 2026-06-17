// Local development environment — all values target local Docker Compose services.
// This is the Supabase local demo anon key (public, dev-only).
export const environment = {
  production: false,
  supabaseUrl: 'http://localhost:8000',
  supabaseAnonKey: 'local-dev-demo-key-replace-in-prod',
  apiUrl: 'http://localhost:8080',
};
