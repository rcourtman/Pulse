// Docker Metadata API
import { apiFetchJSON } from '@/utils/apiClient';

export interface DockerMetadata {
  id: string;
  customUrl?: string;
  description?: string;
  tags?: string[];
  notes?: string[]; // User annotations for AI context
}

export class DockerMetadataAPI {
  private static baseUrl = '/api/docker/metadata';

  // Get metadata for a specific docker resource (container or service)
  static async getMetadata(resourceId: string): Promise<DockerMetadata> {
    return apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(resourceId)}`);
  }

  // Get all docker metadata
  static async getAllMetadata(): Promise<Record<string, DockerMetadata>> {
    return apiFetchJSON(this.baseUrl);
  }

  // Update metadata for a docker resource
  static async updateMetadata(
    resourceId: string,
    metadata: Partial<DockerMetadata>,
  ): Promise<DockerMetadata> {
    return apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(resourceId)}`, {
      method: 'PUT',
      body: JSON.stringify(metadata),
    });
  }

  // Delete metadata for a docker resource
  static async deleteMetadata(resourceId: string): Promise<void> {
    await apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(resourceId)}`, {
      method: 'DELETE',
    });
  }
}
