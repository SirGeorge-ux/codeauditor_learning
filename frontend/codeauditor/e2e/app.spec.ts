import { test, expect } from '@playwright/test';

test.describe('CodeAuditor App', () => {
  test('should load and display home page', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveTitle(/CodeAuditor/i);
  });

  test('should show login form with email and password fields', async ({ page }) => {
    await page.goto('/login');
    await expect(page.locator('h2')).toContainText('Sign In');
    await expect(page.locator('input#email')).toBeVisible();
    await expect(page.locator('input#password')).toBeVisible();
  });

  test('should navigate to register page', async ({ page }) => {
    await page.goto('/register');
    await expect(page.locator('h2')).toContainText('Create Account');
  });

  test('auth guard redirects /dashboard to /login', async ({ page }) => {
    await page.goto('/dashboard');
    await expect(page.locator('h2')).toContainText('Sign In');
  });

  test('auth guard redirects /mcp to /login', async ({ page }) => {
    await page.goto('/mcp');
    await expect(page.locator('h2')).toContainText('Sign In');
  });

  test('auth guard redirects /vault to /login', async ({ page }) => {
    await page.goto('/vault');
    await expect(page.locator('h2')).toContainText('Sign In');
  });
});
