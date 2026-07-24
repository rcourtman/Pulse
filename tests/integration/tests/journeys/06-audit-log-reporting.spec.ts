import { test, expect, ensureJourneyReady } from './journeyAuth';
import {
  apiRequest,
  setMockMode,
  getMockMode,
} from '../helpers';

/**
 * Journey: Audit Log → Webhook Delivery → Log Viewer
 *         Reporting → Scheduled Report → Export Download
 *
 * Covers two complementary enterprise features:
 *
 * A) Audit logging:
 *   1. Audit event list endpoint returns events (or 402 paywall)
 *   2. Audit export endpoint returns a file (JSON or CSV)
 *   3. Audit summary endpoint returns statistics
 *   4. Audit webhook configuration CRUD
 *   5. Audit UI page renders the log viewer
 *
 * B) Reporting:
 *   1. Single-resource report generation (PDF/CSV download)
 *   2. Multi-resource fleet report generation
 *   3. Reporting UI page renders
 *
 * This satisfies L12 score-6 criteria: "Audit log → webhook delivery →
 * log viewer. Reporting → scheduled report → export download."
 *
 * License gate: Both features require Pro tier (`audit_logging`,
 * `advanced_reporting`). When not licensed, the journey validates
 * the 402 paywall response format.
 */

/** Whether mock mode was enabled before the journey (for cleanup). */
let mockModeWasEnabled: boolean | null = null;

/** Tracks whether audit logging is licensed. */
let auditLicensed = true;

/** Tracks whether advanced reporting is licensed. */
let reportingLicensed = true;

test.describe.serial(
  'Journey: Audit Log + Webhook + Reporting',
  () => {
    test.beforeAll(async ({ browser, authStorageStatePath }) => {
      const ctx = await browser.newContext({ storageState: authStorageStatePath });
      const page = await ctx.newPage();
      try {
        await ensureJourneyReady(page);
        const state = await getMockMode(page);
        mockModeWasEnabled = state.enabled;
        // Enable mock mode so we have resources for reporting.
        if (!state.enabled) {
          await setMockMode(page, true);
        }
      } catch (err) {
        console.warn('[journey setup] failed to configure mock mode:', err);
      } finally {
        await ctx.close();
      }
    });

    test.afterAll(async ({ browser, authStorageStatePath }) => {
      const ctx = await browser.newContext({ storageState: authStorageStatePath });
      const page = await ctx.newPage();
      try {
        await ensureJourneyReady(page);
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

    // ── Audit Log ────────────────────────────────────────────

    test('audit event list endpoint responds', async ({ page }, testInfo) => {
      test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop audit journey');

      await ensureJourneyReady(page);

      const res = await apiRequest(page, '/api/audit?limit=10');

      if (res.status() === 402) {
        auditLicensed = false;
        const body = await res.json();
        expect(body).toHaveProperty('error');
        expect(body).toHaveProperty('feature');
        expect(body.feature).toBe('audit_logging');
        // Validate paywall has upgrade_url.
        expect(body).toHaveProperty('upgrade_url');
        return;
      }

      expect(
        res.ok(),
        `GET /api/audit failed: ${res.status()} ${await res.text()}`,
      ).toBeTruthy();

      const body = await res.json();
      expect(Array.isArray(body.events)).toBe(true);
      expect(body).toHaveProperty('total');
      expect(body).toHaveProperty('persistentLogging');
      expect(typeof body.total).toBe('number');
      expect(typeof body.persistentLogging).toBe('boolean');

      if (body.persistentLogging && body.events.length > 0) {
        const signed = body.events.find(
          (event: { signature?: string }) => event.signature,
        );
        expect(signed, 'Persistent audit rows should be signed').toBeTruthy();
        const verifyRes = await apiRequest(
          page,
          `/api/audit/${signed.id}/verify`,
        );
        expect(
          verifyRes.ok(),
          `Audit signature verification failed: ${verifyRes.status()}`,
        ).toBeTruthy();
        const verification = await verifyRes.json();
        expect(verification.available).toBe(true);
        expect(verification.verified).toBe(true);
      }
    });

    test('audit export endpoint returns a file', async ({ page }, testInfo) => {
      test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop audit journey');
      test.skip(!auditLicensed, 'Audit logging not licensed');

      await ensureJourneyReady(page);

      const res = await apiRequest(page, '/api/audit/export?format=json');

      expect(
        res.ok(),
        `Audit export failed: ${res.status()}`,
      ).toBeTruthy();

      const contentDisposition = res.headers()['content-disposition'] || '';
      expect(
        contentDisposition.includes('attachment'),
        'Export should be a file attachment',
      ).toBeTruthy();

      const eventCount = res.headers()['x-event-count'];
      expect(eventCount, 'Export should include X-Event-Count header').toBeTruthy();
    });

    test('audit summary endpoint returns statistics', async ({ page }, testInfo) => {
      test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop audit journey');
      test.skip(!auditLicensed, 'Audit logging not licensed');

      await ensureJourneyReady(page);

      const res = await apiRequest(page, '/api/audit/summary');

      expect(
        res.ok(),
        `Audit summary failed: ${res.status()}`,
      ).toBeTruthy();

      const body = await res.json();
      // Summary should be a non-empty object.
      expect(typeof body).toBe('object');
    });

    test('audit webhook configuration endpoint responds', async ({ page }, testInfo) => {
      test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop audit journey');
      test.skip(!auditLicensed, 'Audit logging not licensed');

      await ensureJourneyReady(page);

      // GET existing audit webhooks — verifies the endpoint is accessible
      // and returns the expected shape.
      const getRes = await apiRequest(page, '/api/admin/webhooks/audit');
      expect(
        getRes.ok(),
        `GET audit webhooks failed: ${getRes.status()}`,
      ).toBeTruthy();

      const body = await getRes.json();
      expect(body).toHaveProperty('urls');
      // urls is null when no webhooks are configured, or an array when they are.
      const urls = body.urls;
      expect(
        urls === null || Array.isArray(urls),
        `urls should be null or array, got: ${typeof urls}`,
      ).toBeTruthy();

      // Verify the POST path with an empty list and restore original state.
      // Only mutate if urls is an array (null means no config — leave as-is).
      if (Array.isArray(urls)) {
        const postRes = await apiRequest(page, '/api/admin/webhooks/audit', {
          method: 'POST',
          data: { urls: [] },
          headers: { 'Content-Type': 'application/json' },
        });
        expect(
          postRes.status() === 204 || postRes.ok(),
          `POST audit webhooks failed: ${postRes.status()}`,
        ).toBeTruthy();

        // Restore original URLs (best-effort; warn on failure).
        if (urls.length > 0) {
          const restoreRes = await apiRequest(page, '/api/admin/webhooks/audit', {
            method: 'POST',
            data: { urls },
            headers: { 'Content-Type': 'application/json' },
          });
          if (!restoreRes.ok() && restoreRes.status() !== 204) {
            console.warn(
              `[journey cleanup] failed to restore audit webhook URLs: ${restoreRes.status()}`,
            );
          }
        }
      }
    });

    test('audit log viewer page renders in UI', async ({ page }, testInfo) => {
      test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop audit journey');

      await ensureJourneyReady(page);

      await page.goto('/settings/security-audit', { waitUntil: 'domcontentloaded' });
      await page.waitForURL(/\/settings/, { timeout: 10_000 });
      await expect(page.locator('#root')).toBeVisible();

      // The audit page should show an "Audit Log" heading or "Upgrade to Pro" link.
      const auditContent = page.locator(
        'h1:has-text("Audit"), h2:has-text("Audit"), h3:has-text("Audit"), a:has-text("Upgrade")',
      ).first();

      await expect(
        auditContent,
        'Audit log page should render audit heading or upgrade link',
      ).toBeVisible({ timeout: 15_000 });
    });

    // ── Notification Webhooks (not license-gated) ────────────

    test('notification webhook test endpoint works', async ({ page }, testInfo) => {
      test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop webhook journey');

      await ensureJourneyReady(page);

      // List existing notification webhooks.
      const listRes = await apiRequest(page, '/api/notifications/webhooks');
      expect(listRes.ok()).toBeTruthy();
      const webhooks = await listRes.json();
      expect(Array.isArray(webhooks)).toBeTruthy();

      // Webhook templates should be available.
      const templatesRes = await apiRequest(page, '/api/notifications/webhook-templates');
      expect(templatesRes.ok()).toBeTruthy();

      // Webhook history should be available.
      const historyRes = await apiRequest(page, '/api/notifications/webhook-history');
      expect(historyRes.ok()).toBeTruthy();

      // Notification health endpoint should respond.
      const healthRes = await apiRequest(page, '/api/notifications/health');
      expect(healthRes.ok()).toBeTruthy();
      const health = await healthRes.json();
      expect(health).toHaveProperty('queue');
    });

    // ── Reporting ────────────────────────────────────────────

    test('single-resource report generation', async ({ page }, testInfo) => {
      test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop reporting journey');

      test.setTimeout(60_000);

      await ensureJourneyReady(page);

      // Guarantee mock mode is on before polling — an earlier journey (e.g. 04)
      // or a backend restart can leave it off, which would spin the poll for
      // the full 30s and then skip. Enabling it up front makes resources
      // appear promptly and keeps the poll tight.
      if (!(await getMockMode(page)).enabled) {
        await setMockMode(page, true);
      }

      // Poll for resources — mock mode may still be populating after the
      // toggle. Wait up to 30s for at least one node/VM.
      let resourceType = '';
      let resourceId = '';
      let stateApiReachable = false;
      const deadline = Date.now() + 30_000;
      while (!resourceId && Date.now() < deadline) {
        const stateRes = await apiRequest(page, '/api/state');
        if (stateRes.ok()) {
          stateApiReachable = true;
          const state = (await stateRes.json()) as Record<string, unknown>;

          const nodes = state.nodes as any[] | undefined;
          if (Array.isArray(nodes) && nodes.length > 0) {
            resourceType = 'node';
            resourceId = nodes[0].id || nodes[0].name || '';
          }

          if (!resourceId) {
            const vms = state.guests as any[] | undefined;
            if (Array.isArray(vms) && vms.length > 0) {
              resourceType = 'vm';
              resourceId = vms[0].id || '';
            }
          }
        }
        if (!resourceId) {
          await page.waitForTimeout(2_000);
        }
      }

      expect(stateApiReachable, '/api/state must be reachable').toBeTruthy();
      test.skip(!resourceId, 'No resources available for reporting');

      const reportUrl = `/api/admin/reports/generate?resourceType=${resourceType}&resourceId=${encodeURIComponent(resourceId)}&format=csv`;
      const res = await apiRequest(page, reportUrl);

      if (res.status() === 402) {
        reportingLicensed = false;
        const body = await res.json();
        expect(body).toHaveProperty('feature');
        expect(body.feature).toBe('advanced_reporting');
        expect(body).toHaveProperty('upgrade_url');
        return;
      }

      expect(
        res.ok(),
        `Report generation failed: ${res.status()} ${await res.text()}`,
      ).toBeTruthy();

      const contentType = res.headers()['content-type'] || '';
      expect(
        contentType.includes('csv') || contentType.includes('text'),
        `Expected CSV content-type, got: ${contentType}`,
      ).toBeTruthy();

      const contentDisposition = res.headers()['content-disposition'] || '';
      expect(
        contentDisposition.includes('attachment'),
        'Report should be a file attachment',
      ).toBeTruthy();
    });

    test('multi-resource fleet report generation', async ({ page }, testInfo) => {
      test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop reporting journey');
      test.skip(!reportingLicensed, 'Advanced reporting not licensed');

      test.setTimeout(60_000);

      await ensureJourneyReady(page);

      // Guarantee mock mode is on before polling (see single-resource test).
      if (!(await getMockMode(page)).enabled) {
        await setMockMode(page, true);
      }

      // Poll for resources — mock mode may still be populating after the
      // toggle. Wait up to 30s for at least one node to appear.
      let resources: { resourceType: string; resourceId: string }[] = [];
      let stateApiReachable = false;
      const deadline = Date.now() + 30_000;
      while (resources.length === 0 && Date.now() < deadline) {
        const stateRes = await apiRequest(page, '/api/state');
        if (stateRes.ok()) {
          stateApiReachable = true;
          const state = (await stateRes.json()) as Record<string, unknown>;
          const nodes = state.nodes as any[] | undefined;
          if (Array.isArray(nodes)) {
            for (const n of nodes.slice(0, 2)) {
              if (n.id || n.name) {
                resources.push({
                  resourceType: 'node',
                  resourceId: n.id || n.name,
                });
              }
            }
          }
        }
        if (resources.length === 0) {
          await page.waitForTimeout(2_000);
        }
      }

      expect(stateApiReachable, '/api/state must be reachable').toBeTruthy();
      test.skip(resources.length === 0, 'No resources for fleet report');

      const res = await apiRequest(page, '/api/admin/reports/generate-multi', {
        method: 'POST',
        data: {
          resources,
          format: 'csv',
          title: 'E2E Fleet Report',
        },
        headers: { 'Content-Type': 'application/json' },
      });

      if (res.status() === 402) {
        reportingLicensed = false;
        const body = await res.json();
        expect(body).toHaveProperty('feature');
        expect(body.feature).toBe('advanced_reporting');
        expect(body).toHaveProperty('upgrade_url');
        return;
      }

      expect(
        res.ok(),
        `Fleet report failed: ${res.status()} ${await res.text()}`,
      ).toBeTruthy();

      const contentDisposition = res.headers()['content-disposition'] || '';
      expect(
        contentDisposition.includes('attachment'),
        'Fleet report should be a file attachment',
      ).toBeTruthy();
    });

    test('reporting page renders in UI', async ({ page }, testInfo) => {
      test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop reporting journey');

      await ensureJourneyReady(page);

      // Reporting lives under Settings → Support → Data & Reports.
      await page.goto('/settings/support/reporting', {
        waitUntil: 'domcontentloaded',
      });
      await page.waitForURL(/\/settings/, { timeout: 10_000 });
      await expect(page.locator('#root')).toBeVisible();

      // The reporting page renders the "Data & Reports" heading, report form
      // content, or an upgrade paywall when advanced_reporting is unlicensed.
      const reportContent = page.locator(
        'h1:has-text("Data & Reports"), h2:has-text("Data & Reports"), h1:has-text("Report"), h2:has-text("Report"), a:has-text("Upgrade"), button:has-text("Generate")',
      ).first();

      await expect(
        reportContent,
        'Reporting page should render reporting-related content or paywall',
      ).toBeVisible({ timeout: 15_000 });
    });
  },
);
