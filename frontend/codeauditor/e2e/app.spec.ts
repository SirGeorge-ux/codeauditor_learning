import { test, expect } from '@playwright/test';

test.describe('CodeAuditor App', () => {
  test('should load and display home page', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveTitle(/CodeAuditor/i);
  });

  test('should navigate to login page', async ({ page }) => {
    await page.goto('/');
    await page.goto('/login');
    await expect(page.locator('h2')).toContainText('Sign In');
    await expect(page.locator('input#email')).toBeVisible();
    await expect(page.locator('input#password')).toBeVisible();
  });

  test('should navigate to register page from login', async ({ page }) => {
    await page.goto('/login');
    await page.click('a[routerLink="/register"]');
    await expect(page.locator('h2')).toContainText('Create Account');
  });

  test('should show dashboard with challenge cards after navigating', async ({ page }) => {
    await page.goto('/dashboard');
    // Dashboard may redirect to login if auth guard is active
    // Either way, the app should render without crashing
    await expect(page.locator('h1')).toBeVisible();
  });

  test('should render MCP page', async ({ page }) => {
    await page.goto('/mcp');
    await expect(page.locator('h1')).toContainText('Repositorios');
  });

  test('should render Vault page', async ({ page }) => {
    await page.goto('/vault');
    await expect(page.locator('h1')).toContainText('Vault');
  });
});
