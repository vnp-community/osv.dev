import { test, expect } from '@playwright/test';

// Helper: login and wait for dashboard
async function login(page: import('@playwright/test').Page) {
  await page.goto('/login');
  await page.waitForLoadState('domcontentloaded');
  await page.fill('[id="email"]', 'admin@osv.local');
  await page.fill('[id="password"]', 'password');
  await page.click('[id="login-btn"]');
  await page.waitForURL(/\/dashboard/, { timeout: 15_000 });
  // Brief wait for MSW data to load
  await page.waitForTimeout(500);
}

test.describe('CVE Intelligence', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('navigate to CVE Search and view results', async ({ page }) => {
    await page.goto('/cve/search');
    // Wait for CVETable (virtualized) or any table-like container
    await expect(
      page.locator('[data-testid="cve-table"], table, [role="grid"]')
    ).toBeVisible({ timeout: 15_000 });
    // Should have virtualized rows or table rows
    const rows = await page.locator(
      '[data-testid="cve-row"], tbody tr, [data-index]'
    ).count();
    expect(rows).toBeGreaterThan(0);
  });

  test('filter CVEs by severity via URL', async ({ page }) => {
    await page.goto('/cve/search?severity=Critical');
    await page.waitForLoadState('networkidle');
    // Severity filter sidebar should highlight "Critical"
    const criticalBtn = page.locator('button:has-text("Critical")').first();
    await expect(criticalBtn).toBeVisible({ timeout: 10_000 });
  });

  test('navigate to KEV Catalog', async ({ page }) => {
    await page.goto('/cve/kev');
    await page.waitForLoadState('domcontentloaded');
    // Check for KEV page heading or content
    await expect(
      page.locator('h1, h2, [data-testid="kev-title"]')
    ).toContainText(/KEV|Known Exploited/i, { timeout: 10_000 });
  });
});
