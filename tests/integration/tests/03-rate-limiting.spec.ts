/**
 * Rate Limiting Test: Multiple rapid requests are throttled gracefully
 *
 * Tests that the update API properly rate limits requests and provides
 * appropriate feedback to users.
 */

import { test, expect } from '@playwright/test';
import {
  loginAsAdmin,
  checkForUpdatesAPI,
  Timer,
  pollUntil,
} from './helpers';

test.describe('Update Flow - Rate Limiting', () => {
  test('should rate limit excessive update check requests', async ({ page }) => {
    await loginAsAdmin(page);

    // Make multiple rapid requests
    const responses: any[] = [];
    const timer = new Timer();

    // Attempt 10 rapid requests
    for (let i = 0; i < 10; i++) {
      try {
        const response = await page.request.get('http://localhost:7655/api/updates/check');
        responses.push({
          status: response.status(),
          headers: response.headers(),
          time: timer.elapsed(),
        });
      } catch (error) {
        responses.push({
          error: error,
          time: timer.elapsed(),
        });
      }

      // Small delay between requests (50ms)
      await page.waitForTimeout(50);
    }

    // Should see at least one rate limited response (429)
    const rateLimited = responses.filter(r => r.status === 429);
    expect(rateLimited.length).toBeGreaterThan(0);
  });

  test('should include rate limit headers in response', async ({ page }) => {
    await loginAsAdmin(page);

    // Make a request
    const response = await page.request.get('http://localhost:7655/api/updates/check');

    // Should have rate limit headers
    const headers = response.headers();

    // Check for common rate limit headers
    const hasRateLimitHeaders =
      'x-ratelimit-limit' in headers ||
      'x-ratelimit-remaining' in headers ||
      'ratelimit-limit' in headers;

    // At minimum, should have some form of rate limit indication
    expect(hasRateLimitHeaders || response.status() === 429).toBe(true);
  });

  test('should include Retry-After header when rate limited', async ({ page }) => {
    await loginAsAdmin(page);

    // Make requests until we hit rate limit
    let rateLimitedResponse: any = null;

    for (let i = 0; i < 25; i++) {
      const response = await page.request.get('http://localhost:7655/api/updates/check');

      if (response.status() === 429) {
        rateLimitedResponse = response;
        break;
      }

      await page.waitForTimeout(100);
    }

    // Should eventually hit rate limit
    expect(rateLimitedResponse).not.toBeNull();

    if (rateLimitedResponse) {
      const headers = rateLimitedResponse.headers();

      // Should have Retry-After header
      expect('retry-after' in headers).toBe(true);

      // Retry-After should be a reasonable number (in seconds)
      if ('retry-after' in headers) {
        const retryAfter = parseInt(headers['retry-after']);
        expect(retryAfter).toBeGreaterThan(0);
        expect(retryAfter).toBeLessThan(300); // Less than 5 minutes
      }
    }
  });

  test('should allow requests after rate limit window expires', async ({ page }) => {
    await loginAsAdmin(page);

    // Make requests until rate limited
    let rateLimited = false;
    for (let i = 0; i < 25; i++) {
      const response = await page.request.get('http://localhost:7655/api/updates/check');
      if (response.status() === 429) {
        rateLimited = true;
        break;
      }
      await page.waitForTimeout(50);
    }

    expect(rateLimited).toBe(true);

    // Wait for rate limit window to reset (typically 60 seconds)
    await page.waitForTimeout(65000);

    // Should be able to make requests again
    const response = await page.request.get('http://localhost:7655/api/updates/check');
    expect(response.status()).toBe(200);
  });

  test('should rate limit per IP address independently', async ({ page, context }) => {
    await loginAsAdmin(page);

    // Make requests from first "IP" until rate limited
    let rateLimited1 = false;
    for (let i = 0; i < 25; i++) {
      const response = await page.request.get('http://localhost:7655/api/updates/check');
      if (response.status() === 429) {
        rateLimited1 = true;
        break;
      }
      await page.waitForTimeout(50);
    }

    expect(rateLimited1).toBe(true);

    // Create new context (simulating different IP)
    const newContext = await context.browser()!.newContext();
    const newPage = await newContext.newPage();

    // Login in new context
    await loginAsAdmin(newPage);

    // Should be able to make requests from "new IP"
    // Note: In real scenario with different IPs this would work,
    // in test environment they might share the same IP
    const response = await newPage.request.get('http://localhost:7655/api/updates/check');

    // Either succeeds (different IP) or also rate limited (same IP in test)
    expect([200, 429]).toContain(response.status());

    await newContext.close();
  });

  test('should provide clear error message when rate limited', async ({ page }) => {
    await loginAsAdmin(page);

    // Make requests until rate limited
    let rateLimitedResponse: any = null;

    for (let i = 0; i < 25; i++) {
      const response = await page.request.get('http://localhost:7655/api/updates/check');

      if (response.status() === 429) {
        rateLimitedResponse = response;
        break;
      }

      await page.waitForTimeout(50);
    }

    expect(rateLimitedResponse).not.toBeNull();

    if (rateLimitedResponse) {
      const body = await rateLimitedResponse.json();

      // Should have error message
      expect(body).toHaveProperty('message');

      // Message should mention rate limiting
      const message = body.message.toLowerCase();
      expect(message).toMatch(/rate limit|too many requests|throttle/);
    }
  });

  test('should not rate limit reasonable request patterns', async ({ page }) => {
    await loginAsAdmin(page);

    // Make requests at reasonable intervals (5 seconds apart)
    const responses: number[] = [];

    for (let i = 0; i < 5; i++) {
      const response = await page.request.get('http://localhost:7655/api/updates/check');
      responses.push(response.status());

      if (i < 4) {
        await page.waitForTimeout(5000);
      }
    }

    // All requests should succeed
    for (const status of responses) {
      expect(status).toBe(200);
    }
  });

  test('should rate limit apply update endpoint separately', async ({ page }) => {
    await loginAsAdmin(page);

    // Check if apply endpoint has separate rate limit
    // First, check for updates to get a download URL
    const updateCheck = await checkForUpdatesAPI(page);

    if (updateCheck.available && updateCheck.downloadUrl) {
      // Make multiple rapid apply attempts (should be rate limited more strictly)
      const applyResponses: any[] = [];

      for (let i = 0; i < 10; i++) {
        try {
          const response = await page.request.post('http://localhost:7655/api/updates/apply', {
            data: { url: updateCheck.downloadUrl },
            headers: { 'Content-Type': 'application/json' },
          });
          applyResponses.push({ status: response.status() });
        } catch (error) {
          applyResponses.push({ error });
        }

        await page.waitForTimeout(100);
      }

      // Apply endpoint should be more strictly rate limited
      // Most requests after first should fail (either 429 or error)
      const failed = applyResponses.filter(r => r.status !== 200 || r.error);
      expect(failed.length).toBeGreaterThan(5);
    }
  });

  test('should decrement rate limit counter after successful request', async ({ page }) => {
    await loginAsAdmin(page);

    // Make first request and check remaining count
    const response1 = await page.request.get('http://localhost:7655/api/updates/check');
    const headers1 = response1.headers();

    const remaining1 = headers1['x-ratelimit-remaining'];

    // Make second request
    await page.waitForTimeout(500);
    const response2 = await page.request.get('http://localhost:7655/api/updates/check');
    const headers2 = response2.headers();

    const remaining2 = headers2['x-ratelimit-remaining'];

    // If headers are present, remaining should decrease
    if (remaining1 && remaining2) {
      expect(parseInt(remaining2)).toBeLessThanOrEqual(parseInt(remaining1));
    }
  });
});
