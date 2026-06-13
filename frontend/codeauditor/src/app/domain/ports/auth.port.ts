import { User } from "../models/user";

/**
 * AuthPort — interface for authentication and user identity.
 *
 * The application layer calls this to verify identity and access user context.
 * The infrastructure layer provides the Supabase implementation.
 */
export interface AuthPort {
  getCurrentUser(): Promise<User | null>;
  signIn(email: string, password: string): Promise<User>;
  signOut(): Promise<void>;
  onAuthStateChange(callback: (user: User | null) => void): () => void;
}