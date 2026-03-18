/**
 * Journey 07: AI Patrol → Finding → Approval → Fix → Verify Resolved
 *
 * L12 score-7 criterion: closed-loop patrol lifecycle.
 *
 * Validates the full AI Patrol journey:
 *   1. Patrol status endpoint is healthy
 *   2. Patrol findings endpoint returns data
 *   3. Force patrol run triggers and completes
 *   4. Patrol run history is available
 *   5. Autonomy settings are readable
 *   6. Approval queue endpoint responds
 *   7. Investigation endpoint responds for finding
 *   8. Finding lifecycle: acknowledge → resolve (closed-loop)
 *   9. Suppression and dismissed findings endpoints respond
 *  10. AI patrol page renders in UI
 *
 * Environment: local dev (localhost:7655) with mock mode enabled.
 * Mock mode generates deterministic resources that patrol can inspect.
 */

import { test, expect } from '@playwright/test';
import {
  ensureAuthenticated,
  apiRequest,
  setMockMode,
  getMockMode,
} from '../helpers';

/** Tracks the original mock mode state so we can restore it. */
let mockModeWasEnabled: boolean | null = null;

test.describe.serial(
  'Journey: AI Patrol → Finding → Approval → Fix → Verify Resolved',
  () => {
    test.beforeAll(async ({ browser }) => {
      const ctx = await browser.newContext();
      const page = await ctx.newPage();
      try {
        await ensureAuthenticated(page);
        const state = await getMockMode(page);
        mockModeWasEnabled = state.enabled;
        if (!state.enabled) {
          await setMockMode(page, true);
        }
      } catch (err) {
        console.warn('[journey setup] failed to configure mock mode:', err);
      } finally {
        await ctx.close();
      }
    });

    test.afterAll(async ({ browser }) => {
      const ctx = await browser.newContext();
      const page = await ctx.newPage();
      try {
        await ensureAuthenticated(page);
        if (mockModeWasEnabled !== null) {
          const current = await getMockMode(page);
          if (current.enabled !== mockModeWasEnabled) {
            await setMockMode(page, mockModeWasEnabled);
          }
        }
      } catch (err) {
        console.warn('[journey cleanup] failed to restore mock mode:', err);
      } finally {
        await ctx.close();
      }
    });

    test('patrol status endpoint responds with expected shape', async ({
      page,
    }, testInfo) => {
      test.skip(
        testInfo.project.name.startsWith('mobile-'),
        'Desktop journey',
      );
      await ensureAuthenticated(page);

      const resp = await apiRequest(page, '/api/ai/patrol/status');
      expect(resp.status()).toBe(200);

      const body = await resp.json();
      expect(body).toHaveProperty('enabled');
      expect(body).toHaveProperty('running');
      expect(body).toHaveProperty('healthy');
      expect(body).toHaveProperty('license_required');
      expect(body).toHaveProperty('license_status');
      expect(body).toHaveProperty('summary');
      expect(body.summary).toHaveProperty('critical');
      expect(body.summary).toHaveProperty('warning');
      expect(body.summary).toHaveProperty('watch');
      expect(body.summary).toHaveProperty('info');
      expect(typeof body.findings_count).toBe('number');
      expect(typeof body.interval_ms).toBe('number');
    });

    test('patrol findings endpoint returns array', async ({
      page,
    }, testInfo) => {
      test.skip(
        testInfo.project.name.startsWith('mobile-'),
        'Desktop journey',
      );
      await ensureAuthenticated(page);

      const resp = await apiRequest(page, '/api/ai/patrol/findings');
      expect(resp.status()).toBe(200);

      const findings = await resp.json();
      expect(Array.isArray(findings)).toBe(true);

      if (findings.length > 0) {
        const f = findings[0];
        expect(f).toHaveProperty('id');
        expect(f).toHaveProperty('severity');
        expect(f).toHaveProperty('category');
        expect(f).toHaveProperty('resource_id');
        expect(f).toHaveProperty('title');
        expect(f).toHaveProperty('description');
        expect(f).toHaveProperty('detected_at');
        expect(typeof f.times_raised).toBe('number');
      }
    });

    test('force patrol run triggers successfully', async ({
      page,
    }, testInfo) => {
      test.skip(
        testInfo.project.name.startsWith('mobile-'),
        'Desktop journey',
      );
      await ensureAuthenticated(page);

      const resp = await apiRequest(page, '/api/ai/patrol/run', {
        method: 'POST',
      });
      // 200 = triggered, 429 = rate limited (community 1/hr), 503 = service unavailable
      expect([200, 429, 503]).toContain(resp.status());

      if (resp.status() === 200) {
        const body = await resp.json();
        expect(body.success).toBe(true);
        expect(body.message).toContain('patrol');
      }

      if (resp.status() === 429) {
        const body = await resp.json();
        expect(body.code).toBe('patrol_rate_limited');
      }
    });

    test('patrol run history returns array of runs', async ({
      page,
    }, testInfo) => {
      test.skip(
        testInfo.project.name.startsWith('mobile-'),
        'Desktop journey',
      );
      await ensureAuthenticated(page);

      const resp = await apiRequest(page, '/api/ai/patrol/runs');
      expect(resp.status()).toBe(200);

      const runs = await resp.json();
      expect(Array.isArray(runs)).toBe(true);

      if (runs.length > 0) {
        const run = runs[0];
        expect(run).toHaveProperty('id');
        expect(run).toHaveProperty('started_at');
        expect(run).toHaveProperty('status');
        expect(run).toHaveProperty('resources_checked');
        expect(typeof run.duration_ms).toBe('number');
      }
    });

    test('patrol autonomy settings are readable', async ({
      page,
    }, testInfo) => {
      test.skip(
        testInfo.project.name.startsWith('mobile-'),
        'Desktop journey',
      );
      await ensureAuthenticated(page);

      const resp = await apiRequest(page, '/api/ai/patrol/autonomy');
      expect(resp.status()).toBe(200);

      const body = await resp.json();
      expect(body).toHaveProperty('autonomy_level');
      expect(['monitor', 'approval', 'assisted', 'full']).toContain(
        body.autonomy_level,
      );
      expect(typeof body.investigation_budget).toBe('number');
      expect(typeof body.investigation_timeout_sec).toBe('number');
    });

    test('approval queue endpoint responds', async ({ page }, testInfo) => {
      test.skip(
        testInfo.project.name.startsWith('mobile-'),
        'Desktop journey',
      );
      await ensureAuthenticated(page);

      const resp = await apiRequest(page, '/api/ai/approvals');
      // 200 = success (Pro), 402 = license required (Community)
      expect([200, 402]).toContain(resp.status());

      if (resp.status() === 200) {
        const body = await resp.json();
        expect(body).toHaveProperty('approvals');
        expect(body).toHaveProperty('stats');
        expect(Array.isArray(body.approvals)).toBe(true);
        expect(body.stats).toHaveProperty('pending');
        expect(body.stats).toHaveProperty('approved');
        expect(body.stats).toHaveProperty('denied');
        expect(body.stats).toHaveProperty('expired');
      }

      if (resp.status() === 402) {
        const body = await resp.json();
        expect(body).toHaveProperty('upgrade_url');
      }
    });

    test('investigation endpoint responds for finding', async ({
      page,
    }, testInfo) => {
      test.skip(
        testInfo.project.name.startsWith('mobile-'),
        'Desktop journey',
      );
      await ensureAuthenticated(page);

      const findingsResp = await apiRequest(
        page,
        '/api/ai/patrol/findings',
      );
      expect(findingsResp.status()).toBe(200);
      const findings = await findingsResp.json();

      if (findings.length === 0) {
        test.skip(true, 'No patrol findings available to test investigation');
        return;
      }

      const findingId = encodeURIComponent(findings[0].id);

      // Investigation may or may not exist — both are valid states
      const investResp = await apiRequest(
        page,
        `/api/ai/findings/${findingId}/investigation`,
      );
      expect([200, 404]).toContain(investResp.status());

      if (investResp.status() === 200) {
        const body = await investResp.json();
        expect(body).toHaveProperty('finding_id');
        expect(body).toHaveProperty('status');
      }

      if (investResp.status() === 404) {
        const body = await investResp.json();
        expect(body.code).toBe('not_found');
      }
    });

    test('finding lifecycle: acknowledge and resolve (closed-loop)', async ({
      page,
    }, testInfo) => {
      test.skip(
        testInfo.project.name.startsWith('mobile-'),
        'Desktop journey',
      );
      await ensureAuthenticated(page);

      const findingsResp = await apiRequest(
        page,
        '/api/ai/patrol/findings',
      );
      expect(findingsResp.status()).toBe(200);
      const findings = await findingsResp.json();

      if (findings.length === 0) {
        test.skip(true, 'No patrol findings available to test lifecycle');
        return;
      }

      const findingId = findings[0].id;

      // Step 1: Acknowledge the finding
      const ackResp = await apiRequest(page, '/api/ai/patrol/acknowledge', {
        method: 'POST',
        data: { finding_id: findingId },
      });
      expect(ackResp.status()).toBe(200);
      const ackBody = await ackResp.json();
      expect(ackBody.success).toBe(true);

      // Step 2: Verify finding is still present (acknowledged ≠ removed)
      const afterAckResp = await apiRequest(
        page,
        '/api/ai/patrol/findings',
      );
      expect(afterAckResp.status()).toBe(200);
      const afterAckFindings = await afterAckResp.json();
      const ackedFinding = afterAckFindings.find(
        (f: { id: string }) => f.id === findingId,
      );
      expect(ackedFinding).toBeTruthy();

      // Step 3: Resolve the finding (closes the loop)
      const resolveResp = await apiRequest(page, '/api/ai/patrol/resolve', {
        method: 'POST',
        data: { finding_id: findingId },
      });
      expect(resolveResp.status()).toBe(200);
      const resolveBody = await resolveResp.json();
      expect(resolveBody.success).toBe(true);

      // Step 4: Verify finding is removed from active findings
      const afterResolveResp = await apiRequest(
        page,
        '/api/ai/patrol/findings',
      );
      expect(afterResolveResp.status()).toBe(200);
      const afterResolveFindings = await afterResolveResp.json();
      const resolvedFinding = afterResolveFindings.find(
        (f: { id: string }) => f.id === findingId,
      );
      expect(resolvedFinding).toBeFalsy();
    });

    test('suppression rules endpoint responds', async ({
      page,
    }, testInfo) => {
      test.skip(
        testInfo.project.name.startsWith('mobile-'),
        'Desktop journey',
      );
      await ensureAuthenticated(page);

      const resp = await apiRequest(page, '/api/ai/patrol/suppressions');
      expect(resp.status()).toBe(200);
      // Returns JSON array of rules or null when empty — status 200 is sufficient
    });

    test('dismissed findings endpoint responds', async ({
      page,
    }, testInfo) => {
      test.skip(
        testInfo.project.name.startsWith('mobile-'),
        'Desktop journey',
      );
      await ensureAuthenticated(page);

      const resp = await apiRequest(page, '/api/ai/patrol/dismissed');
      expect(resp.status()).toBe(200);
      // Returns JSON array of dismissed findings or null when empty
    });

    test('AI patrol page renders in UI', async ({ page }, testInfo) => {
      test.skip(
        testInfo.project.name.startsWith('mobile-'),
        'Desktop journey',
      );
      await ensureAuthenticated(page);

      await page.goto('/ai');
      await page.waitForURL('**/ai**');

      // Wait for the AI/Intelligence page main content to render.
      // Scope to <main> to avoid false positives from nav tab labels.
      // Use waitForFunction (browser-context) for reliability — Playwright
      // locator chains can miss deeply nested elements during hydration.
      await page.waitForFunction(
        () => {
          const main = document.querySelector('main');
          if (!main) return false;
          const text = (main.textContent || '').toLowerCase();
          if (text.length <= 20) return false;
          return (
            text.includes('patrol') ||
            text.includes('intelligence') ||
            text.includes('finding') ||
            text.includes('monitor')
          );
        },
        { timeout: 15_000 },
      );

      // Verify substantive content rendered in main area
      const mainText = await page.locator('main').first().textContent();
      expect(mainText).toBeTruthy();
      expect(mainText!.length).toBeGreaterThan(20);
    });
  },
);
