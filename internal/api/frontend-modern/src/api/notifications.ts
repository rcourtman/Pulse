
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
}

export interface NotificationTestRequest {
  type: 'email' | 'webhook';
  config?: any;  // Backend expects different format than frontend types
  webhookId?: string;
}

export class NotificationsAPI {
  private static baseUrl = '/api/notifications';

  // Email configuration
  static async getEmailConfig(): Promise<EmailConfig> {
    const response = await fetch(`${this.baseUrl}/email`);
    if (!response.ok) {
      throw new Error('Failed to fetch email configuration');
    }
    const backendConfig = await response.json();
    
    // Backend already returns fields with correct names (server, port)
    return {
      enabled: backendConfig.enabled || false,
      provider: backendConfig.provider || '',
      server: backendConfig.server || '',
      port: backendConfig.port || 587,
      username: backendConfig.username || '',
      password: backendConfig.password || '',
      from: backendConfig.from || '',
      to: backendConfig.to || [],
      tls: backendConfig.tls || false,
      startTLS: backendConfig.startTLS || false
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
      provider: config.provider || ''
    };
    
    const response = await fetch(`${this.baseUrl}/email`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(backendConfig),
    });
    
    if (!response.ok) {
      throw new Error('Failed to update email configuration');
    }
    
    return response.json();
  }

  // Webhook management
  static async getWebhooks(): Promise<Webhook[]> {
    const response = await fetch(`${this.baseUrl}/webhooks`);
    if (!response.ok) {
      throw new Error('Failed to fetch webhooks');
    }
    return response.json();
  }

  static async createWebhook(webhook: Omit<Webhook, 'id'>): Promise<Webhook> {
    const response = await fetch(`${this.baseUrl}/webhooks`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(webhook),
    });
    
    if (!response.ok) {
      throw new Error('Failed to create webhook');
    }
    
    return response.json();
  }

  static async updateWebhook(id: string, webhook: Partial<Webhook>): Promise<Webhook> {
    const response = await fetch(`${this.baseUrl}/webhooks/${id}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(webhook),
    });
    
    if (!response.ok) {
      throw new Error('Failed to update webhook');
    }
    
    return response.json();
  }

  static async deleteWebhook(id: string): Promise<{ success: boolean }> {
    const response = await fetch(`${this.baseUrl}/webhooks/${id}`, {
      method: 'DELETE',
    });
    
    if (!response.ok) {
      throw new Error('Failed to delete webhook');
    }
    
    return response.json();
  }

  // Templates and providers
  static async getEmailProviders(): Promise<EmailProvider[]> {
    const response = await fetch(`${this.baseUrl}/email-providers`);
    if (!response.ok) {
      throw new Error('Failed to fetch email providers');
    }
    return response.json();
  }

  static async getWebhookTemplates(): Promise<WebhookTemplate[]> {
    const response = await fetch(`${this.baseUrl}/webhook-templates`);
    if (!response.ok) {
      throw new Error('Failed to fetch webhook templates');
    }
    return response.json();
  }

  // Testing
  static async testNotification(request: NotificationTestRequest): Promise<{ success: boolean; message?: string }> {
    const body: { method: string; config?: any } = { method: request.type };
    
    // Include config if provided for testing without saving
    if (request.config) {
      body.config = request.config;
    }
    
    const response = await fetch(`${this.baseUrl}/test`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
    });
    
    if (!response.ok) {
      const error = await response.text();
      throw new Error(error || 'Failed to test notification');
    }
    
    return response.json();
  }

  static async testWebhook(webhook: Webhook): Promise<{ success: boolean; message?: string }> {
    const response = await fetch(`${this.baseUrl}/webhooks/test`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(webhook),
    });
    
    if (!response.ok) {
      const error = await response.text();
      throw new Error(error || 'Failed to test webhook');
    }
    
    return response.json();
  }
}