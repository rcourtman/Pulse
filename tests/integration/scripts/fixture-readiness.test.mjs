import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { runInNewContext } from 'node:vm';
import test from 'node:test';
import { defaultMockInventoryReady, defaultMockHistoryReady } from './default-mock-readiness.mjs';

const integrationRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const root = path.resolve(integrationRoot, '..', '..');
const source = file => fs.readFileSync(path.join(integrationRoot, file), 'utf8');
const workflow = fs.readFileSync(process.env.PULSE_READINESS_TEST_WORKFLOW ||
  path.join(root, '.github/workflows/test-e2e.yml'), 'utf8');
const steps = workflow.split('\n  e2e:\n')[1].split('\n  agent-registration:\n')[0];
const step = name => steps.split(`      - name: ${name}\n`)[1]?.split('\n      - name:')[0];

const readyInventory = () => ({
  connectedInfrastructure: [{ name: 'esxi-01.lab.local' }],
  resources: [
    { name: 'nvme-primary', type: 'storage', sources: ['vmware'] },
    { type: 'k8s-cluster', sources: ['kubernetes'] },
    { name: 'tank', sources: ['truenas'] },
    { type: 'vm', sources: ['proxmox'] },
    { name: 'esxi-01.lab.local', type: 'agent', sources: ['vmware'] },
    { type: 'docker-host' }, { type: 'pbs' }, { type: 'pmg' },
  ],
});
const series = [{ timestamp: 1 }, { timestamp: 7 * 24 * 60 * 60 * 1000 }];
const readyHistory = () => ({ pools: { tank: { used: series } }, disks: { sdc: { temperature: series } } });

test('complete existing inventory passes, including the container alternative', () => {
  const state = readyInventory();
  assert.equal(defaultMockInventoryReady(state), true);
  state.resources[3].type = 'system-container';
  assert.equal(defaultMockInventoryReady(state), true);
});

test('each required resource and connected-host identity is necessary', () => {
  for (let i = 0; i < readyInventory().resources.length; i++) {
    const state = readyInventory();
    state.resources.splice(i, 1);
    assert.equal(defaultMockInventoryReady(state), false, `missing resource ${i}`);
  }
  const state = readyInventory();
  state.connectedInfrastructure = [{ name: 'another-host' }];
  assert.equal(defaultMockInventoryReady(state), false);
});

test('platform attribution and fixed names cannot borrow another source', () => {
  for (let i = 0; i < 5; i++) {
    const state = readyInventory();
    state.resources[i].sources = ['unrelated'];
    assert.equal(defaultMockInventoryReady(state), false);
  }
  for (const i of [0, 2, 4]) {
    const state = readyInventory();
    state.resources[i].name = 'unrelated';
    assert.equal(defaultMockInventoryReady(state), false);
  }
});

test('unknown or malformed inventory cannot pass admission', () => {
  for (const state of [null, undefined, {}, { resources: {} }, { resources: [null] }]) {
    assert.equal(defaultMockInventoryReady(state), false);
  }
  const state = readyInventory();
  state.resources[0].sources = 'vmware';
  assert.equal(defaultMockInventoryReady(state), false);
});

test('both pool and physical-disk history are required at the original depth', () => {
  assert.equal(defaultMockHistoryReady(readyHistory()), true);
  for (const charts of [null, undefined, {}, { pools: readyHistory().pools }, { disks: readyHistory().disks }]) {
    assert.equal(defaultMockHistoryReady(charts), false);
  }
  for (const points of [[], [{ timestamp: 1 }], [{ timestamp: 'bad' }, { timestamp: 700_000_000 }],
    [{ timestamp: 0 }, { timestamp: 5 * 24 * 60 * 60 * 1000 }]]) {
    for (const kind of ['pool', 'disk']) {
      const charts = readyHistory();
      if (kind === 'pool') charts.pools.tank.used = points;
      else charts.disks.sdc.temperature = points;
      assert.equal(defaultMockHistoryReady(charts), false, `${kind}: ${JSON.stringify(points)}`);
    }
  }
  const charts = readyHistory();
  charts.pools.tank.used = [...series].reverse();
  assert.equal(defaultMockHistoryReady(charts), true);
});

async function invokeAdmission(flag, check) {
  let invocation;
  // Execute the committed spec body with controlled browser/check callbacks,
  // not a copy of its orchestration. Real Playwright discovery is also checked
  // by the owning exact-source command.
  const body = source('fixture-readiness/default-mock-ready.spec.ts').replace(/^import .*;\n/gm, '');
  runInNewContext(body, {
    fs, os, path, Error,
    process: { env: { PULSE_E2E_REQUIRE_DEFAULT_MOCK_READY: flag } },
    test: (_name, callback) => { invocation = callback({ browser: 'owned-browser' }); },
    createAuthenticatedStorageState: check,
  });
  await invocation;
}

test('the real admission spec calls shared authentication once and clears temporary state', async () => {
  let calls = 0;
  let filename;
  await invokeAdmission('true', async (browser, statePath) => {
    calls++;
    filename = statePath;
    assert.equal(browser, 'owned-browser');
    fs.writeFileSync(statePath, 'controlled session');
  });
  assert.equal(calls, 1);
  assert.equal(fs.existsSync(path.dirname(filename)), false);
});

test('original inventory/history failure escapes unchanged and clears temporary state', async () => {
  for (const message of ['default mock inventory should be complete before browser fixtures start',
    'default mock history should cover the seven-day Core E2E chart window before browser fixtures start']) {
    let calls = 0;
    let filename;
    const failure = new Error(message);
    await assert.rejects(invokeAdmission('true', async (_browser, statePath) => {
      calls++;
      filename = statePath;
      fs.writeFileSync(statePath, 'partial controlled session');
      throw failure;
    }), err => err === failure);
    assert.equal(calls, 1);
    assert.equal(fs.existsSync(path.dirname(filename)), false);
  }
});

test('missing readiness opt-in fails instead of admitting unchecked fixtures', async () => {
  for (const flag of [undefined, '', 'false']) {
    let called = false;
    await assert.rejects(invokeAdmission(flag, async () => { called = true; }),
      /requires PULSE_E2E_REQUIRE_DEFAULT_MOCK_READY=true/);
    assert.equal(called, false);
  }
});

test('dedicated discovery cannot retry, absorb or mix admission with either tier', () => {
  const config = source('fixture-readiness.config.ts');
  for (const required of ["testDir: './fixture-readiness'", "testMatch: 'default-mock-ready.spec.ts'",
    'workers: 1', 'retries: 0', 'timeout: 360_000', 'globalTimeout: 420_000',
    'test-results-readiness/junit.xml', 'playwright-report-readiness']) {
    assert.ok(config.includes(required), required);
  }
  assert.ok(!config.includes('PULSE_E2E_TIER'));
  const helper = source('tests/helpers.ts');
  assert.match(helper, /defaultMockInventoryReady\(await response\.json\(\)\)/);
  assert.match(helper, /defaultMockHistoryReady\(await response\.json\(\)\)/);
  assert.match(helper, /timeout: 120_000/);
  assert.match(helper, /timeout: 180_000/);
});

test('workflow admission precedes both tiers without an ignored failure', () => {
  const admission = step('Admit the default mock runtime');
  assert.ok(admission, 'missing fixture admission');
  assert.match(admission, /PULSE_E2E_REQUIRE_DEFAULT_MOCK_READY: "true"/);
  assert.match(admission, /timeout-minutes: 8/);
  assert.match(admission, /working-directory: tests\/integration/);
  assert.doesNotMatch(admission, /continue-on-error|\n        if:|\|\|/);
  assert.ok(steps.indexOf('- name: Admit the default mock runtime') < steps.indexOf('- name: Run stable-tier E2E suite'));
});

test('actual workflow shell admits healthy fixtures and stops broken ones before suite work', () => {
  const command = step('Admit the default mock runtime')?.match(/\n        run: (.+)/)?.[1] || '';
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'pulse-admission-command-'));
  try {
    fs.writeFileSync(path.join(directory, 'npx'), '#!/bin/sh\n[ "$*" = "playwright test --config=fixture-readiness.config.ts" ] || exit 9\n' +
      'echo "controlled default mock readiness result"\nexit "$ADMISSION_EXIT"\n', { mode: 0o700 });
    for (const status of [0, 1]) {
      const marker = path.join(directory, `suite-${status}`);
      const result = spawnSync('bash', ['--noprofile', '--norc', '-e', '-o', 'pipefail', '-c',
        `${command}\nprintf suite-started > "${marker}"`], {
        cwd: integrationRoot, encoding: 'utf8', env: { ...process.env, PATH: `${directory}:${process.env.PATH}`, ADMISSION_EXIT: String(status) },
      });
      assert.equal(result.status, status, result.stdout + result.stderr);
      assert.equal(fs.existsSync(marker), status === 0);
    }
  } finally { fs.rmSync(directory, { recursive: true, force: true }); }
});

test('completed stable failure receipts are uploaded before probation can be cancelled', () => {
  for (const name of ['Upload Playwright report', 'Upload JUnit results']) {
    assert.ok(steps.indexOf(`- name: ${name}`) < steps.indexOf('- name: Run probation-tier E2E suite'), name);
    assert.match(step(name), /!cancelled\(\) && steps\.stable\.outcome == 'failure'/);
  }
  assert.match(step('Upload failed fixture admission'), /!cancelled\(\) && steps\.fixture\.outcome == 'failure'/);
  assert.match(step('Run probation-tier E2E suite (non-gating)'), /continue-on-error: true/);
  assert.ok(steps.includes('timeout-minutes: 60'));
});
