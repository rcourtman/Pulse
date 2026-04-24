#!/usr/bin/env node

import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

import {
  restartHostedTenantRuntime,
  shellQuote,
} from './hosted-tenant-runtime.mjs';
import { createHostedRelayMobileToken } from './hosted-mobile-token-runtime.mjs';

const DEFAULT_POLL_INTERVAL_MS = 500;
const DEFAULT_POLL_TIMEOUT_MS = 15_000;
const SCRIPT_PATH = fileURLToPath(import.meta.url);

function usage(message) {
  if (message) {
    console.error(`error: ${message}`);
    console.error('');
  }

  console.error(
    'usage: node ./tests/integration/scripts/bootstrap-hosted-mobile-onboarding.mjs --tenant-id <id> --cloud-host <user@host> --control-plane-url <url> [--email <email>] [--poll-timeout-ms <ms>] [--poll-interval-ms <ms>]',
  );
  process.exit(1);
}

function parseArgs(argv) {
  const parsed = {
    cloudHost: '',
    controlPlaneUrl: '',
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
  if (!parsed.cloudHost) {
    usage('--cloud-host is required');
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

export function deriveTenantBaseUrl(controlPlaneUrl, tenantId) {
  const parsed = new URL(controlPlaneUrl);
  parsed.hostname = `${tenantId}.${parsed.hostname}`;
  parsed.pathname = '';
  parsed.search = '';
  parsed.hash = '';
  return parsed.toString().replace(/\/$/, '');
}

export function redactBearerTokens(value) {
  return String(value).replace(/Bearer\s+[A-Za-z0-9._~+/-]+/g, 'Bearer [REDACTED]');
}

export async function fetchOnboardingPayload({ fetchImpl = globalThis.fetch, rawToken, tenantBaseUrl }) {
  if (typeof fetchImpl !== 'function') {
    throw new Error('fetch is not available in this Node runtime');
  }

  const url = `${tenantBaseUrl}/api/onboarding/qr`;
  let response;
  try {
    response = await fetchImpl(url, {
      headers: {
        Authorization: `Bearer ${rawToken}`,
      },
    });
  } catch (error) {
    throw new Error(`failed to fetch hosted onboarding payload from ${tenantBaseUrl}: ${redactBearerTokens(error instanceof Error ? error.message : String(error))}`);
  }

  const body = await response.text();
  if (!response.ok) {
    throw new Error(`hosted onboarding payload request returned HTTP ${response.status} from ${tenantBaseUrl}: ${redactBearerTokens(body.slice(0, 500))}`);
  }

  try {
    return JSON.parse(body);
  } catch (error) {
    throw new Error(`hosted onboarding payload from ${tenantBaseUrl} was not valid JSON: ${redactBearerTokens(error instanceof Error ? error.message : String(error))}`);
  }
}

const REMOTE_ONBOARDING_FETCH_PY = `
import json
import sys
import urllib.error
import urllib.request

base_url = sys.argv[1].rstrip("/")
token = sys.stdin.read().strip()
request = urllib.request.Request(
    base_url + "/api/onboarding/qr",
    headers={"Authorization": "Bearer " + token},
)
try:
    with urllib.request.urlopen(request, timeout=20) as response:
        sys.stdout.write(response.read().decode("utf-8"))
except urllib.error.HTTPError as exc:
    body = exc.read().decode("utf-8", errors="replace")[:500]
    sys.stderr.write(json.dumps({"status": exc.code, "body": body}))
    sys.exit(22)
except Exception as exc:
    sys.stderr.write(str(exc))
    sys.exit(1)
`;

export function fetchOnboardingPayloadViaCloudHost({
  cloudHost,
  rawToken,
  runner = execFileSync,
  tenantBaseUrl,
}) {
  const host = String(cloudHost ?? '').trim();
  if (!host) {
    throw new Error('cloud host is required for hosted onboarding remote fetch');
  }
  const command = `python3 -c ${shellQuote(REMOTE_ONBOARDING_FETCH_PY)} ${shellQuote(tenantBaseUrl)}`;
  let output;
  try {
    output = runner('ssh', [host, command], {
      encoding: 'utf8',
      input: rawToken,
      maxBuffer: 32 * 1024 * 1024,
      stdio: ['pipe', 'pipe', 'pipe'],
    });
  } catch (error) {
    const detail = ['stderr', 'stdout']
      .map((field) => error?.[field])
      .filter((value) => value !== undefined && value !== null && String(value).trim() !== '')
      .map((value) => String(value).trim())
      .join('\n') || (error instanceof Error ? error.message : String(error));
    throw new Error(`failed to fetch hosted onboarding payload via ${host}: ${redactBearerTokens(detail)}`);
  }

  try {
    return JSON.parse(output);
  } catch (error) {
    throw new Error(`hosted onboarding payload fetched via ${host} was not valid JSON: ${redactBearerTokens(error instanceof Error ? error.message : String(error))}`);
  }
}

function sleep(ms) {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms);
}

async function pollOnboardingPayload({
  cloudHost,
  fetchImpl,
  pollIntervalMs,
  pollTimeoutMs,
  rawToken,
  tenantBaseUrl,
}) {
  const deadline = Date.now() + pollTimeoutMs;
  let lastError = null;

  while (Date.now() <= deadline) {
    try {
      return await fetchOnboardingPayload({ fetchImpl, rawToken, tenantBaseUrl });
    } catch (error) {
      lastError = error;
      if (String(cloudHost ?? '').trim() !== '') {
        try {
          return fetchOnboardingPayloadViaCloudHost({ cloudHost, rawToken, tenantBaseUrl });
        } catch (remoteError) {
          lastError = remoteError;
        }
      }
      if (Date.now() >= deadline) {
        break;
      }
      sleep(pollIntervalMs);
    }
  }

  throw new Error(`timed out waiting for hosted onboarding payload from ${tenantBaseUrl}: ${String(lastError instanceof Error ? lastError.message : lastError)}`);
}

async function main() {
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
    const qrPayload = await pollOnboardingPayload({
      cloudHost: args.cloudHost,
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

if (process.argv[1] && path.resolve(process.argv[1]) === SCRIPT_PATH) {
  main().catch((error) => {
    console.error(redactBearerTokens(error instanceof Error ? error.message : String(error)));
    process.exit(1);
  });
}
