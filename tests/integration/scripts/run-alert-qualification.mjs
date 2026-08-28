#!/usr/bin/env node

import fs from 'node:fs/promises';
import path from 'node:path';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const scriptsDir = path.dirname(fileURLToPath(import.meta.url));
const integrationRoot = path.resolve(scriptsDir, '..');
const repoRoot = path.resolve(integrationRoot, '..', '..');
const runId = `alert-qualification-${Date.now()}-${process.pid}`;
const runRoot = path.join(repoRoot, 'tmp', runId);
const fixtureRoot = path.join(runRoot, 'fixtures');
const runtimeStatePath = path.join(repoRoot, 'tmp', `${runId}.runtime-state.json`);
const binaryPath = path.join(repoRoot, 'tmp', 'integration-local-backend', 'bin', 'pulse-alert-qualification');

const minutesAgo = (minutes) => new Date(Date.now() - minutes * 60_000).toISOString();

const legacyHistory = [
  {
    alert: {
      id: 'legacy-history-qualification',
      type: 'cpu',
      level: 'warning',
      resourceId: 'qualification-legacy-vm',
      resourceName: 'Legacy Qualification VM',
      node: 'qualification-node',
      nodeDisplayName: 'Qualification Node',
      instance: 'qualification',
      message: 'Legacy history import rendered from SQLite',
      value: 91,
      threshold: 80,
      startTime: minutesAgo(90),
      lastSeen: minutesAgo(60),
      acknowledged: false,
      metadata: { resourceType: 'vm' },
    },
    timestamp: minutesAgo(60),
  },
];

const activeAlerts = [
  {
    id: 'active-overlay-qualification',
    type: 'memory',
    level: 'warning',
    resourceId: 'agent:qualification-active',
    resourceName: 'Active Overlay Node',
    node: 'qualification-active',
    nodeDisplayName: 'Active Overlay Node',
    instance: 'qualification',
    message: 'Active alert remains visible after history clear',
    value: 86,
    threshold: 80,
    startTime: minutesAgo(10),
    lastSeen: minutesAgo(1),
    acknowledged: false,
    metadata: { resourceType: 'agent' },
  },
];

const run = (command, args, options = {}) =>
  new Promise((resolve, reject) => {
    const child = spawn(command, args, { stdio: 'inherit', ...options });
    child.on('error', reject);
    child.on('close', (code) => resolve(code ?? 1));
  });

await fs.mkdir(fixtureRoot, { recursive: true });
const historyFixturePath = path.join(fixtureRoot, 'alert-history.json');
const activeFixturePath = path.join(fixtureRoot, 'active-alerts.json');
await fs.writeFile(historyFixturePath, `${JSON.stringify(legacyHistory, null, 2)}\n`, 'utf8');
await fs.writeFile(activeFixturePath, `${JSON.stringify(activeAlerts, null, 2)}\n`, 'utf8');

let exitCode = 1;
try {
  const runtimeQualificationPattern = [
    'TestDurableLifecycleFailureSynchronouslyCheckpointsRecoveryMirror',
    'TestDurableLifecycleFailureSurfacesRecoveryMarkerFailure',
    'TestDegradedMarkerRepairsSQLiteFromRecoveryMirror',
    'TestAlertSnoozeIsDurableAndEnforcedByDeliveryPolicy',
    'TestSnoozeExpiryResumesEscalationWithoutReplayingMissedLevels',
    'TestDeliveryReceiptPersistsAndIsClearedAfterRecovery',
    'TestSendResolvedWebhookHTTP',
    'TestBuildNotificationDeliveryJobsRoutesGroupedAlertsByDestinationSeverity',
    'TestBuildNotificationDeliveryJobsTargetsExactEscalationDestinations',
    'TestWebhookDeliveryCarriesSignatureAndEventID',
    'TestDeadManRunCycleSendsHealthySignalAndPersistsProgress',
    'TestDeadManRestartGapIsReportedExternallyAndRecordedInAlertHistory',
  ].join('|');
  exitCode = await run(
    'go',
    [
      'test',
      './internal/alerts',
      './internal/notifications',
      './internal/monitoring',
      '-run',
      `^(${runtimeQualificationPattern})$`,
      '-count=1',
    ],
    { cwd: repoRoot, env: process.env },
  );

  if (exitCode === 0) {
    exitCode = await run(
      process.execPath,
      [
        './scripts/run-playwright.mjs',
        'tests/93-alert-operator-qualification.spec.ts',
        'tests/94-alert-history-real-backend.spec.ts',
        '--project=chromium',
      ],
      {
        cwd: integrationRoot,
        env: {
          ...process.env,
          PULSE_E2E_USE_LOCAL_BACKEND: '1',
          PULSE_MOCK_MODE: 'false',
          PULSE_E2E_ALERT_HISTORY_QUALIFICATION: '1',
          PULSE_E2E_ALERT_HISTORY_FIXTURE: historyFixturePath,
          PULSE_E2E_ACTIVE_ALERTS_FIXTURE: activeFixturePath,
          PULSE_E2E_LOCAL_BACKEND_BINARY: binaryPath,
          PULSE_E2E_RUN_ID: runId,
          PULSE_E2E_RUNTIME_STATE_PATH: runtimeStatePath,
          PULSE_E2E_REPORT_DIR: path.join(runRoot, 'playwright-report'),
          PULSE_E2E_RESULTS_DIR: path.join(runRoot, 'test-results'),
        },
      },
    );
  }
} finally {
  await fs.rm(fixtureRoot, { recursive: true, force: true });
  await fs.rm(runtimeStatePath, { force: true });
}

process.exit(exitCode);
