import { spawn } from 'node:child_process';

const truthy = (value) => {
  if (!value) return false;
  return ['1', 'true', 'yes', 'on'].includes(String(value).trim().toLowerCase());
};

const shouldSkipDocker = truthy(process.env.PULSE_E2E_SKIP_DOCKER);
const shouldSkipPlaywrightInstall = truthy(process.env.PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL);

const run = (command, args, options = {}) =>
  new Promise((resolve, reject) => {
    const child = spawn(command, args, { stdio: 'inherit', ...options });
    child.on('error', reject);
    child.on('close', (code) => {
      if (code === 0) resolve();
      else reject(new Error(`${command} ${args.join(' ')} exited with code ${code}`));
    });
  });

const npxCmd = process.platform === 'win32' ? 'npx.cmd' : 'npx';

const canRun = async (command, args) => {
  try {
    await run(command, args, { stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
};

const waitForHealth = async (healthURL, timeoutMs = 120_000) => {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    try {
      const res = await fetch(healthURL, { method: 'GET' });
      if (res.ok) return;
    } catch {
      // ignore and retry
    }
    await new Promise((r) => setTimeout(r, 1000));
  }
  throw new Error(`Timed out waiting for ${healthURL}`);
};

if (truthy(process.env.PULSE_E2E_INSECURE_TLS)) {
  process.env.NODE_TLS_REJECT_UNAUTHORIZED = '0';
}

if (!shouldSkipPlaywrightInstall) {
  await run(npxCmd, ['playwright', 'install', 'chromium']);
}

if (shouldSkipDocker) {
  console.log('[integration] PULSE_E2E_SKIP_DOCKER enabled, skipping docker compose up');
  process.exit(0);
}

const composeArgs = ['compose', '-f', 'docker-compose.test.yml', 'up', '-d'];
const legacyComposeArgs = ['-f', 'docker-compose.test.yml', 'up', '-d'];
const useDockerCompose = !(await canRun('docker', ['compose', 'version']));

if (useDockerCompose) {
  await run('docker-compose', legacyComposeArgs);
} else {
  await run('docker', composeArgs);
}

const baseURL = (process.env.PULSE_BASE_URL || 'http://localhost:7655').replace(/\/+$/, '');
await waitForHealth(`${baseURL}/api/health`);
