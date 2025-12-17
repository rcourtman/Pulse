import { spawn } from 'node:child_process';

const truthy = (value) => {
  if (!value) return false;
  return ['1', 'true', 'yes', 'on'].includes(String(value).trim().toLowerCase());
};

if (truthy(process.env.PULSE_E2E_SKIP_DOCKER)) {
  console.log('[integration] PULSE_E2E_SKIP_DOCKER enabled, skipping docker compose down');
  process.exit(0);
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

const canRun = async (command, args) => {
  try {
    await run(command, args, { stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
};

const useDockerCompose = !(await canRun('docker', ['compose', 'version']));

try {
  if (useDockerCompose) {
    await run('docker-compose', ['-f', 'docker-compose.test.yml', 'down', '-v']);
  } else {
    await run('docker', ['compose', '-f', 'docker-compose.test.yml', 'down', '-v']);
  }
} catch (err) {
  // Avoid masking test failures with cleanup errors
  console.warn('[integration] docker compose down failed:', err?.message || err);
}

