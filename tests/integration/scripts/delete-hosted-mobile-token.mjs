#!/usr/bin/env node

import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import process from 'node:process';

import { deleteHostedRelayMobileToken } from './hosted-mobile-token-runtime.mjs';
import { restartHostedTenantRuntime } from './hosted-tenant-runtime.mjs';

const DEFAULT_CLOUD_HOST = 'root@pulse-cloud';

function usage(message) {
  if (message) {
    console.error(`error: ${message}`);
    console.error('');
  }

  console.error(
    'usage: node ./tests/integration/scripts/delete-hosted-mobile-token.mjs --tenant-id <id> [--token-id <id>] [--token <raw-token>] [--cloud-host <user@host>]',
  );
  process.exit(1);
}

function parseArgs(argv) {
  const parsed = {
    cloudHost: DEFAULT_CLOUD_HOST,
    tenantId: '',
    token: '',
    tokenId: '',
  };

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    switch (arg) {
      case '--tenant-id':
        parsed.tenantId = argv[index + 1] ?? usage('missing value for --tenant-id');
        index += 1;
        break;
      case '--token-id':
        parsed.tokenId = argv[index + 1] ?? usage('missing value for --token-id');
        index += 1;
        break;
      case '--token':
        parsed.token = argv[index + 1] ?? usage('missing value for --token');
        index += 1;
        break;
      case '--cloud-host':
        parsed.cloudHost = argv[index + 1] ?? usage('missing value for --cloud-host');
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

  parsed.tenantId = String(parsed.tenantId).trim();
  parsed.tokenId = String(parsed.tokenId).trim();
  parsed.token = String(parsed.token).trim();

  if (!parsed.tenantId) {
    usage('--tenant-id is required');
  }
  if (!parsed.tokenId && !parsed.token) {
    usage('either --token-id or --token is required');
  }

  return parsed;
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'pulse-hosted-mobile-token-delete-'));

  try {
    const result = deleteHostedRelayMobileToken({
      cloudHost: args.cloudHost,
      tenantId: args.tenantId,
      tempDir,
      token: args.token || null,
      tokenId: args.tokenId || null,
    });
    restartHostedTenantRuntime(args.cloudHost, args.tenantId);
    result.runtimeRestarted = true;
    process.stdout.write(`${JSON.stringify(result, null, 2)}\n`);
  } finally {
    fs.rmSync(tempDir, { force: true, recursive: true });
  }
}

main();
