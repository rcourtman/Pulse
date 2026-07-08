/**
 * Update-flow coverage against the mock GitHub server.
 *
 * The compose stack (docker-compose.test.yml) points PULSE_UPDATE_SERVER at
 * tests/integration/mock-github-server, which serves sentinel releases
 * v99.0.0 (stable) and v99.1.0-rc.1 (prerelease). This spec replaces the
 * pre-v6 Go test tests/integration/api/update_flow_test.go at the same
 * surface: /api/updates/{check,plan,apply,status}.
 *
 * v6 made SSHSIG verification against the pinned pulse-installer key
 * mandatory and fail-closed (internal/updates/signature.go), so an apply can
 * no longer complete against unsigned mock artifacts. The apply test asserts
 * the fail-closed rejection instead of a completed update; a "completed"
 * status here would mean an unsigned artifact was installed.
 *
 * Self-skips when the update check does not surface the mock sentinel
 * release (e.g. managed local backend without PULSE_UPDATE_SERVER).
 */

import { expect, test, type Page } from '@playwright/test';
import { apiRequest, ensureAuthenticated } from './helpers';

const MOCK_STABLE_VERSION = '99.0.0';
const MOCK_RC_VERSION = '99.1.0-rc.1';

type UpdateInfo = {
  available: boolean;
  currentVersion: string;
  latestVersion: string;
  downloadUrl: string;
  isPrerelease: boolean;
};

type UpdateStatus = {
  status: string;
  progress: number;
  message: string;
  error?: string;
};

let mockUpdateServerWired: boolean | null = null;

async function checkUpdates(page: Page, channel: 'stable' | 'rc'): Promise<UpdateInfo> {
  const res = await apiRequest(page, `/api/updates/check?channel=${channel}`);
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()) as UpdateInfo;
}

async function fetchUpdateStatus(page: Page): Promise<UpdateStatus> {
  const res = await apiRequest(page, '/api/updates/status');
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()) as UpdateStatus;
}

test.describe.serial('update flow against the mock update server', () => {
  test.beforeEach(async ({ page }) => {
    await ensureAuthenticated(page);
    if (mockUpdateServerWired === null) {
      const res = await apiRequest(page, '/api/updates/check?channel=stable');
      if (!res.ok()) {
        mockUpdateServerWired = false;
      } else {
        const info = (await res.json()) as UpdateInfo;
        mockUpdateServerWired = info.latestVersion === MOCK_STABLE_VERSION;
      }
    }
    test.skip(
      !mockUpdateServerWired,
      'update check is not served by the mock GitHub server (PULSE_UPDATE_SERVER not wired)',
    );
  });

  test('stable channel offers the latest stable release, not the prerelease', async ({ page }) => {
    const info = await checkUpdates(page, 'stable');
    expect(info.available, JSON.stringify(info)).toBeTruthy();
    expect(info.latestVersion).toBe(MOCK_STABLE_VERSION);
    expect(info.isPrerelease).toBeFalsy();
    expect(info.downloadUrl).toContain(`pulse-v${MOCK_STABLE_VERSION}-linux-amd64.tar.gz`);
  });

  test('rc channel surfaces the newer prerelease', async ({ page }) => {
    const info = await checkUpdates(page, 'rc');
    expect(info.latestVersion, JSON.stringify(info)).toBe(MOCK_RC_VERSION);
    expect(info.isPrerelease).toBeTruthy();
    expect(info.downloadUrl).toContain(`pulse-v${MOCK_RC_VERSION}-linux-amd64.tar.gz`);
  });

  test('update plan reports manual instructions for the docker deployment', async ({ page }) => {
    // The test image is a release build, where PULSE_MOCK_MODE fails closed
    // for deployment-type detection: the container is honestly a "docker"
    // deployment, which cannot replace its own image, so the plan offers
    // instructions with readiness attached instead of an auto update.
    const res = await apiRequest(
      page,
      `/api/updates/plan?version=${MOCK_STABLE_VERSION}&channel=stable`,
    );
    expect(res.ok(), await res.text()).toBeTruthy();
    const plan = await res.json();
    expect(plan.canAutoUpdate, JSON.stringify(plan)).toBeFalsy();
    expect(plan.rollbackSupport).toBeTruthy();
    expect(Array.isArray(plan.instructions) && plan.instructions.length > 0).toBeTruthy();
    expect(plan.readiness?.status, JSON.stringify(plan.readiness)).toBeTruthy();
  });

  test('stable channel refuses to apply a prerelease download URL', async ({ page }) => {
    const rcInfo = await checkUpdates(page, 'rc');
    const res = await apiRequest(page, '/api/updates/apply?channel=stable', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      data: { downloadUrl: rcInfo.downloadUrl },
    });
    const body = await res.text();
    expect(res.status(), body).toBe(409);
    expect(body).toMatch(/prerelease/i);
  });

  test('apply of an unsigned artifact fails closed at signature verification', async ({ page }) => {
    const info = await checkUpdates(page, 'stable');
    const applyRes = await apiRequest(page, '/api/updates/apply?channel=stable', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      data: { downloadUrl: info.downloadUrl },
    });
    // A fast failure inside the start-ack window surfaces as a 5xx with a
    // generic message; a slower one returns "started". Either way the real
    // failure reason lands in /api/updates/status.
    if (!applyRes.ok()) {
      expect(applyRes.status(), await applyRes.text()).toBeGreaterThanOrEqual(500);
    }

    let last: UpdateStatus | null = null;
    const deadline = Date.now() + 60_000;
    while (Date.now() < deadline) {
      last = await fetchUpdateStatus(page);
      expect(last.status, JSON.stringify(last)).not.toBe('completed');
      if (last.status === 'error') {
        break;
      }
      await page.waitForTimeout(500);
    }
    expect(last?.status, JSON.stringify(last)).toBe('error');
    expect(`${last?.error ?? ''} ${last?.message ?? ''}`).toMatch(/signature/i);
  });
});
