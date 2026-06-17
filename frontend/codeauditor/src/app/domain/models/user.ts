// User — represents the authenticated user identity.
//
// In this scaffold, user identity is managed by Supabase Auth.
// The domain model does not depend on Supabase directly.
export interface User {
  id: string;
  email: string;
  displayName?: string;
  avatarUrl?: string;
  createdAt: Date;
}
