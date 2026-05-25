
import { test, expect } from '@playwright/test';

test('verify all views', async ({ page }) => {
  await page.goto('http://localhost:8443');

  // Login
  await page.fill('#login-username', 'admin');
  await page.fill('#login-password', 'admin123');
  await page.click('#login-submit-btn');

  await expect(page.locator('#main-app-container')).toBeVisible();

  const views = [
    'dashboard', 'domains', 'filemanager', 'cron', 'databases',
    'backups', 'containers', 'security', 'software', 'system',
    'server-identity', 'migration', 'clustering', 'reseller', 'terminal'
  ];

  for (const view of views) {
    console.log(`Checking view: ${view}`);
    await page.click(`.menu-item[data-target="${view}"]`);
    await page.waitForTimeout(500);
    await page.screenshot({ path: `verification/${view}.png`, fullPage: true });

    // Check if view is visible and has content
    const viewElem = page.locator(`#view-${view}`);
    await expect(viewElem).toBeVisible();

    // Basic check for h3 or h4
    const heading = viewElem.locator('h3, h4').first();
    await expect(heading).toBeVisible();
  }
});
