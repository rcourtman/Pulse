import { apiFetchJSON } from '@/utils/apiClient';

export interface AgentLedgerEntry {
  name: string;
  type: string; // "agent"
  status: string; // "online" | "offline" | "unknown"
  last_seen: string; // RFC3339 or empty
  source: string; // "agent"
}

export interface AgentLedgerResponse {
  agents: AgentLedgerEntry[];
  total: number;
  limit: number; // 0 = unlimited
}

export class AgentLedgerAPI {
  private static readonly baseUrl = '/api/license/agent-ledger';

  static async getLedger(): Promise<AgentLedgerResponse> {
    return apiFetchJSON<AgentLedgerResponse>(this.baseUrl);
  }
}
