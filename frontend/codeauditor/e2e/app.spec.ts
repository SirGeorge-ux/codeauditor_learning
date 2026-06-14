import { test, expect } from '@playwright/test';

test.describe('CodeAuditor App', () => {
  test('should load and display home page', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveTitle(/CodeAuditor/i);
  });

  test('should navigate to login page', async ({ page }) => {
    await page.goto('/login');
    await expect(page.locator('h2')).toContainText('Sign In');
    await expect(page.locator('input#email')).toBeVisible();
    await expect(page.locator('input#password')).toBeVisible();
  });

  test('should navigate to register page from login', async ({ page }) => {
    await page.goto('/login');
    await page.click('a[routerlink="/register"]');
    await page.waitForURL('**/register');
    await expect(page.locator('h2')).toContainText('Create Account');
  });

  test('should redirect unauthenticated users to login', async ({ page }) => {
    await page.goto('/dashboard');
    await page.waitForURL('**/login');
    await expect(page.locator('h2')).toContainText('Sign In');
  });

  test('should redirect MCP page to login when unauthenticated', async ({ page }) => {
    await page.goto('/mcp');
    await page.waitForURL('**/login');
    await expect(page.locator('h2')).toContainText('Sign In');
  });

  test('should redirect Vault page to login when unauthenticated', async ({ page }) => {
    await page.goto('/vault');
    await page.waitForURL('**/login');
    await expect(page.locator('h2')).toContainText('Sign In');
  });
});
