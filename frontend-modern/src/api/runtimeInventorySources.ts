import { apiFetchJSON } from '@/utils/apiClient';
import type { ConnectionState, ConnectionType } from './connections';

/**
 * RuntimeInventorySource mirrors the backend whitelist type served by
 * GET /api/runtime/inventory-sources at monitoring:read.
 *
 * It is deliberately NOT `Connection`. Monitoring surfaces warn viewers that
 * inventory is incomplete; they do not need — and must not receive — the
 * addresses, agent identity, fleet governance or raw error text that the
 * administrative connections ledger carries. Reach for `ConnectionsAPI.list()`
 * only from admin surfaces such as Settings > Infrastructure.
 */
export interface RuntimeInventorySource {
  id: string;
  type: ConnectionType;
  name: string;
  state: ConnectionState;
  /** Effective inventory coverage; the backend already resolves scope over surfaces. */
  surfaces: string[];
  /** True when this source needs an administrator to fix its credentials. */
  credentialsInvalid: boolean;
}

export interface RuntimeInventorySourcesResponse {
  sources: RuntimeInventorySource[];
}

interface RuntimeInventorySourcesWireResponse {
  sources?: RuntimeInventorySource[];
}

export class RuntimeInventorySourcesAPI {
  private static readonly baseUrl = '/api/runtime/inventory-sources';

  static async list(): Promise<RuntimeInventorySourcesResponse> {
    const response: RuntimeInventorySourcesWireResponse = await apiFetchJSON(this.baseUrl);
    return { sources: response.sources ?? [] };
  }
}
