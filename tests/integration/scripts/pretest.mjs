import { spawn } from 'node:child_process';
import http from 'node:http';

// Add signal handlers to debug unexpected termination
const signals = ['SIGTERM', 'SIGINT', 'SIGHUP', 'SIGPIPE', 'SIGQUIT'];
signals.forEach(sig => {
  process.on(sig, () => {
    console.error(`[pretest] Received signal: ${sig}`);
    process.exit(128 + (process[sig] || 1));
  });
});

process.on('uncaughtException', (err) => {
  console.error('[pretest] Uncaught exception:', err);
  process.exit(1);
});

process.on('unhandledRejection', (reason, promise) => {
  console.error('[pretest] Unhandled rejection at:', promise, 'reason:', reason);
  process.exit(1);
});

const truthy = (value) => {
  if (!value) return false;
  return ['1', 'true', 'yes', 'on'].includes(String(value).trim().toLowerCase());
};

const shouldSkipDocker = truthy(process.env.PULSE_E2E_SKIP_DOCKER);
const shouldSkipPlaywrightInstall = truthy(process.env.PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL);

const DEFAULT_E2E_BOOTSTRAP_TOKEN = '0123456789abcdef0123456789abcdef0123456789abcdef';
if (!process.env.PULSE_E2E_BOOTSTRAP_TOKEN) {
  process.env.PULSE_E2E_BOOTSTRAP_TOKEN = DEFAULT_E2E_BOOTSTRAP_TOKEN;
}


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
  console.log(`[pretest] Waiting for ${healthURL} to become healthy...`);
  const startedAt = Date.now();
  let attempt = 0;

  const checkHealth = () => {
    return new Promise((resolve) => {
      const req = http.get(healthURL, (res) => {
        res.resume(); // Consume response data to free up memory
        resolve(res.statusCode >= 200 && res.statusCode < 300);
      });
      req.on('error', () => resolve(false));
      req.setTimeout(5000, () => {
        req.destroy();
        resolve(false);
      });
    });
  };

  while (Date.now() - startedAt < timeoutMs) {
    attempt++;
    try {
      const ok = await checkHealth();
      if (ok) {
        console.log(`[pretest] Health check passed after ${attempt} attempts`);
        return;
      }
    } catch {
      // ignore and retry
    }
    await new Promise((r) => setTimeout(r, 1000));
  }
  throw new Error(`Timed out waiting for ${healthURL} after ${attempt} attempts`);
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

console.log('[pretest] Starting docker compose...');
try {
  if (useDockerCompose) {
    console.log('[pretest] Using legacy docker-compose command');
    await run('docker-compose', legacyComposeArgs);
  } else {
    console.log('[pretest] Using modern docker compose command');
    await run('docker', composeArgs);
  }
  console.log('[pretest] Docker compose completed successfully');
} catch (error) {
  console.error('[pretest] Docker compose failed:', error.message);
  // Try to get container logs for debugging
  try {
    await run('docker', ['logs', 'pulse-test-server'], { stdio: 'inherit' });
  } catch {
    // ignore
  }
  process.exit(1);
}

const baseURL = (process.env.PULSE_BASE_URL || 'http://localhost:7655').replace(/\/+$/, '');
console.log(`[pretest] Waiting for health check at ${baseURL}/api/health...`);

try {
  await waitForHealth(`${baseURL}/api/health`);
  console.log('[pretest] Health check passed!');
} catch (error) {
  console.error('[pretest] Health check failed:', error.message);
  // Try to get container logs for debugging
  console.log('[pretest] Attempting to retrieve container logs...');
  try {
    console.log('[pretest] === pulse-test-server logs ===');
    await run('docker', ['logs', 'pulse-test-server'], { stdio: 'inherit' });
  } catch {
    console.log('[pretest] Could not retrieve pulse-test-server logs');
  }
  process.exit(1);
}
