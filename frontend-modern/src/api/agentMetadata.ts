import { buildMetadataAPI, type ResourceMetadataRecord } from './metadataClient';

export interface AgentMetadata extends ResourceMetadataRecord {}

const agentMetadataAPI = buildMetadataAPI<AgentMetadata>('/api/agents/metadata');

export class AgentMetadataAPI {
  // Get metadata for a specific agent
  static async getMetadata(agentId: string): Promise<AgentMetadata> {
    return agentMetadataAPI.getMetadata(agentId);
  }

  // Get all agent metadata
  static async getAllMetadata(): Promise<Record<string, AgentMetadata>> {
    return agentMetadataAPI.getAllMetadata();
  }

  // Update metadata for an agent
  static async updateMetadata(
    agentId: string,
    metadata: Partial<AgentMetadata>,
  ): Promise<AgentMetadata> {
    return agentMetadataAPI.updateMetadata(agentId, metadata);
  }

  // Delete metadata for an agent
  static async deleteMetadata(agentId: string): Promise<void> {
    await agentMetadataAPI.deleteMetadata(agentId);
  }
}
