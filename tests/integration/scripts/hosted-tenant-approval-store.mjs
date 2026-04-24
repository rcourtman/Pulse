#!/usr/bin/env node

import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import process from 'node:process';
import { pathToFileURL } from 'node:url';

import {
  assertHostedTenantRuntimeExists,
  resolveHostedTenantRootDataDir,
  restartHostedTenantRuntime,
  runRemote,
  shellQuote,
} from './hosted-tenant-runtime.mjs';

const repoRoot = path.resolve(path.dirname(new URL(import.meta.url).pathname), '..', '..', '..');
function usage(message) {
  if (message) {
    console.error(`error: ${message}`);
    console.error('');
  }

  console.error(
    'usage: node ./tests/integration/scripts/hosted-tenant-approval-store.mjs <create|get> --tenant-id <id> [--org-id <id>] [--cloud-host <host>] [--no-restart] [approval options]',
  );
  process.exit(1);
}

export function parseArgs(argv) {
  if (argv.length === 0) {
    usage('missing action');
  }

  const parsed = {
    action: argv[0],
    cloudHost: 'root@pulse-cloud',
    orgId: '',
    restartAfterCreate: true,
    tenantId: '',
    passthrough: [],
  };

  for (let index = 1; index < argv.length; index += 1) {
    const arg = argv[index];
    switch (arg) {
      case '--cloud-host':
        parsed.cloudHost = argv[index + 1] ?? usage('missing value for --cloud-host');
        index += 1;
        break;
      case '--tenant-id':
        parsed.tenantId = argv[index + 1] ?? usage('missing value for --tenant-id');
        index += 1;
        break;
      case '--org-id':
        parsed.orgId = argv[index + 1] ?? usage('missing value for --org-id');
        index += 1;
        break;
      case '--no-restart':
      case '--skip-restart':
        parsed.restartAfterCreate = false;
        break;
      case '--help':
      case '-h':
        usage();
        break;
      default:
        parsed.passthrough.push(arg);
    }
  }

  if (!['create', 'get'].includes(parsed.action)) {
    usage(`unsupported action ${parsed.action}`);
  }
  if (!parsed.tenantId) {
    usage('--tenant-id is required');
  }
  if (parsed.orgId === '') {
    parsed.orgId = parsed.tenantId;
  }

  return parsed;
}

export function resolveHostedTenantApprovalStoreDataDir(tenantId) {
  return resolveHostedTenantRootDataDir(tenantId);
}

function buildLocalHelper(tempDir) {
  const binaryPath = path.join(tempDir, 'approval-store-helper');
  execFileSync('go', [
    'build',
    '-buildvcs=false',
    '-o',
    binaryPath,
    './tests/integration/scripts/approval-store-helper',
  ], {
    cwd: repoRoot,
    env: {
      ...process.env,
      CGO_ENABLED: '0',
      GOARCH: 'amd64',
      GOOS: 'linux',
    },
    encoding: 'utf8',
    stdio: 'pipe',
  });
  return binaryPath;
}

function main() {
  const args = parseArgs(process.argv.slice(2));

  if (args.action === 'create') {
    assertHostedTenantRuntimeExists(args.cloudHost, args.tenantId);
  }

  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'pulse-hosted-approval-helper-'));
  const localBinaryPath = buildLocalHelper(tempDir);
  const remoteBinaryPath = `/tmp/approval-store-helper-${process.pid}-${Date.now()}`;
  const tenantDataDir = resolveHostedTenantApprovalStoreDataDir(args.tenantId);

  try {
    execFileSync('scp', [localBinaryPath, `${args.cloudHost}:${remoteBinaryPath}`], {
      encoding: 'utf8',
      stdio: 'pipe',
      maxBuffer: 32 * 1024 * 1024,
    });
    runRemote(args.cloudHost, `chmod +x ${shellQuote(remoteBinaryPath)}`);

    const remoteArgs = [
      shellQuote(remoteBinaryPath),
      args.action,
      '--data-dir',
      shellQuote(tenantDataDir),
      '--org-id',
      shellQuote(args.orgId),
      ...args.passthrough.map(shellQuote),
    ];
    const output = runRemote(args.cloudHost, remoteArgs.join(' '));
    const payload = JSON.parse(output);

    if (args.action === 'create' && args.restartAfterCreate) {
      // Approval store state is loaded in-memory at runtime startup. Hosted proof
      // seeding edits the backing file out-of-band, so restart the tenant runtime
      // before returning to ensure the live process serves the seeded approval.
      restartHostedTenantRuntime(args.cloudHost, args.tenantId);
      payload.runtimeRestarted = true;
    } else if (args.action === 'create') {
      payload.runtimeRestarted = false;
    }

    process.stdout.write(`${JSON.stringify(payload, null, 2)}\n`);
  } finally {
    try {
      runRemote(args.cloudHost, `rm -f ${shellQuote(remoteBinaryPath)}`);
    } catch {}
    fs.rmSync(tempDir, { recursive: true, force: true });
  }
}

const invokedAsScript = process.argv[1]
  && import.meta.url === pathToFileURL(process.argv[1]).href;

if (invokedAsScript) {
  try {
    main();
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exit(1);
  }
}
