import { apiFetchJSON } from '@/utils/apiClient';

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

export class NotificationsAPI {
  private static baseUrl = '/api/notifications';
  private static readString(value: unknown, fallback = ''): string {
    return typeof value === 'string' ? value : fallback;
  }

  private static readBoolean(value: unknown, fallback = false): boolean {
    return typeof value === 'boolean' ? value : fallback;
  }

  private static readNumber(value: unknown): number | undefined {
    return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
  }

  private static readStringArray(value: unknown): string[] {
    if (!Array.isArray(value)) {
      return [];
    }

    return value.filter((item): item is string => typeof item === 'string');
  }

  static async getAppriseConfig(): Promise<AppriseConfig> {
    return apiFetchJSON(`${this.baseUrl}/apprise`);
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
    const port = this.readNumber(backendConfig.port);

    // Backend already returns fields with correct names (server, port)
    return {
      enabled: this.readBoolean(backendConfig.enabled),
      provider: this.readString(backendConfig.provider),
      server: this.readString(backendConfig.server),
      port: port ?? 587,
      username: this.readString(backendConfig.username),
      password: this.readString(backendConfig.password),
      from: this.readString(backendConfig.from),
      to: this.readStringArray(backendConfig.to),
      tls: this.readBoolean(backendConfig.tls),
      startTLS: this.readBoolean(backendConfig.startTLS),
      rateLimit: this.readNumber(backendConfig.rateLimit),
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
    return Array.isArray(data) ? data : [];
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
