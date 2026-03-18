import { apiFetchJSON } from '@/utils/apiClient';

export interface MonitoredSystemLedgerEntry {
  name: string;
  type: string;
  status: string; // "online" | "offline" | "unknown"
  last_seen: string; // RFC3339 or empty
  source: string;
}

export interface MonitoredSystemLedgerResponse {
  systems: MonitoredSystemLedgerEntry[];
  total: number;
  limit: number; // 0 = unlimited
}

export class MonitoredSystemLedgerAPI {
  private static readonly baseUrl = '/api/license/monitored-system-ledger';

  static async getLedger(): Promise<MonitoredSystemLedgerResponse> {
    return apiFetchJSON<MonitoredSystemLedgerResponse>(this.baseUrl);
  }
}
