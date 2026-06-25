import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { AuthState, User } from '@/shared/types/auth';

// Extend AuthState to include setLoading
type AuthStoreState = AuthState & {
  setLoading: (loading: boolean) => void;
};

export const useAuthStore = create<AuthStoreState>()(
  persist(
    (set) => ({
      user: null,
      accessToken: null,       // In-memory ONLY — KHÔNG vào localStorage
      isAuthenticated: false,
      isLoading: true,

      setUser: (user: User) =>
        set({ user, isAuthenticated: true }),

      setAccessToken: (accessToken: string) =>
        set({ accessToken }),

      setLoading: (isLoading: boolean) =>
        set({ isLoading }),

      logout: () =>
        set({ user: null, accessToken: null, isAuthenticated: false }),
    }),
    {
      name: 'osv-auth',
      // Persist user + isAuthenticated so full-page navigations don't lose auth state.
      // accessToken stays in-memory only and is refreshed via httpOnly cookie.
      // TUYỆT ĐỐI KHÔNG persist accessToken.
      partialize: (state) => ({
        user: state.user,
        isAuthenticated: state.isAuthenticated,
      }),
      onRehydrateStorage: () => (state) => {
        if (state && state.user && !state.isAuthenticated) {
          state.isAuthenticated = true;
        }
        // IMPORTANT: isLoading STAYS true after rehydration.
        // SessionRestorer will set it to false after restoreSession() completes
        // (refresh token → access token → /me). This prevents the race where
        // AuthGuard sees isLoading=false + isAuthenticated=true but accessToken=null.
      },
    }
  )
);

// Expose getState cho Axios interceptor (outside React tree)
export const authStoreApi = useAuthStore;
