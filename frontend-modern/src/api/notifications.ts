import { apiFetchJSON } from '@/utils/apiClient';
import {
  arrayOrEmpty,
  finiteNumberOrUndefined,
  strictBoolean,
  strictString,
  stringArray,
} from './responseUtils';

export interface EmailProvider {
  id?: string;
  name: string;
  smtpHost: string;
  smtpPort: number;
  tls: boolean;
  startTLS: boolean;
  authRequired: boolean;
  instructions: string;
  server?: string;
  port?: number;
  security?: 'none' | 'tls' | 'starttls';
}

export interface WebhookTemplate {
  id?: string;
  service: string;
  label?: string;
  mentionPlaceholder?: string;
  mentionHelp?: string;
  name: string;
  urlPattern: string;
  method: string;
  headers: Record<string, string>;
  payloadTemplate: string;
  instructions: string;
  description?: string;
  template?: {
    url?: string;
    method?: string;
    headers?: Record<string, string>;
    body?: string;
  };
}

export interface EmailConfig {
  enabled: boolean;
  provider: string;
  server: string;
  port: number;
  username: string;
  password?: string;
  from: string;
  to: string[];
  tls: boolean;
  startTLS: boolean;
  rateLimit?: number;
}

export interface Webhook {
  id: string;
  name: string;
  url: string;
  method: string;
  headers: Record<string, string>;
  template?: string;
  enabled: boolean;
  service?: string; // Added to support Discord, Slack, etc.
  customFields?: Record<string, string>;
  mention?: string; // Platform-specific mention (e.g., @everyone, @channel, <@USER_ID>)
}

export interface AppriseConfig {
  enabled: boolean;
  mode?: 'cli' | 'http';
  targets?: string[];
  cliPath?: string;
  timeoutSeconds?: number;
  serverUrl?: string;
  configKey?: string;
  apiKey?: string;
  apiKeyHeader?: string;
  skipTlsVerify?: boolean;
}

export interface NotificationTestRequest {
  type: 'email' | 'webhook' | 'apprise';
  config?: Record<string, unknown> | AppriseConfig; // Backend expects different format than frontend types
  webhookId?: string;
}

export type NotificationQueueHealthStatus = 'healthy' | 'degraded' | 'unavailable';

export interface NotificationQueueHealth {
  pending: number;
  sending: number;
  sent: number;
  failed: number;
  deadLetter: number;
  healthy: boolean;
  status: NotificationQueueHealthStatus;
  attentionRequired: number;
  reasonCodes: string[];
  completedRetentionDays: number;
  deadLetterRetentionDays: number;
  countsAreRetentionBounded: boolean;
  retryAttemptsAffectHealth: boolean;
  terminalFailuresAffectHealth: boolean;
}

export interface NotificationHealth {
  overallHealthy: boolean;
  queue: NotificationQueueHealth;
}

function apiRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' ? (value as Record<string, unknown>) : {};
}

function normalizeQueueHealthStatus(value: unknown): NotificationQueueHealthStatus {
  return value === 'healthy' || value === 'degraded' ? value : 'unavailable';
}

function nonNegativeCount(value: unknown): number | undefined {
  const normalized = finiteNumberOrUndefined(value);
  return normalized !== undefined && Number.isInteger(normalized) && normalized >= 0
    ? normalized
    : undefined;
}

export class NotificationsAPI {
  private static baseUrl = '/api/notifications';

  static async getAppriseConfig(): Promise<AppriseConfig> {
    return apiFetchJSON(`${this.baseUrl}/apprise`);
  }

  static async getHealth(): Promise<NotificationHealth> {
    const payload = await apiFetchJSON<Record<string, unknown>>(`${this.baseUrl}/health`);
    const queue = apiRecord(payload.queue);
    const pending = nonNegativeCount(queue.pending);
    const sending = nonNegativeCount(queue.sending);
    const sent = nonNegativeCount(queue.sent);
    const failed = nonNegativeCount(queue.failed);
    const deadLetter = nonNegativeCount(queue.dlq);
    const attentionRequired = nonNegativeCount(queue.attention_required);
    const completedRetentionDays = nonNegativeCount(queue.completed_retention_days);
    const deadLetterRetentionDays = nonNegativeCount(queue.dead_letter_retention_days);
    const reasonCodesAreValid =
      Array.isArray(queue.reason_codes) &&
      queue.reason_codes.every((reasonCode) => typeof reasonCode === 'string');
    const semanticsAreValid =
      queue.counts_are_retention_bounded === true &&
      queue.retry_attempts_affect_health === false &&
      queue.terminal_failures_affect_health === true;
    const availableFieldsAreValid =
      pending !== undefined &&
      sending !== undefined &&
      sent !== undefined &&
      attentionRequired !== undefined &&
      completedRetentionDays !== undefined &&
      completedRetentionDays > 0 &&
      deadLetterRetentionDays !== undefined &&
      deadLetterRetentionDays > 0 &&
      reasonCodesAreValid &&
      semanticsAreValid;
    const rawStatus = normalizeQueueHealthStatus(queue.status);
    const rawHealthy = typeof queue.healthy === 'boolean' ? queue.healthy : undefined;
    const terminalFailureCount =
      failed !== undefined && deadLetter !== undefined ? failed + deadLetter : undefined;
    const statusIsConsistent =
      rawStatus === 'unavailable' ||
      (rawStatus === 'healthy' &&
        availableFieldsAreValid &&
        rawHealthy === true &&
        terminalFailureCount === 0 &&
        attentionRequired === 0) ||
      (rawStatus === 'degraded' &&
        availableFieldsAreValid &&
        rawHealthy === false &&
        terminalFailureCount !== undefined &&
        terminalFailureCount > 0 &&
        attentionRequired === terminalFailureCount);
    const status = statusIsConsistent ? rawStatus : 'unavailable';

    return {
      overallHealthy: status === 'healthy' && strictBoolean(payload.overall_healthy),
      queue: {
        pending: pending ?? 0,
        sending: sending ?? 0,
        sent: sent ?? 0,
        failed: failed ?? 0,
        deadLetter: deadLetter ?? 0,
        healthy: status === 'healthy',
        status,
        attentionRequired: attentionRequired ?? 0,
        reasonCodes: stringArray(queue.reason_codes),
        completedRetentionDays: completedRetentionDays ?? 7,
        deadLetterRetentionDays: deadLetterRetentionDays ?? 30,
        countsAreRetentionBounded: strictBoolean(queue.counts_are_retention_bounded),
        retryAttemptsAffectHealth: strictBoolean(queue.retry_attempts_affect_health),
        terminalFailuresAffectHealth: strictBoolean(queue.terminal_failures_affect_health, true),
      },
    };
  }

  static async updateAppriseConfig(config: AppriseConfig): Promise<AppriseConfig> {
    return apiFetchJSON(`${this.baseUrl}/apprise`, {
      method: 'PUT',
      body: JSON.stringify(config),
    });
  }

  // Email configuration
  static async getEmailConfig(): Promise<EmailConfig> {
    const backendConfig = await apiFetchJSON<Record<string, unknown>>(`${this.baseUrl}/email`);
    const port = finiteNumberOrUndefined(backendConfig.port);

    // Backend already returns fields with correct names (server, port)
    return {
      enabled: strictBoolean(backendConfig.enabled),
      provider: strictString(backendConfig.provider),
      server: strictString(backendConfig.server),
      port: port ?? 587,
      username: strictString(backendConfig.username),
      password: strictString(backendConfig.password),
      from: strictString(backendConfig.from),
      to: stringArray(backendConfig.to),
      tls: strictBoolean(backendConfig.tls),
      startTLS: strictBoolean(backendConfig.startTLS),
      rateLimit: finiteNumberOrUndefined(backendConfig.rateLimit),
    };
  }

  static async updateEmailConfig(config: EmailConfig): Promise<{ success: boolean }> {
    // Backend expects fields with these names (server, port)
    const backendConfig: Record<string, unknown> = {
      enabled: config.enabled,
      server: config.server,
      port: config.port,
      username: config.username,
      password: config.password,
      from: config.from,
      to: config.to,
      tls: config.tls,
      startTLS: config.startTLS,
      provider: config.provider,
    };

    // Only include rateLimit if it's explicitly set
    if (config.rateLimit !== undefined) {
      backendConfig.rateLimit = config.rateLimit;
    }

    return apiFetchJSON(`${this.baseUrl}/email`, {
      method: 'PUT',
      body: JSON.stringify(backendConfig),
    });
  }

  // Webhook management
  static async getWebhooks(): Promise<Webhook[]> {
    const data = await apiFetchJSON<Webhook[] | null>(`${this.baseUrl}/webhooks`);
    return arrayOrEmpty<Webhook>(data);
  }

  static async createWebhook(webhook: Omit<Webhook, 'id'>): Promise<Webhook> {
    return apiFetchJSON(`${this.baseUrl}/webhooks`, {
      method: 'POST',
      body: JSON.stringify(webhook),
    });
  }

  static async updateWebhook(id: string, webhook: Partial<Webhook>): Promise<Webhook> {
    return apiFetchJSON(`${this.baseUrl}/webhooks/${encodeURIComponent(id)}`, {
      method: 'PUT',
      body: JSON.stringify(webhook),
    });
  }

  static async deleteWebhook(id: string): Promise<{ success: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/webhooks/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
  }

  // Templates and providers
  static async getEmailProviders(): Promise<EmailProvider[]> {
    return apiFetchJSON(`${this.baseUrl}/email-providers`);
  }

  static async getWebhookTemplates(): Promise<WebhookTemplate[]> {
    return apiFetchJSON(`${this.baseUrl}/webhook-templates`);
  }

  // Testing
  static async testNotification(
    request: NotificationTestRequest,
  ): Promise<{ success: boolean; message?: string }> {
    const body: {
      method: string;
      config?: Record<string, unknown> | AppriseConfig;
      webhookId?: string;
    } = {
      method: request.type,
    };

    // Include config if provided for testing without saving
    if (request.config) {
      body.config = request.config;
    }

    // Include webhookId for webhook testing
    if (request.webhookId) {
      body.webhookId = request.webhookId;
    }

    return apiFetchJSON(`${this.baseUrl}/test`, {
      method: 'POST',
      body: JSON.stringify(body),
    });
  }

  static async testWebhook(
    webhook: Omit<Webhook, 'id'>,
  ): Promise<{ success: boolean; message?: string }> {
    return apiFetchJSON(`${this.baseUrl}/webhooks/test`, {
      method: 'POST',
      body: JSON.stringify(webhook),
    });
  }
}
