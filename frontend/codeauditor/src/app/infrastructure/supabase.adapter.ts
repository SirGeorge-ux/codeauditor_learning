// Supabase adapter — implements domain ports using Supabase client.
// This is infrastructure, not domain, so it can import Angular and RxJS.
import { createClient, SupabaseClient } from "@supabase/supabase-js";
import { AuditRepository } from "../domain/ports/audit-repository.port";
import { AuthPort } from "../domain/ports/auth.port";
import { User } from "../domain/models/user";
import { AuditSession } from "../domain/models/audit-session";
import { Finding } from "../domain/models/finding";

/**
 * SupabaseAuditRepository — implements AuditRepository using Supabase.
 * Supabase adapter is infrastructure (driven/secondary) — depends on domain ports.
 */
export class SupabaseAuditRepository implements AuditRepository {
  private client: SupabaseClient;

  constructor(supabaseUrl: string, supabaseKey: string) {
    this.client = createClient(supabaseUrl, supabaseKey);
  }

  async createSession(
    session: Omit<AuditSession, "id" | "createdAt" | "updatedAt">
  ): Promise<AuditSession> {
    // Stub — real implementation will insert into Supabase
    return {
      ...session,
      id: "stub-id-" + Date.now(),
      createdAt: new Date(),
      updatedAt: new Date(),
    } as AuditSession;
  }

  async getSession(_id: string): Promise<AuditSession | null> {
    return null; // Stub
  }

  async listSessions(): Promise<AuditSession[]> {
    return []; // Stub
  }

  async updateSessionStatus(_id: string, _status: AuditSession["status"]): Promise<void> {
    // Stub
  }

  async addFinding(finding: Omit<Finding, "id" | "detectedAt">): Promise<Finding> {
    return { ...finding, id: "stub", detectedAt: new Date() } as Finding;
  }

  async getFindingsForSession(_sessionId: string): Promise<Finding[]> {
    return []; // Stub
  }
}

/**
 * SupabaseAuthAdapter — implements AuthPort using Supabase Auth.
 */
export class SupabaseAuthAdapter implements AuthPort {
  private client: SupabaseClient;

  constructor(supabaseUrl: string, supabaseKey: string) {
    this.client = createClient(supabaseUrl, supabaseKey);
  }

  async getCurrentUser() {
    const { data } = await this.client.auth.getUser();
    if (!data.user) return null;
    return {
      id: data.user.id,
      email: data.user.email ?? "",
      createdAt: new Date(data.user.created_at),
    };
  }

  async signIn(email: string, password: string) {
    const { data } = await this.client.auth.signInWithPassword({ email, password });
    if (!data.user) throw new Error("Sign-in failed");
    return {
      id: data.user.id,
      email: data.user.email ?? "",
      createdAt: new Date(data.user.created_at),
    };
  }

  async signOut() {
    await this.client.auth.signOut();
  }

  onAuthStateChange(callback: (user: User | null) => void) {
    const { data } = this.client.auth.onAuthStateChange((_event, session) => {
      callback(session?.user ? { id: session.user.id, email: session.user.email ?? '', createdAt: new Date(session.user.created_at) } : null);
    });
    return () => data.subscription.unsubscribe();
  }
}