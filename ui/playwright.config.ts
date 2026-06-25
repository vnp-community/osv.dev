import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [['html'], ['list']],
  use: {
    baseURL: 'http://localhost:3000',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    // Ensure axios uses relative URLs so MSW Service Worker can intercept
    extraHTTPHeaders: {},
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
  webServer: {
    // VITE_API_BASE_URL='' → axios uses relative URLs → MSW SW intercepts correctly
    command: 'VITE_ENABLE_MSW=true VITE_API_BASE_URL="" pnpm dev',
    url: 'http://localhost:3000',
    reuseExistingServer: !process.env.CI,
    env: {
      VITE_ENABLE_MSW: 'true',
      VITE_API_BASE_URL: '',
    },
  },
});

