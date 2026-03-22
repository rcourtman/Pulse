import type { AppriseConfig, EmailConfig } from '@/api/notifications';

import {
  formatAppriseTargets,
  normalizeEmailConfigFromAPI,
  parseAppriseTargets,
} from './helpers';
import type { UIAppriseConfig, UIEmailConfig } from './types';

export function normalizeAppriseConfig(
  config: Partial<AppriseConfig> | null | undefined,
): UIAppriseConfig {
  return {
    enabled: config?.enabled ?? false,
    mode: config?.mode === 'http' ? 'http' : 'cli',
    targetsText: formatAppriseTargets(config?.targets),
    cliPath: config?.cliPath || 'apprise',
    timeoutSeconds:
      typeof config?.timeoutSeconds === 'number' && config.timeoutSeconds > 0
        ? config.timeoutSeconds
        : 15,
    serverUrl: config?.serverUrl || '',
    configKey: config?.configKey || '',
    apiKey: config?.apiKey || '',
    apiKeyHeader: config?.apiKeyHeader || 'X-API-KEY',
    skipTlsVerify: Boolean(config?.skipTlsVerify),
  };
}

export function buildEmailConfigPayload(config: UIEmailConfig): EmailConfig {
  return {
    enabled: config.enabled,
    provider: config.provider,
    server: config.server,
    port: config.port,
    username: config.username,
    password: config.password,
    from: config.from,
    to: config.to,
    tls: config.tls,
    startTLS: config.startTLS,
  } as EmailConfig;
}

export function buildAppriseConfigPayload(config: UIAppriseConfig): AppriseConfig {
  return {
    enabled: config.enabled,
    mode: config.mode,
    targets: parseAppriseTargets(config.targetsText),
    cliPath: config.cliPath,
    timeoutSeconds: config.timeoutSeconds,
    serverUrl: config.serverUrl,
    configKey: config.configKey,
    apiKey: config.apiKey,
    apiKeyHeader: config.apiKeyHeader,
    skipTlsVerify: config.skipTlsVerify,
  } as AppriseConfig;
}

export {
  normalizeEmailConfigFromAPI,
};
