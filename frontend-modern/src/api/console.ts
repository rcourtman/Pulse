/**
 * Console API client for interacting with the Pulse console proxy.
 *
 * Provides methods to:
 * - Request console tickets (VNC, SPICE, SSH, serial)
 * - Get supported console types for a VM
 * - List active console sessions
 */

export interface ConsoleTicketRequest {
  providerId: string;
  nodeId: string;
  vmId: string;
  type: 'vnc' | 'spice' | 'ssh' | 'serial' | 'rdp';
}

export interface ConsoleTicketResponse {
  sessionId: string;
  type: string;
  websocketUrl: string;
  vmId: string;
  nodeId: string;
  providerId: string;
}

export interface ConsoleSession {
  id: string;
  vmId: string;
  nodeId: string;
  providerId: string;
  type: string;
  createdAt: string;
  active: boolean;
}

export class ConsoleAPI {
  /**
   * Request a console ticket for a VM.
   * Returns session info including the WebSocket URL for the console.
   */
  static async requestTicket(req: ConsoleTicketRequest): Promise<ConsoleTicketResponse> {
    const response = await fetch('/api/console/ticket', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req),
    });
    if (!response.ok) {
      throw new Error(`Console ticket request failed: ${response.statusText}`);
    }
    return response.json();
  }

  /**
   * Get supported console types for a provider.
   */
  static async getConsoleTypes(providerId: string): Promise<string[]> {
    const response = await fetch(`/api/console/types?providerId=${encodeURIComponent(providerId)}`);
    if (!response.ok) {
      throw new Error(`Failed to get console types: ${response.statusText}`);
    }
    const data = await response.json();
    return data.types || [];
  }

  /**
   * List active console sessions.
   */
  static async listSessions(): Promise<ConsoleSession[]> {
    const response = await fetch('/api/console/sessions');
    if (!response.ok) {
      throw new Error(`Failed to list sessions: ${response.statusText}`);
    }
    return response.json();
  }

  /**
   * Build a full WebSocket URL from a relative path.
   */
  static buildWebSocketUrl(relativePath: string): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${protocol}//${window.location.host}${relativePath}`;
  }
}
