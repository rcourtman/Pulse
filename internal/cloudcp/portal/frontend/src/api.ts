import type { PortalBootstrapData, PortalTeamMember } from './types';

interface PortalAPIContext {
  getBootstrap(): PortalBootstrapData;
}

export class PortalAPIError extends Error {
  status: number;
  payload: unknown;

  constructor(message: string, status = 0, payload: unknown = null) {
    super(message);
    this.name = 'PortalAPIError';
    this.status = status;
    this.payload = payload;
  }
}

export interface PortalBillingResponse {
  url?: string;
}

export interface PortalMagicLinkResponse {
  message?: string;
}

export interface PortalWorkspaceCreateRequest {
  display_name: string;
}

export interface PortalMemberInviteRequest {
  email: string;
  role: string;
}

export interface PortalMemberRoleRequest {
  role: string;
}

export interface PortalAPI {
  fetchBootstrap(): Promise<PortalBootstrapData>;
  requestMagicLink(email: string): Promise<PortalMagicLinkResponse>;
  logout(): Promise<void>;
  postCommercialJSON<T>(path: string, body: Record<string, unknown>): Promise<T>;
  createWorkspace(accountID: string, body: PortalWorkspaceCreateRequest): Promise<void>;
  suspendWorkspace(accountID: string, tenantID: string): Promise<void>;
  deleteWorkspace(accountID: string, tenantID: string): Promise<void>;
  openBilling(accountID: string): Promise<PortalBillingResponse>;
  listMembers(accountID: string): Promise<PortalTeamMember[]>;
  inviteMember(accountID: string, body: PortalMemberInviteRequest): Promise<void>;
  updateMemberRole(accountID: string, userID: string, body: PortalMemberRoleRequest): Promise<void>;
  removeMember(accountID: string, userID: string): Promise<void>;
}

export function createPortalAPI(context: PortalAPIContext): PortalAPI {
  function bootstrap() {
    return context.getBootstrap();
  }

  async function readPayload(response: Response): Promise<unknown> {
    var contentType = response.headers && typeof response.headers.get === 'function'
      ? response.headers.get('content-type') || ''
      : '';
    if (typeof response.json === 'function') {
      try {
        return await response.json();
      } catch {
        // Fall through to text/null handling.
      }
    }
    if (contentType.includes('application/json')) {
      try {
        return await response.json();
      } catch {
        return null;
      }
    }
    try {
      var text = await response.text();
      return text || null;
    } catch {
      return null;
    }
  }

  function messageFromPayload(payload: unknown, fallback: string): string {
    if (payload && typeof payload === 'object') {
      var errorMessage = (payload as { error?: unknown }).error;
      if (typeof errorMessage === 'string' && errorMessage.trim()) {
        return errorMessage;
      }
      var message = (payload as { message?: unknown }).message;
      if (typeof message === 'string' && message.trim()) {
        return message;
      }
    }
    if (typeof payload === 'string' && payload.trim()) {
      return payload;
    }
    return fallback;
  }

  async function request<T>(input: string, init: RequestInit, fallbackMessage: string): Promise<T> {
    var response: Response;
    try {
      if (Object.keys(init).length > 0) {
        response = await fetch(input, init);
      } else {
        response = await fetch(input);
      }
    } catch {
      throw new PortalAPIError('Network error.', 0, null);
    }
    var payload = await readPayload(response);
    if (!response.ok) {
      throw new PortalAPIError(messageFromPayload(payload, fallbackMessage), response.status, payload);
    }
    return payload as T;
  }

  function accountURL(accountID: string, suffix = ''): string {
    return bootstrap().account_api_base_path + '/' + encodeURIComponent(accountID) + suffix;
  }

  return {
    fetchBootstrap: function() {
      return request<PortalBootstrapData>(bootstrap().bootstrap_path, {
        headers: { Accept: 'application/json' },
      }, 'Failed to refresh account state.');
    },
    requestMagicLink: function(email: string) {
      return request<PortalMagicLinkResponse>(bootstrap().magic_link_request_path, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email, target: 'portal' }),
      }, 'Failed to send magic link.');
    },
    logout: function() {
      return request<void>(bootstrap().logout_path, {
        method: 'POST',
      }, 'Failed to sign out.');
    },
    postCommercialJSON: function<T>(path: string, body: Record<string, unknown>) {
      return request<T>(bootstrap().commercial_api_base_url + path, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      }, 'Commercial request failed.');
    },
    createWorkspace: function(accountID: string, body: PortalWorkspaceCreateRequest) {
      return request<void>(accountURL(accountID, '/tenants'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      }, 'Failed to create workspace.');
    },
    suspendWorkspace: function(accountID: string, tenantID: string) {
      return request<void>(accountURL(accountID, '/tenants/' + encodeURIComponent(tenantID)), {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ state: 'suspended' }),
      }, 'Failed to suspend workspace.');
    },
    deleteWorkspace: function(accountID: string, tenantID: string) {
      return request<void>(accountURL(accountID, '/tenants/' + encodeURIComponent(tenantID)), {
        method: 'DELETE',
      }, 'Failed to delete workspace.');
    },
    openBilling: function(accountID: string) {
      return request<PortalBillingResponse>(bootstrap().portal_api_base_path + '/billing?account_id=' + encodeURIComponent(accountID), {
        method: 'POST',
      }, 'Failed to open billing portal.');
    },
    listMembers: function(accountID: string) {
      return request<PortalTeamMember[]>(accountURL(accountID, '/members'), {}, 'Failed to load team.');
    },
    inviteMember: function(accountID: string, body: PortalMemberInviteRequest) {
      return request<void>(accountURL(accountID, '/members'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      }, 'Failed to invite member.');
    },
    updateMemberRole: function(accountID: string, userID: string, body: PortalMemberRoleRequest) {
      return request<void>(accountURL(accountID, '/members/' + encodeURIComponent(userID)), {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      }, 'Failed to update role.');
    },
    removeMember: function(accountID: string, userID: string) {
      return request<void>(accountURL(accountID, '/members/' + encodeURIComponent(userID)), {
        method: 'DELETE',
      }, 'Failed to remove member.');
    },
  };
}
