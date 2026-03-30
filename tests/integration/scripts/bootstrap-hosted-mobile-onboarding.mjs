#!/usr/bin/env node

import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import process from 'node:process';

import {
  resolveHostedTenantRootDataDir,
  restartHostedTenantRuntime,
  runRemote,
  shellQuote,
} from './hosted-tenant-runtime.mjs';

const DEFAULT_CLOUD_HOST = 'root@pulse-cloud';
const DEFAULT_CONTROL_PLANE_URL = 'https://cloud.pulserelay.pro';
const DEFAULT_POLL_INTERVAL_MS = 500;
const DEFAULT_POLL_TIMEOUT_MS = 15_000;
const REPO_ROOT = path.resolve(path.dirname(new URL(import.meta.url).pathname), '..', '..', '..');

function usage(message) {
  if (message) {
    console.error(`error: ${message}`);
    console.error('');
  }

  console.error(
    'usage: node ./tests/integration/scripts/bootstrap-hosted-mobile-onboarding.mjs --tenant-id <id> [--email <email>] [--cloud-host <user@host>] [--control-plane-url <url>] [--poll-timeout-ms <ms>] [--poll-interval-ms <ms>]',
  );
  process.exit(1);
}

function parseArgs(argv) {
  const parsed = {
    cloudHost: DEFAULT_CLOUD_HOST,
    controlPlaneUrl: DEFAULT_CONTROL_PLANE_URL,
    email: '',
    pollIntervalMs: DEFAULT_POLL_INTERVAL_MS,
    pollTimeoutMs: DEFAULT_POLL_TIMEOUT_MS,
    tenantId: '',
  };

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    switch (arg) {
      case '--tenant-id':
        parsed.tenantId = argv[index + 1] ?? usage('missing value for --tenant-id');
        index += 1;
        break;
      case '--email':
        parsed.email = argv[index + 1] ?? usage('missing value for --email');
        index += 1;
        break;
      case '--cloud-host':
        parsed.cloudHost = argv[index + 1] ?? usage('missing value for --cloud-host');
        index += 1;
        break;
      case '--control-plane-url':
        parsed.controlPlaneUrl = argv[index + 1] ?? usage('missing value for --control-plane-url');
        index += 1;
        break;
      case '--poll-interval-ms':
        parsed.pollIntervalMs = Number(argv[index + 1] ?? usage('missing value for --poll-interval-ms'));
        index += 1;
        break;
      case '--poll-timeout-ms':
        parsed.pollTimeoutMs = Number(argv[index + 1] ?? usage('missing value for --poll-timeout-ms'));
        index += 1;
        break;
      case '--help':
      case '-h':
        usage();
        break;
      default:
        usage(`unsupported flag ${arg}`);
    }
  }

  parsed.controlPlaneUrl = String(parsed.controlPlaneUrl).trim().replace(/\/+$/, '');
  parsed.email = String(parsed.email).trim().toLowerCase();
  parsed.tenantId = String(parsed.tenantId).trim();

  if (!parsed.tenantId) {
    usage('--tenant-id is required');
  }
  if (!parsed.controlPlaneUrl) {
    usage('--control-plane-url is required');
  }
  if (!Number.isFinite(parsed.pollIntervalMs) || parsed.pollIntervalMs < 1) {
    usage('--poll-interval-ms must be a positive number');
  }
  if (!Number.isFinite(parsed.pollTimeoutMs) || parsed.pollTimeoutMs < 1) {
    usage('--poll-timeout-ms must be a positive number');
  }

  return parsed;
}

function runText(command, args, options = {}) {
  return execFileSync(command, args, {
    encoding: 'utf8',
    maxBuffer: 32 * 1024 * 1024,
    stdio: 'pipe',
    ...options,
  });
}

function buildLocalHelper(tempDir) {
  const binaryPath = path.join(tempDir, 'relay-mobile-token-helper');
  execFileSync('go', [
    'build',
    '-buildvcs=false',
    '-o',
    binaryPath,
    './tests/integration/scripts/relay-mobile-token-helper.go',
  ], {
    cwd: REPO_ROOT,
    encoding: 'utf8',
    env: {
      ...process.env,
      CGO_ENABLED: '0',
      GOARCH: 'amd64',
      GOOS: 'linux',
    },
    stdio: 'pipe',
  });
  return binaryPath;
}

function deriveTenantBaseUrl(controlPlaneUrl, tenantId) {
  const parsed = new URL(controlPlaneUrl);
  parsed.hostname = `${tenantId}.${parsed.hostname}`;
  parsed.pathname = '';
  parsed.search = '';
  parsed.hash = '';
  return parsed.toString().replace(/\/$/, '');
}

function createHostedRelayMobileToken({ cloudHost, tenantId, tempDir }) {
  const localBinaryPath = buildLocalHelper(tempDir);
  const remoteBinaryPath = `/tmp/relay-mobile-token-helper-${process.pid}-${Date.now()}`;
  const tenantDataDir = resolveHostedTenantRootDataDir(tenantId);

  try {
    execFileSync('scp', [localBinaryPath, `${cloudHost}:${remoteBinaryPath}`], {
      encoding: 'utf8',
      maxBuffer: 32 * 1024 * 1024,
      stdio: 'pipe',
    });
    runRemote(cloudHost, `chmod +x ${shellQuote(remoteBinaryPath)}`);

    const output = runRemote(cloudHost, [
      shellQuote(remoteBinaryPath),
      'create',
      '--data-dir',
      shellQuote(tenantDataDir),
      '--org-id',
      shellQuote(tenantId),
    ].join(' '));

    return JSON.parse(output);
  } finally {
    try {
      runRemote(cloudHost, `rm -f ${shellQuote(remoteBinaryPath)}`);
    } catch {}
  }
}

function curlJson(args) {
  return JSON.parse(runText('curl', args));
}

function fetchOnboardingPayload({ rawToken, tenantBaseUrl }) {
  return curlJson([
    '-fsS',
    '-H',
    `Authorization: Bearer ${rawToken}`,
    `${tenantBaseUrl}/api/onboarding/qr`,
  ]);
}

function sleep(ms) {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms);
}

function pollOnboardingPayload({
  pollIntervalMs,
  pollTimeoutMs,
  rawToken,
  tenantBaseUrl,
}) {
  const deadline = Date.now() + pollTimeoutMs;
  let lastError = null;

  while (Date.now() <= deadline) {
    try {
      return fetchOnboardingPayload({ rawToken, tenantBaseUrl });
    } catch (error) {
      lastError = error;
      if (Date.now() >= deadline) {
        break;
      }
      sleep(pollIntervalMs);
    }
  }

  throw new Error(`timed out waiting for hosted onboarding payload from ${tenantBaseUrl}: ${String(lastError instanceof Error ? lastError.message : lastError)}`);
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'pulse-hosted-mobile-onboarding-'));

  try {
    const tokenPayload = createHostedRelayMobileToken({
      cloudHost: args.cloudHost,
      tenantId: args.tenantId,
      tempDir,
    });
    const rawToken = String(tokenPayload?.token ?? '').trim();
    if (!rawToken) {
      throw new Error('hosted relay-mobile token helper did not return a raw token');
    }

    restartHostedTenantRuntime(args.cloudHost, args.tenantId);

    const tenantBaseUrl = deriveTenantBaseUrl(args.controlPlaneUrl, args.tenantId);
    const qrPayload = pollOnboardingPayload({
      pollIntervalMs: args.pollIntervalMs,
      pollTimeoutMs: args.pollTimeoutMs,
      rawToken,
      tenantBaseUrl,
    });

    const instanceId = String(qrPayload?.instance_id ?? '').trim();
    const deepLink = String(qrPayload?.deep_link ?? '').trim();
    const relayUrl = String(qrPayload?.relay?.url ?? '').trim();

    if (!instanceId) {
      throw new Error(`hosted tenant ${args.tenantId} does not currently expose a relay instance_id`);
    }
    if (!deepLink) {
      throw new Error(`hosted tenant ${args.tenantId} did not return an onboarding deep link`);
    }
    if (qrPayload?.relay?.enabled !== true) {
      throw new Error(`hosted tenant ${args.tenantId} relay is not enabled for mobile onboarding`);
    }

    process.stdout.write(`${JSON.stringify({
      controlPlaneUrl: args.controlPlaneUrl,
      deepLink,
      email: args.email,
      instanceId,
      relayConnected: instanceId !== '',
      relayStatus: {
        diagnostics: Array.isArray(qrPayload?.diagnostics) ? qrPayload.diagnostics : [],
        enabled: qrPayload?.relay?.enabled === true,
        instanceId,
      },
      relayUrl,
      tenantBaseUrl,
      tenantId: args.tenantId,
      tokenId: tokenPayload?.record?.id ?? null,
      runtimeRestarted: true,
    }, null, 2)}\n`);
  } finally {
    fs.rmSync(tempDir, { force: true, recursive: true });
  }
}

main();
