import { QueryClientProvider } from '@tanstack/react-query';
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';
import { Toaster } from 'sonner';
import { useEffect } from 'react';
import { queryClient } from '@/shared/api/queryClient';
import { injectAuthStore } from '@/shared/api/client';
import { useAuthStore } from '@/features/auth/store/authStore';

function AuthStoreInjector({ children }: { children: React.ReactNode }) {
  const { setAccessToken, logout } = useAuthStore();

  useEffect(() => {
    // Inject auth store functions vào Axios client (tránh circular import)
    injectAuthStore(
      () => useAuthStore.getState().accessToken,
      setAccessToken,
      logout
    );
  }, [setAccessToken, logout]);

  return <>{children}</>;
}

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthStoreInjector>
        {children}
      </AuthStoreInjector>
      <Toaster
        position="top-right"
        theme="dark"
        toastOptions={{
          style: {
            background: '#1E2A45',
            border: '1px solid rgba(255,255,255,0.1)',
            color: '#E5E7EB',
            fontFamily: "'Inter', sans-serif",
          },
        }}
      />
      {import.meta.env.DEV && (
        <ReactQueryDevtools initialIsOpen={false} buttonPosition="bottom-right" />
      )}
    </QueryClientProvider>
  );
}
