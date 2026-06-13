import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock Supabase before importing the service
const mockSignUp = vi.fn();
const mockSignInWithPassword = vi.fn();
const mockSignOut = vi.fn();
const mockGetSession = vi.fn();
const mockGetUser = vi.fn();
const mockOnAuthStateChange = vi.fn();

vi.mock('@supabase/supabase-js', () => ({
  createClient: () => ({
    auth: {
      signUp: mockSignUp,
      signInWithPassword: mockSignInWithPassword,
      signOut: mockSignOut,
      getSession: mockGetSession,
      getUser: mockGetUser,
      onAuthStateChange: mockOnAuthStateChange,
    },
  }),
}));

import { AuthService } from './auth.service';

describe('AuthService', () => {
  let service: AuthService;

  beforeEach(() => {
    vi.clearAllMocks();
    // Default: no session, return null
    mockGetSession.mockResolvedValue({ data: { session: null } });
    mockOnAuthStateChange.mockReturnValue({
      data: { subscription: { unsubscribe: vi.fn() } },
    });
    service = new AuthService();
  });

  it('should create the service', () => {
    expect(service).toBeTruthy();
  });

  it('should start with no user', () => {
    expect(service.userSignal()).toBeNull();
  });

  it('should start unauthenticated', () => {
    expect(service.isAuthenticatedSignal()).toBe(false);
  });

  it('should return null access token when no session', () => {
    expect(service.getAccessToken()).toBeNull();
  });

  describe('register', () => {
    it('should call supabase signUp', async () => {
      mockSignUp.mockResolvedValue({ error: null });
      const result = await service.register('test@test.com', 'password');
      expect(mockSignUp).toHaveBeenCalledWith({
        email: 'test@test.com',
        password: 'password',
      });
      expect(result.error).toBeNull();
    });

    it('should return error on signUp failure', async () => {
      const err = new Error('Email taken');
      mockSignUp.mockResolvedValue({ error: err });
      const result = await service.register('test@test.com', 'password');
      expect(result.error).toBe(err);
    });
  });

  describe('login', () => {
    it('should call supabase signInWithPassword', async () => {
      mockSignInWithPassword.mockResolvedValue({
        data: { session: null },
        error: null,
      });
      const result = await service.login('test@test.com', 'password');
      expect(mockSignInWithPassword).toHaveBeenCalledWith({
        email: 'test@test.com',
        password: 'password',
      });
      expect(result.error).toBeNull();
    });

    it('should return error on login failure', async () => {
      const err = new Error('Invalid credentials');
      mockSignInWithPassword.mockResolvedValue({
        data: { session: null },
        error: err,
      });
      const result = await service.login('test@test.com', 'wrong');
      expect(result.error).toBe(err);
    });
  });

  describe('logout', () => {
    it('should call supabase signOut and clear signals', async () => {
      mockSignOut.mockResolvedValue(undefined);
      await service.logout();
      expect(mockSignOut).toHaveBeenCalled();
      expect(service.sessionSignal()).toBeNull();
      expect(service.userSignal()).toBeNull();
    });
  });

  describe('getCurrentUser', () => {
    it('should call supabase getUser', async () => {
      const mockUser = { id: 'u1', email: 'a@b.com' };
      mockGetUser.mockResolvedValue({ data: { user: mockUser } });
      const user = await service.getCurrentUser();
      expect(user).toBe(mockUser);
    });
  });
});
