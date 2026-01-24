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

    // Backend already returns fields with correct names (server, port)
    return {
      enabled: (backendConfig.enabled as boolean) || false,
      provider: (backendConfig.provider as string) || '',
      server: (backendConfig.server as string) || '',
      port: (backendConfig.port as number) || 587,
      username: (backendConfig.username as string) || '',
      password: (backendConfig.password as string) || '',
      from: (backendConfig.from as string) || '',
      to: (backendConfig.to as string[]) || [],
      tls: (backendConfig.tls as boolean) || false,
      startTLS: (backendConfig.startTLS as boolean) || false,
    };
  }

  static async updateEmailConfig(config: EmailConfig): Promise<{ success: boolean }> {
    // Backend expects fields with these names (server, port)
    const backendConfig = {
      enabled: config.enabled,
      server: config.server,
      port: config.port,
      username: config.username,
      password: config.password,
      from: config.from,
      to: config.to,
      tls: config.tls || false,
      startTLS: config.startTLS || false,
      provider: config.provider || '',
    };

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
    return apiFetchJSON(`${this.baseUrl}/webhooks/${id}`, {
      method: 'PUT',
      body: JSON.stringify(webhook),
    });
  }

  static async deleteWebhook(id: string): Promise<{ success: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/webhooks/${id}`, {
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
    const body: { method: string; config?: Record<string, unknown> | AppriseConfig; webhookId?: string } = {
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
