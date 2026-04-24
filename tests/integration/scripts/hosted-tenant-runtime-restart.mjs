#!/usr/bin/env node

import process from 'node:process';
import { pathToFileURL } from 'node:url';

import { restartHostedTenantRuntime } from './hosted-tenant-runtime.mjs';

const DEFAULT_CLOUD_HOST = 'root@pulse-cloud';

function usage(message) {
  if (message) {
    console.error(`error: ${message}`);
    console.error('');
  }

  console.error(
    'usage: node ./tests/integration/scripts/hosted-tenant-runtime-restart.mjs --tenant-id <id> [--cloud-host <user@host>]',
  );
  process.exit(1);
}

export function parseArgs(argv) {
  const parsed = {
    cloudHost: DEFAULT_CLOUD_HOST,
    tenantId: '',
  };

  for (let index = 0; index < argv.length; index += 1) {
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
      case '--help':
      case '-h':
        usage();
        break;
      default:
        usage(`unsupported flag ${arg}`);
    }
  }

  parsed.cloudHost = String(parsed.cloudHost || '').trim();
  parsed.tenantId = String(parsed.tenantId || '').trim();

  if (!parsed.tenantId) {
    usage('--tenant-id is required');
  }
  if (!parsed.cloudHost) {
    usage('--cloud-host is required');
  }

  return parsed;
}

export function main(argv = process.argv.slice(2)) {
  const args = parseArgs(argv);
  restartHostedTenantRuntime(args.cloudHost, args.tenantId);
  process.stdout.write(`${JSON.stringify({
    cloudHost: args.cloudHost,
    restarted: true,
    tenantId: args.tenantId,
  }, null, 2)}\n`);
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
