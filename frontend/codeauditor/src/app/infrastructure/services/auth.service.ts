import { Injectable, signal, computed } from '@angular/core';
import { createClient, SupabaseClient, Session, User } from '@supabase/supabase-js';
import { environment } from '../../../environments/environment';

export interface UserProfile {
  id: string;
  email: string;
  display_name?: string;
  racha_dias: number;
  puntos_maestria: number;
  rango_actual: string;
  ultimo_intento_valido?: string;
  created_at: string;
  updated_at: string;
}

@Injectable({ providedIn: 'root' })
export class AuthService {
  private supabase: SupabaseClient = createClient(
    environment.supabaseUrl,
    environment.supabaseAnonKey,
  );

  userSignal = signal<UserProfile | null>(null);
  sessionSignal = signal<Session | null>(null);
  isAuthenticatedSignal = computed(() => !!this.sessionSignal());

  constructor() {
    this.initSession();
    this.listenToAuthChanges();
  }

  private initSession(): void {
    this.supabase.auth.getSession().then(({ data: { session } }) => {
      this.sessionSignal.set(session);
      if (session) {
        this.fetchProfile();
      }
    });
  }

  private listenToAuthChanges(): void {
    this.supabase.auth.onAuthStateChange((_event, session) => {
      this.sessionSignal.set(session);
      if (session) {
        this.fetchProfile();
      } else {
        this.userSignal.set(null);
      }
    });
  }

  async register(email: string, password: string): Promise<{ error: Error | null }> {
    const { error } = await this.supabase.auth.signUp({ email, password });
    return { error };
  }

  async login(email: string, password: string): Promise<{ error: Error | null }> {
    const { data, error } = await this.supabase.auth.signInWithPassword({ email, password });
    if (error) {
      return { error };
    }
    if (data.session) {
      this.sessionSignal.set(data.session);
      await this.fetchProfile();
    }
    return { error: null };
  }

  async logout(): Promise<void> {
    await this.supabase.auth.signOut();
    this.sessionSignal.set(null);
    this.userSignal.set(null);
  }

  async fetchProfile(): Promise<void> {
    const session = this.sessionSignal();
    if (!session) {
      return;
    }

    try {
      const response = await fetch(`${environment.apiUrl}/auth/me`, {
        headers: {
          Authorization: `Bearer ${session.access_token}`,
          'Content-Type': 'application/json',
        },
      });

      if (response.ok) {
        const profile = await response.json();
        this.userSignal.set(profile);
      }
    } catch (error) {
      console.error('Failed to fetch profile:', error);
    }
  }

  getAccessToken(): string | null {
    return this.sessionSignal()?.access_token ?? null;
  }

  async getCurrentUser(): Promise<User | null> {
    const { data } = await this.supabase.auth.getUser();
    return data.user;
  }
}
