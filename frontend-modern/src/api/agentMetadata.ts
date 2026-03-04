import { apiFetchJSON } from '@/utils/apiClient';

export interface AgentMetadata {
  id: string;
  customUrl?: string;
  description?: string;
  tags?: string[];
  notes?: string[]; // User annotations for AI context
}

// Compatibility alias while call sites migrate from host naming to agent naming.
export type HostMetadata = AgentMetadata;

export class AgentMetadataAPI {
  private static baseUrl = '/api/agents/metadata';

  // Get metadata for a specific agent
  static async getMetadata(agentId: string): Promise<AgentMetadata> {
    return apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(agentId)}`);
  }

  // Get all agent metadata
  static async getAllMetadata(): Promise<Record<string, AgentMetadata>> {
    return apiFetchJSON(this.baseUrl);
  }

  // Update metadata for an agent
  static async updateMetadata(
    agentId: string,
    metadata: Partial<AgentMetadata>,
  ): Promise<AgentMetadata> {
    return apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(agentId)}`, {
      method: 'PUT',
      body: JSON.stringify(metadata),
    });
  }

  // Delete metadata for an agent
  static async deleteMetadata(agentId: string): Promise<void> {
    await apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(agentId)}`, {
      method: 'DELETE',
    });
  }
}

// Compatibility alias while transitioning imports.
export class HostMetadataAPI extends AgentMetadataAPI {}
