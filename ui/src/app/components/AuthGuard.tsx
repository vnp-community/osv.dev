import { Navigate, useLocation } from 'react-router';
import { useAuthStore } from '@/features/auth/store/authStore';

export function FullPageSpinner() {
  return (
    <div
      className="w-full h-screen flex items-center justify-center"
      style={{ background: '#0B1020' }}
    >
      <div
        className="w-10 h-10 rounded-full border-2 border-t-transparent animate-spin"
        style={{ borderColor: '#4F8CFF', borderTopColor: 'transparent' }}
      />
    </div>
  );
}

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuthStore();
  const location = useLocation();

  if (isLoading) return <FullPageSpinner />;

  if (!isAuthenticated) {
    return (
      <Navigate
        to="/login"
        state={{ from: location }}
        replace
      />
    );
  }

  return <>{children}</>;
}
