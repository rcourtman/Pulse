import { apiFetchJSON } from '@/utils/apiClient';

export interface HostLedgerEntry {
  name: string;
  type: string; // "agent"
  status: string; // "online" | "offline" | "unknown"
  last_seen: string; // RFC3339 or empty
  source: string; // "agent"
}

export interface HostLedgerResponse {
  hosts: HostLedgerEntry[];
  total: number;
  limit: number; // 0 = unlimited
}

export class HostLedgerAPI {
  private static readonly baseUrl = '/api/license/host-ledger';

  static async getLedger(): Promise<HostLedgerResponse> {
    return apiFetchJSON<HostLedgerResponse>(this.baseUrl);
  }
}
