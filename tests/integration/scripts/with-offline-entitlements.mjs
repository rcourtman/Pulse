// Run the complete managed-local E2E lifecycle with an in-memory issuer.
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { spawn } from 'node:child_process';
import { stopManagedLocalBackend } from './managed-local-backend.mjs';
import { startOfflineLicenseIssuer } from './offline-license-issuer.mjs';

const [command, ...args] = process.argv.slice(2);
if (!command) throw new Error('Usage: node scripts/with-offline-entitlements.mjs <command> [args...]');
if (process.env.PULSE_E2E_USE_LOCAL_BACKEND !== 'true') {
  throw new Error('Offline issuer requires PULSE_E2E_USE_LOCAL_BACKEND=true; not Docker, remote or release binaries');
}
if (process.env.PULSE_E2E_LOCAL_BACKEND_BINARY || process.env.PULSE_E2E_LOCAL_BACKEND_VARIANT) {
  throw new Error('Use the source-built Community managed backend, not a supplied binary/runtime');
}
for (const key of ['PULSE_BASE_URL', 'PULSE_E2E_USE_HOT_DEV', 'PULSE_E2E_SKIP_DOCKER', 'PULSE_E2E_RUNTIME_STATE_PATH', 'GOFLAGS']) {
  if (process.env[key]) throw new Error(`Offline managed lifecycle does not accept ${key}`);
}
const binaryRoot = await mkdtemp(path.join(tmpdir(), 'pulse-offline-e2e-'));
const issuer = await startOfflineLicenseIssuer();
const runtimeEnv = { ...process.env, PULSE_E2E_RUNTIME_STATE_PATH: path.join(binaryRoot, 'runtime.json') };
try {
  const child = spawn(command, args, { stdio: 'inherit', env: { ...runtimeEnv,
    PULSE_E2E_LOCAL_BACKEND_BINARY: path.join(binaryRoot, 'pulse'),
    PULSE_E2E_LOCAL_BACKEND_HOST: '127.0.0.1',
    PULSE_LICENSE_PUBLIC_KEY: issuer.publicKey, PULSE_LICENSE_SERVER_URL: issuer.url,
    PULSE_LICENSE_DEV_MODE: 'false', PULSE_MOCK_MODE: 'false', PULSE_MULTI_TENANT_ENABLED: 'true',
    PULSE_E2E_OFFLINE_ACTIVATION_KEY: issuer.activationKey,
  } });
  const forward = signal => child.kill(signal);
  const term = () => forward('SIGTERM');
  const interrupt = () => forward('SIGINT');
  process.on('SIGTERM', term); process.on('SIGINT', interrupt);
  try {
    process.exitCode = await new Promise((resolve, reject) => {
      child.once('error', reject);
      child.once('exit', (code, signal) => resolve(code ?? (signal === 'SIGINT' ? 130 : 143)));
    });
  } finally {
    process.off('SIGTERM', term); process.off('SIGINT', interrupt);
  }
} finally {
  try { await stopManagedLocalBackend({ env: runtimeEnv }); }
  finally { await issuer.close(); await rm(binaryRoot, { recursive: true, force: true }); }
}
