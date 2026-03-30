#!/usr/bin/env node

import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import process from 'node:process';

const repoRoot = path.resolve(path.dirname(new URL(import.meta.url).pathname), '..', '..', '..');
function usage(message) {
  if (message) {
    console.error(`error: ${message}`);
    console.error('');
  }

  console.error(
    'usage: node ./tests/integration/scripts/hosted-tenant-approval-store.mjs <create|get> --tenant-id <id> [--org-id <id>] [--cloud-host <host>] [approval options]',
  );
  process.exit(1);
}

function shellQuote(value) {
  return `'${String(value).replace(/'/g, `'\\''`)}'`;
}

function parseArgs(argv) {
  if (argv.length === 0) {
    usage('missing action');
  }

  const parsed = {
    action: argv[0],
    cloudHost: 'root@pulse-cloud',
    orgId: '',
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

function resolveTenantDataDir(tenantId, orgId) {
  const scopedOrgId = String(orgId || tenantId).trim();
  if (scopedOrgId === '' || scopedOrgId === 'default') {
    return `/data/tenants/${tenantId}`;
  }
  return `/data/tenants/${tenantId}/orgs/${scopedOrgId}`;
}

function buildLocalHelper(tempDir) {
  const binaryPath = path.join(tempDir, 'approval-store-helper');
  execFileSync('go', [
    'build',
    '-buildvcs=false',
    '-o',
    binaryPath,
    './tests/integration/scripts/approval-store-helper.go',
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

function runRemote(cloudHost, command) {
  return execFileSync('ssh', [cloudHost, command], {
    encoding: 'utf8',
    stdio: 'pipe',
    maxBuffer: 32 * 1024 * 1024,
  });
}

function restartHostedTenantRuntime(cloudHost, tenantId) {
  const containerName = `pulse-${tenantId}`;
  const script = `
set -eu
container=${shellQuote(containerName)}
docker restart "$container" >/dev/null
deadline=$(( $(date +%s) + 60 ))
while [ "$(date +%s)" -lt "$deadline" ]; do
  state="$(docker inspect --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$container" 2>/dev/null || true)"
  case "$state" in
    "running healthy"|"running none")
      exit 0
      ;;
  esac
  sleep 1
done
echo "timed out waiting for $container to become ready" >&2
docker inspect --format '{{json .State}}' "$container" >&2 || true
exit 1
`;
  runRemote(cloudHost, `sh -lc ${shellQuote(script)}`);
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'pulse-hosted-approval-helper-'));
  const localBinaryPath = buildLocalHelper(tempDir);
  const remoteBinaryPath = `/tmp/approval-store-helper-${process.pid}-${Date.now()}`;
  const tenantDataDir = resolveTenantDataDir(args.tenantId, args.orgId);

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

    if (args.action === 'create') {
      // Approval store state is loaded in-memory at runtime startup. Hosted proof
      // seeding edits the backing file out-of-band, so restart the tenant runtime
      // before returning to ensure the live process serves the seeded approval.
      restartHostedTenantRuntime(args.cloudHost, args.tenantId);
      payload.runtimeRestarted = true;
    }

    process.stdout.write(`${JSON.stringify(payload, null, 2)}\n`);
  } finally {
    try {
      runRemote(args.cloudHost, `rm -f ${shellQuote(remoteBinaryPath)}`);
    } catch {}
    fs.rmSync(tempDir, { recursive: true, force: true });
  }
}

main();
