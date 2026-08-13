import { apiFetchJSON } from '@/utils/apiClient';

export type RuntimeInventorySourceType = 'pve' | 'vmware' | 'docker' | 'kubernetes';

export type RuntimeInventorySourceState =
  'active' | 'paused' | 'pending' | 'stale' | 'unauthorized' | 'unreachable';

export interface RuntimeInventoryCompletenessIssue {
  stage?: string;
  category?: string;
  occurrences?: number;
}

export interface RuntimeInventoryCompleteness {
  state: 'degraded';
  issueCount: number;
  issues?: RuntimeInventoryCompletenessIssue[];
}

/**
 * Complete viewer-safe wire shape returned by GET /api/runtime/inventory-sources.
 * This intentionally does not reuse the administrative Connection model.
 */
export interface RuntimeInventorySource {
  type: RuntimeInventorySourceType;
  name: string;
  state: RuntimeInventorySourceState;
  surfaces: string[];
  completeness?: RuntimeInventoryCompleteness;
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
