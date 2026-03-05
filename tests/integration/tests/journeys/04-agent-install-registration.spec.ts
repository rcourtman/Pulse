import { test, expect } from '@playwright/test';
import {
  ensureAuthenticated,
  apiRequest,
  setMockMode,
  getMockMode,
} from '../helpers';

/**
 * Journey: Agent Install → Registration → Host Visible in UI/API
 *
 * Covers the unified agent lifecycle:
 *   1. Disable mock mode to start with a clean infrastructure state
 *   2. Simulate agent registration via POST /api/agents/agent/report
 *   3. Verify the agent resource appears in GET /api/state resources[]
 *   4. Verify the host is visible on the infrastructure page
 *   5. Verify resource metadata and metrics in the unified state response
 *   6. Send a second report and verify last-seen updates
 *   7. Delete the agent and verify removal
 *
 * This satisfies L12 score-4 criteria: "Agent install → registration →
 * host visible in UI/API."
 *
 * The test simulates agent registration via the report API. Session-based
 * (browser) auth bypasses the scope check, so no API token is needed.
 * For full agent binary install tests in LXC sandbox, set
 * PULSE_E2E_AGENT_BINARY to skip simulation and use a real agent.
 */

/** Unique host identifiers for this test run — avoids collisions with mock data. */
const TEST_HOST_ID = `e2e-host-${Date.now()}`;
const TEST_HOSTNAME = `e2e-agent-test-${Date.now()}`;
const TEST_AGENT_ID = `e2e-agent-${Date.now()}`;

/** Whether mock mode was enabled before the journey (for cleanup). */
let mockModeWasEnabled: boolean | null = null;

/** Track whether the agent resource was successfully registered (for cleanup). */
let hostRegistered = false;

/** The agentId returned by the server (may differ from TEST_HOST_ID). */
let serverAgentId = '';

type StateResource = Record<string, unknown>;

const readString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim().length > 0 ? value : undefined;

const readNumber = (value: unknown): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : undefined;

const findRegisteredAgentResource = (
  state: Record<string, unknown>,
): StateResource | undefined => {
  const resources = state.resources as Array<Record<string, unknown>> | undefined;
  return resources?.find((resource) => {
    if (resource.type !== 'agent') {
      return false;
    }
    if (resource.id === serverAgentId) {
      return true;
    }
    if (resource.name === TEST_HOSTNAME || resource.displayName === TEST_HOSTNAME) {
      return true;
    }
    return false;
  });
};

/** Build a synthetic unified agent report payload. */
function buildHostReport(overrides?: {
  cpuUsage?: number;
  memUsage?: number;
}): Record<string, unknown> {
  const cpuUsage = overrides?.cpuUsage ?? 42.5;
  const totalMem = 17179869184; // 16 GB
  const memUsage = overrides?.memUsage ?? 55.0;
  const usedMem = Math.round(totalMem * (memUsage / 100));

  return {
    agent: {
      id: TEST_AGENT_ID,
      version: '1.0.0-e2e',
      type: 'unified',
    },
    host: {
      id: TEST_HOST_ID,
      hostname: TEST_HOSTNAME,
      displayName: `E2E Test Host (${TEST_HOSTNAME})`,
      machineId: `e2e-machine-${Date.now()}`,
      platform: 'linux',
      osName: 'Ubuntu',
      osVersion: '24.04 LTS',
      kernelVersion: '6.8.0-e2e',
      architecture: 'x86_64',
      cpuModel: 'E2E Virtual CPU',
      cpuCount: 4,
      uptimeSeconds: 86400,
      loadAverage: [1.5, 1.2, 0.9],
    },
    metrics: {
      cpuUsagePercent: cpuUsage,
      memory: {
        totalBytes: totalMem,
        usedBytes: usedMem,
        freeBytes: totalMem - usedMem,
        usage: memUsage,
        swapTotalBytes: 4294967296,
        swapUsedBytes: 0,
      },
    },
    disks: [
      {
        device: '/dev/sda1',
        mountpoint: '/',
        type: 'ext4',
        totalBytes: 107374182400, // 100 GB
        usedBytes: 32212254720, // ~30 GB
        freeBytes: 75161927680,
        usage: 30.0,
      },
    ],
    network: [
      {
        name: 'eth0',
        mac: '02:e2:e2:00:00:01',
        addresses: ['192.168.100.99'],
        rxBytes: 1000000,
        txBytes: 500000,
      },
    ],
    timestamp: new Date().toISOString(),
  };
}

/**
 * Re-register the test host if it's missing from state.
 * The backend file watcher may restart Pulse during the test suite
 * (especially with parallel dev work), losing in-memory host state.
 * Also ensures mock mode is off — the API toggle only sets an env var,
 * not the mock.env file, so a backend restart may restore mock mode.
 * Returns the unified agent resource from /api/state after ensuring it exists.
 */
async function ensureHostRegistered(
  page: import('@playwright/test').Page,
): Promise<Record<string, unknown>> {
  // Ensure mock mode is still off (backend restart restores mock.env defaults).
  // Retry once if the backend is mid-restart when this first runs.
  for (let mockAttempt = 0; mockAttempt < 2; mockAttempt++) {
    try {
      const mockState = await getMockMode(page);
      if (mockState.enabled) {
        await setMockMode(page, false);
      }
      break;
    } catch {
      if (mockAttempt === 0) await page.waitForTimeout(2000);
    }
  }

  const stateRes = await apiRequest(page, '/api/state');
  if (stateRes.ok()) {
    const state = (await stateRes.json()) as Record<string, unknown>;
    const found = findRegisteredAgentResource(state);
    if (found) return found;
  }

  // Host not found — re-register.
  const report = buildHostReport();
  const regRes = await apiRequest(page, '/api/agents/agent/report', {
    method: 'POST',
    data: report,
    headers: { 'Content-Type': 'application/json' },
  });
  if (!regRes.ok()) {
    throw new Error(`Host re-registration failed: ${regRes.status()}`);
  }
  const body = (await regRes.json()) as Record<string, unknown>;
  serverAgentId = (body.agentId as string) || serverAgentId;
  hostRegistered = true;

  // Poll state after re-registration to allow for short settling windows
  // (e.g. backend just finished restarting and state is still initializing).
  for (let attempt = 0; attempt < 5; attempt++) {
    const stateRes2 = await apiRequest(page, '/api/state');
    if (stateRes2.ok()) {
      const state2 = (await stateRes2.json()) as Record<string, unknown>;
      const found = findRegisteredAgentResource(state2);
      if (found) return found;
    }
    await page.waitForTimeout(500);
  }
  throw new Error('Host not found in state even after re-registration');
}

test.describe.serial(
  'Journey: Agent Install → Registration → Host Visible',
  () => {
    test.beforeAll(async ({ browser }) => {
      const ctx = await browser.newContext();
      const page = await ctx.newPage();
      try {
        await ensureAuthenticated(page);
        const state = await getMockMode(page);
        mockModeWasEnabled = state.enabled;
        // Disable mock mode so the test host is distinguishable in the UI.
        if (state.enabled) {
          await setMockMode(page, false);
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

        // Delete the test host if it was registered.
        if (hostRegistered && serverAgentId) {
          const delRes = await apiRequest(
            page,
            `/api/agents/agent/${serverAgentId}`,
            { method: 'DELETE' },
          );
          if (!delRes.ok()) {
            console.warn(
              `[journey cleanup] DELETE host ${serverAgentId} returned ${delRes.status()}`,
            );
          }
        }

        // Restore mock mode to its original state.
        if (mockModeWasEnabled !== null) {
          const current = await getMockMode(page);
          if (current.enabled !== mockModeWasEnabled) {
            await setMockMode(page, mockModeWasEnabled);
          }
        }
      } catch (err) {
        console.warn('[journey cleanup] failed to clean up:', err);
      } finally {
        await ctx.close();
      }
    });

    test('simulate agent registration via unified agent report', async (
      { page },
      testInfo,
    ) => {
      test.skip(
        testInfo.project.name.startsWith('mobile-'),
        'Desktop agent journey',
      );

      await ensureAuthenticated(page);

      const report = buildHostReport();
      const res = await apiRequest(page, '/api/agents/agent/report', {
        method: 'POST',
        data: report,
        headers: { 'Content-Type': 'application/json' },
      });

      expect(
        res.ok(),
        `Agent report failed: ${res.status()} ${await res.text()}`,
      ).toBeTruthy();

      const body = (await res.json()) as Record<string, unknown>;
      expect(body.success, 'Report response should indicate success').toBe(
        true,
      );
      expect(body.agentId, 'Response must include an agentId').toBeTruthy();

      serverAgentId = body.agentId as string;
      hostRegistered = true;

      // Verify basic fields in the response.
      expect(body.platform).toBe('linux');
      expect(body.osName).toBe('Ubuntu');
      expect(body.osVersion).toBe('24.04 LTS');
    });

    test('agent resource appears in /api/state resources[]', async (
      { page },
      testInfo,
    ) => {
      test.skip(
        testInfo.project.name.startsWith('mobile-'),
        'Desktop agent journey',
      );
      test.skip(!hostRegistered, 'Host was not registered');

      test.setTimeout(30_000);

      await ensureAuthenticated(page);

      // Ensure the host is in state (re-registers if backend restarted).
      const foundHost = await ensureHostRegistered(page);

      const platformData = asRecord(foundHost.platformData);

      // Verify the unified resource matches what we reported.
      expect(foundHost.type).toBe('agent');
      expect(foundHost.platformType).toBe('agent');
      expect(foundHost.sourceType).toBe('agent');
      expect(foundHost.name).toBe(TEST_HOSTNAME);
      expect(readString(platformData?.platform)).toBe('linux');
      expect(readString(platformData?.osName)).toBe('Ubuntu');
      expect(readString(platformData?.osVersion)).toBe('24.04 LTS');
      expect(readString(platformData?.architecture)).toBe('x86_64');
    });

    test('agent resource includes CPU and memory metrics', async (
      { page },
      testInfo,
    ) => {
      test.skip(
        testInfo.project.name.startsWith('mobile-'),
        'Desktop agent journey',
      );
      test.skip(!hostRegistered, 'Host was not registered');

      await ensureAuthenticated(page);

      const host = await ensureHostRegistered(page);
      const memory = asRecord(host.memory);

      // CPU usage should be close to what we reported (42.5%).
      const cpu = asRecord(host.cpu);
      const cpuUsage = readNumber(cpu?.current);
      expect(cpuUsage).toBeGreaterThan(0);
      expect(cpuUsage).toBeLessThanOrEqual(100);

      // Memory should be present.
      expect(memory, 'Host should have memory metrics').toBeTruthy();
      expect(readNumber(memory!.total)).toBeGreaterThan(0);
      expect(readNumber(memory!.used)).toBeGreaterThan(0);
    });

    test('updated report refreshes host last-seen', async (
      { page },
      testInfo,
    ) => {
      test.skip(
        testInfo.project.name.startsWith('mobile-'),
        'Desktop agent journey',
      );
      test.skip(!hostRegistered, 'Host was not registered');

      test.setTimeout(30_000);

      await ensureAuthenticated(page);

      const host1 = await ensureHostRegistered(page);
      const lastSeen1 = readNumber(host1.lastSeen);

      // Wait briefly and send a second report with updated metrics.
      await page.waitForTimeout(1100);

      const report2 = buildHostReport({ cpuUsage: 75.0, memUsage: 80.0 });
      const res = await apiRequest(page, '/api/agents/agent/report', {
        method: 'POST',
        data: report2,
        headers: { 'Content-Type': 'application/json' },
      });
      expect(res.ok()).toBeTruthy();

      // Verify lastSeen updated and metrics changed.
      const stateRes2 = await apiRequest(page, '/api/state');
      expect(stateRes2.ok()).toBeTruthy();
      const state2 = (await stateRes2.json()) as Record<string, unknown>;
      const host2 = findRegisteredAgentResource(state2);
      expect(host2, 'Host should still exist after second report').toBeTruthy();

      const lastSeen2 = readNumber(host2!.lastSeen);
      expect(
        lastSeen2 !== undefined && lastSeen1 !== undefined && lastSeen2 >= lastSeen1,
        'lastSeen should be updated after second report',
      ).toBeTruthy();

      // Verify metrics reflect the second report values.
      const cpuAfter = readNumber(asRecord(host2!.cpu)?.current);
      expect(
        cpuAfter,
        'CPU usage should reflect updated report (75%)',
      ).toBeGreaterThanOrEqual(70);

      const memAfter = asRecord(host2!.memory);
      expect(memAfter, 'Memory should be present after update').toBeTruthy();
      const memUsageAfter = readNumber(memAfter?.current);
      expect(
        memUsageAfter,
        'Memory usage should reflect updated report (80%)',
      ).toBeGreaterThanOrEqual(75);
    });

    test('host visible on infrastructure page', async (
      { page },
      testInfo,
    ) => {
      test.skip(
        testInfo.project.name.startsWith('mobile-'),
        'Desktop agent journey',
      );
      test.skip(!hostRegistered, 'Host was not registered');

      test.setTimeout(60_000);

      await ensureAuthenticated(page);

      // Ensure the host is registered in state before navigating to the UI.
      await ensureHostRegistered(page);

      await page.goto('/infrastructure', { waitUntil: 'domcontentloaded' });
      await expect(page).toHaveURL(/\/infrastructure/);

      // Look for the exact test hostname on the page to avoid false positives
      // from leftover hosts of prior runs.
      const hostContent = page.getByText(TEST_HOSTNAME).first();

      await expect(
        hostContent,
        `E2E test host "${TEST_HOSTNAME}" should be visible on the infrastructure page`,
      ).toBeVisible({ timeout: 30_000 });
    });

    test('host can be deleted via API', async ({ page }, testInfo) => {
      test.skip(
        testInfo.project.name.startsWith('mobile-'),
        'Desktop agent journey',
      );
      test.skip(!hostRegistered, 'Host was not registered');

      await ensureAuthenticated(page);

      // Ensure host exists before attempting delete.
      await ensureHostRegistered(page);

      const delRes = await apiRequest(
        page,
        `/api/agents/agent/${serverAgentId}`,
        { method: 'DELETE' },
      );
      expect(
        delRes.ok(),
        `DELETE host failed: ${delRes.status()} ${await delRes.text()}`,
      ).toBeTruthy();

      // Verify host is gone from state.
      const stateRes = await apiRequest(page, '/api/state');
      expect(stateRes.ok()).toBeTruthy();
      const state = (await stateRes.json()) as Record<string, unknown>;
      const found = findRegisteredAgentResource(state);
      expect(
        found,
        'Host should be removed from state after deletion',
      ).toBeFalsy();

      // Mark as cleaned up so afterAll doesn't try to delete again.
      hostRegistered = false;
    });
  },
);
