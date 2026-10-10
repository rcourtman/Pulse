import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { test } from '@playwright/test';
import { createAuthenticatedStorageState } from '../tests/helpers';

test('admit the authenticated default mock inventory and seven-day history', async ({ browser }) => {
  if (!['1', 'true', 'yes', 'on'].includes(
    String(process.env.PULSE_E2E_REQUIRE_DEFAULT_MOCK_READY || '').trim().toLowerCase(),
  )) {
    throw new Error('default mock admission requires PULSE_E2E_REQUIRE_DEFAULT_MOCK_READY=true');
  }

  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'pulse-e2e-readiness-'));
  try {
    // The existing helper performs first-run authentication and the complete
    // inventory/pool/disk-history checks. It also retains the usual shared
    // cookie session for subsequent fixtures; do not add another login path.
    await createAuthenticatedStorageState(browser, path.join(directory, 'session.json'));
  } finally {
    fs.rmSync(directory, { recursive: true, force: true });
  }
});
