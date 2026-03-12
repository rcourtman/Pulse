#!/usr/bin/env node

import { spawn } from 'node:child_process';

const nodeCmd = process.execPath;
const npxCmd = process.platform === 'win32' ? 'npx.cmd' : 'npx';
const playwrightArgs = process.argv.slice(2);

const run = (command, args, options = {}) =>
  new Promise((resolve, reject) => {
    const child = spawn(command, args, { stdio: 'inherit', ...options });
    child.on('error', reject);
    child.on('close', (code) => resolve(code ?? 1));
  });

let exitCode = 0;

try {
  const pretestCode = await run(nodeCmd, ['./scripts/pretest.mjs']);
  if (pretestCode !== 0) {
    process.exit(pretestCode);
  }

  exitCode = await run(npxCmd, ['playwright', 'test', ...playwrightArgs]);
} finally {
  const posttestCode = await run(nodeCmd, ['./scripts/posttest.mjs']);
  if (exitCode === 0 && posttestCode !== 0) {
    exitCode = posttestCode;
  }
}

process.exit(exitCode);
