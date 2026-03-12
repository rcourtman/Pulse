import fs from 'node:fs/promises';
import path from 'node:path';
import { spawn } from 'node:child_process';

const truthyValues = new Set(['1', 'true', 'yes', 'on']);

const BILLING_PROFILES = {
  'multi-tenant': {
    capabilities: [
      'advanced_reporting',
      'advanced_sso',
      'agent_profiles',
      'ai_alerts',
      'ai_autofix',
      'ai_patrol',
      'audit_logging',
      'kubernetes_ai',
      'long_term_metrics',
      'mobile_app',
      'multi_tenant',
      'multi_user',
      'push_notifications',
      'rbac',
      'relay',
      'sso',
      'unlimited',
      'update_alerts',
      'white_label',
    ],
    limits: {},
    meters_enabled: [],
    plan_version: 'enterprise_eval',
    subscription_state: 'active',
    integrity: '',
  },
  infra: {
    capabilities: [
      'advanced_reporting',
      'advanced_sso',
      'agent_profiles',
      'ai_alerts',
      'ai_autofix',
      'ai_patrol',
      'audit_logging',
      'kubernetes_ai',
      'long_term_metrics',
      'mobile_app',
      'push_notifications',
      'rbac',
      'relay',
      'sso',
      'update_alerts',
    ],
    limits: {},
    meters_enabled: [],
    plan_version: 'pro_eval',
    subscription_state: 'active',
    integrity: '',
  },
};

const trim = (value) => String(value || '').trim();

export const truthy = (value) => truthyValues.has(trim(value).toLowerCase());

const shellQuote = (value) => `'${String(value).replace(/'/g, `'\"'\"'`)}'`;

export function resolveEntitlementProfile(env = process.env) {
  const explicitProfile = trim(env.PULSE_E2E_ENTITLEMENT_PROFILE);
  if (explicitProfile !== '') {
    return { profile: explicitProfile, explicit: true };
  }
  if (truthy(env.PULSE_MULTI_TENANT_ENABLED)) {
    return { profile: 'multi-tenant', explicit: false };
  }
  return { profile: '', explicit: false };
}

export function buildBillingState(profile) {
  const preset = BILLING_PROFILES[profile];
  if (!preset) {
    throw new Error(
      `Unsupported entitlement profile ${JSON.stringify(profile)}. Supported profiles: ${Object.keys(BILLING_PROFILES).join(', ')}`,
    );
  }
  return JSON.parse(JSON.stringify(preset));
}

export function resolveEntitlementTarget(env = process.env) {
  const writeCommand = trim(env.PULSE_E2E_ENTITLEMENT_WRITE_COMMAND);
  if (writeCommand !== '') {
    return { kind: 'command', command: writeCommand };
  }

  const billingPath = trim(env.PULSE_E2E_BILLING_STATE_PATH);
  if (billingPath !== '') {
    return { kind: 'file', path: billingPath };
  }

  if (!truthy(env.PULSE_E2E_SKIP_DOCKER)) {
    return {
      kind: 'docker',
      container: trim(env.PULSE_E2E_PULSE_CONTAINER) || 'pulse-test-server',
      path: trim(env.PULSE_E2E_CONTAINER_BILLING_PATH) || '/data/billing.json',
    };
  }

  return null;
}

async function defaultRun(command, args, options = {}) {
  const { input, stdio = 'inherit', shell = false, cwd, env } = options;
  await new Promise((resolve, reject) => {
    const child = spawn(command, args, { cwd, env, shell, stdio: ['pipe', stdio, stdio] });
    child.on('error', reject);
    if (typeof input === 'string') {
      child.stdin.end(input);
    } else {
      child.stdin.end();
    }
    child.on('close', (code) => {
      if (code === 0) {
        resolve();
        return;
      }
      reject(new Error(`${command} ${args.join(' ')} exited with code ${code}`));
    });
  });
}

export async function applyRequestedEntitlementProfile({
  env = process.env,
  logger = console,
  run = defaultRun,
  fsModule = fs,
} = {}) {
  const request = resolveEntitlementProfile(env);
  if (request.profile === '') {
    return { applied: false, reason: 'no_profile_requested' };
  }

  const target = resolveEntitlementTarget(env);
  if (!target) {
    const message =
      `Entitlement profile ${JSON.stringify(request.profile)} was requested for a live instance, ` +
      'but no entitlement write target is configured. Set PULSE_E2E_BILLING_STATE_PATH ' +
      'or PULSE_E2E_ENTITLEMENT_WRITE_COMMAND for deterministic local runs.';
    if (request.explicit) {
      throw new Error(message);
    }
    logger.warn(`[integration] ${message}`);
    return { applied: false, reason: 'no_target_configured', profile: request.profile };
  }

  const payload = `${JSON.stringify(buildBillingState(request.profile))}\n`;

  if (target.kind === 'docker') {
    await run('docker', [
      'exec',
      '-i',
      target.container,
      'sh',
      '-lc',
      `cat > ${shellQuote(target.path)}`,
    ], { input: payload });
    logger.log(
      `[integration] Applied entitlement profile ${request.profile} to docker container ${target.container}:${target.path}`,
    );
    return { applied: true, target, profile: request.profile };
  }

  if (target.kind === 'file') {
    await fsModule.mkdir(path.dirname(target.path), { recursive: true });
    await fsModule.writeFile(target.path, payload, 'utf8');
    logger.log(
      `[integration] Applied entitlement profile ${request.profile} to ${target.path}`,
    );
    return { applied: true, target, profile: request.profile };
  }

  if (target.kind === 'command') {
    await run('sh', ['-lc', target.command], { input: payload });
    logger.log(
      `[integration] Applied entitlement profile ${request.profile} via PULSE_E2E_ENTITLEMENT_WRITE_COMMAND`,
    );
    return { applied: true, target, profile: request.profile };
  }

  throw new Error(`Unknown entitlement bootstrap target kind ${JSON.stringify(target.kind)}`);
}
