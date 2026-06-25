import axios from 'axios';

// Import lazy để tránh circular dependency với authStore
// authStore sẽ import client.ts → không import authStore trực tiếp ở top level
let getAccessToken: () => string | null = () => null;
let setAccessToken: (token: string) => void = () => {};
let logoutFn: () => void = () => {};

/**
 * Inject auth store functions sau khi store được khởi tạo.
 * Gọi trong app/providers.tsx sau khi authStore ready.
 */
export function injectAuthStore(
  getToken: () => string | null,
  setToken: (token: string) => void,
  logout: () => void
) {
  getAccessToken = getToken;
  setAccessToken = setToken;
  logoutFn = logout;
}

// When MSW is active, use relative URLs (empty baseURL) so the Service Worker
// can intercept all requests. In production, use the configured API base URL.
const isMSW = import.meta.env.VITE_ENABLE_MSW === 'true';

export const apiClient = axios.create({
  baseURL: isMSW ? '' : (import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'),
  timeout: 30_000,
  headers: {
    'Content-Type': 'application/json',
  },
  withCredentials: true, // Gửi httpOnly cookie (refresh token)
});

// ─── Request Interceptor: Inject JWT Bearer ────────────────────────────────
apiClient.interceptors.request.use(
  (config) => {
    const token = getAccessToken();
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// ─── Response Interceptor: 401 → Refresh Token ────────────────────────────
apiClient.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    // Skip interception for login/refresh endpoints themselves
    const isAuthEndpoint = originalRequest.url?.includes('/auth/login') || originalRequest.url?.includes('/auth/refresh');

    // Chỉ retry 1 lần (tránh vòng lặp vô tận)
    if (error.response?.status === 401 && !originalRequest._retry && !isAuthEndpoint) {
      originalRequest._retry = true;
      try {
        // POST /api/v1/auth/refresh — refresh token từ httpOnly cookie
        const { data } = await axios.post(
          isMSW ? '/api/v1/auth/refresh' : `${import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'}/api/v1/auth/refresh`,
          {},
          { withCredentials: true }
        );

        const newToken: string = data.access_token;
        setAccessToken(newToken);

        // Retry original request với token mới
        originalRequest.headers.Authorization = `Bearer ${newToken}`;
        return apiClient(originalRequest);
      } catch (refreshError) {
        // Refresh thất bại → logout, để React Router xử lý redirect
        logoutFn();
        return Promise.reject(refreshError);
      }
    }

    return Promise.reject(error);
  }
);
