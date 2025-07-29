
export interface EmailProvider {
  id: string;
  name: string;
  server: string;
  port: number;
  security: 'none' | 'tls' | 'starttls';
}

export interface WebhookTemplate {
  id: string;
  name: string;
  description: string;
  template: {
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
  starttls: boolean;
}

export interface Webhook {
  id: string;
  name: string;
  url: string;
  method: string;
  headers: Record<string, string>;
  template?: string;
  enabled: boolean;
}

export interface NotificationTestRequest {
  type: 'email' | 'webhook';
  config?: EmailConfig | Webhook;
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
    return response.json();
  }

  static async updateEmailConfig(config: EmailConfig): Promise<{ success: boolean }> {
    const response = await fetch(`${this.baseUrl}/email`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(config),
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
    const response = await fetch(`${this.baseUrl}/test`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(request),
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