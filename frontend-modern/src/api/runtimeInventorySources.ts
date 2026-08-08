import { apiFetchJSON } from '@/utils/apiClient';

export type RuntimeInventorySourceType = 'pve' | 'vmware' | 'docker' | 'kubernetes';

export type RuntimeInventorySourceState =
  'paused' | 'pending' | 'stale' | 'unauthorized' | 'unreachable';

/**
 * Complete viewer-safe wire shape returned by GET /api/runtime/inventory-sources.
 * This intentionally does not reuse the administrative Connection model.
 */
export interface RuntimeInventorySource {
  type: RuntimeInventorySourceType;
  name: string;
  state: RuntimeInventorySourceState;
  surfaces: string[];
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
    const response = await apiFetchJSON<RuntimeInventorySourcesWireResponse>(this.baseUrl);
    return { sources: Array.isArray(response.sources) ? response.sources : [] };
  }
}
