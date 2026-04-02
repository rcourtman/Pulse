import { execFileSync } from 'node:child_process';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

import {
  resolveHostedTenantRootDataDir,
  runRemote,
  shellQuote,
} from './hosted-tenant-runtime.mjs';

const REPO_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

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

function runHostedRelayMobileTokenHelper({ args, cloudHost, tempDir }) {
  const localBinaryPath = buildLocalHelper(tempDir);
  const remoteBinaryPath = `/tmp/relay-mobile-token-helper-${process.pid}-${Date.now()}`;

  try {
    execFileSync('scp', [localBinaryPath, `${cloudHost}:${remoteBinaryPath}`], {
      encoding: 'utf8',
      maxBuffer: 32 * 1024 * 1024,
      stdio: 'pipe',
    });
    runRemote(cloudHost, `chmod +x ${shellQuote(remoteBinaryPath)}`);

    return runRemote(cloudHost, [
      shellQuote(remoteBinaryPath),
      ...args.map((value) => shellQuote(value)),
    ].join(' '));
  } finally {
    try {
      runRemote(cloudHost, `rm -f ${shellQuote(remoteBinaryPath)}`);
    } catch {}
  }
}

export function createHostedRelayMobileToken({ cloudHost, issuedVia = null, tenantId, tempDir }) {
  const args = [
    'create',
    '--data-dir',
    resolveHostedTenantRootDataDir(tenantId),
    '--org-id',
    tenantId,
  ];
  if (issuedVia) {
    args.push('--issued-via', issuedVia);
  }
  return JSON.parse(runHostedRelayMobileTokenHelper({ args, cloudHost, tempDir }));
}

export function deleteHostedRelayMobileToken({
  cloudHost,
  tenantId,
  tempDir,
  token = null,
  tokenId = null,
}) {
  const args = [
    'delete',
    '--data-dir',
    resolveHostedTenantRootDataDir(tenantId),
  ];
  if (tokenId) {
    args.push('--token-id', tokenId);
  }
  if (token) {
    args.push('--token', token);
  }
  return JSON.parse(runHostedRelayMobileTokenHelper({ args, cloudHost, tempDir }));
}

export function validateHostedRelayMobileToken({ cloudHost, tenantId, tempDir, token }) {
  const args = [
    'validate',
    '--data-dir',
    resolveHostedTenantRootDataDir(tenantId),
    '--token',
    token,
  ];
  return JSON.parse(runHostedRelayMobileTokenHelper({ args, cloudHost, tempDir }));
}
