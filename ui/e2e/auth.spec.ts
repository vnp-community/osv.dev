import { test, expect } from '@playwright/test';

test.describe('Authentication Flow', () => {
  test('login success → redirect to dashboard', async ({ page }) => {
    await page.goto('/login');
    await page.fill('[id="email"]', 'admin@osv.local');
    await page.fill('[id="password"]', 'password');
    await page.click('[id="login-btn"]');
    // Wait for URL change — MSW handles auth, app navigates to /dashboard
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 });
    // Wait for page content to fully render (lazy loading + data fetching)
    await page.waitForLoadState('networkidle');
    await expect(
      page.locator('h1, h2, [data-testid="page-title"], [data-testid="dashboard-title"]')
    ).toContainText(/Dashboard|Security|Overview/i, { timeout: 10_000 });
  });

  test('protected route → redirect to login', async ({ page }) => {
    await page.goto('/findings');
    await expect(page).toHaveURL(/\/login/, { timeout: 10_000 });
  });

  test('invalid credentials → show error', async ({ page }) => {
    await page.goto('/login');
    await page.fill('[id="email"]', 'wrong@email.com');
    await page.fill('[id="password"]', 'wrongpass');
    await page.click('[id="login-btn"]');
    await expect(page.locator('[role="alert"], [data-testid="login-error"]')).toBeVisible({ timeout: 10_000 });
  });
});
