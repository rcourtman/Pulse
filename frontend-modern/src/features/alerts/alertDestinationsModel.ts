import type { AppriseConfig, EmailConfig } from '@/api/notifications';

import { formatAppriseTargets, normalizeEmailConfigFromAPI, parseAppriseTargets } from './helpers';
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
    hasApiKey: Boolean(config?.hasApiKey || config?.apiKey),
    minimumSeverity: config?.minimumSeverity === 'critical' ? 'critical' : 'all',
  };
}

export function buildEmailConfigPayload(config: UIEmailConfig): EmailConfig {
  const payload: EmailConfig = {
    enabled: config.enabled,
    provider: config.provider,
    server: config.server,
    port: config.port,
    username: config.username,
    password: config.password,
    from: config.from,
    to: config.to.map((entry) => entry.trim()).filter((entry) => entry.length > 0),
    tls: config.tls,
    startTLS: config.startTLS,
  };
  if (config.tagFilter !== undefined) {
    payload.tagFilter = config.tagFilter;
  }
  if (config.tagFilterMode !== undefined) {
    payload.tagFilterMode = config.tagFilterMode === 'any' ? 'any' : 'all';
  }
  payload.minimumSeverity = config.minimumSeverity === 'critical' ? 'critical' : 'all';
  return payload;
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
    minimumSeverity: config.minimumSeverity === 'critical' ? 'critical' : 'all',
  } as AppriseConfig;
}

export { normalizeEmailConfigFromAPI };
