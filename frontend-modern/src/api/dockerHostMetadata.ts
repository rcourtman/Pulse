import { apiFetchJSON } from '@/utils/apiClient';

export interface DockerHostMetadata {
  customDisplayName?: string;
  customUrl?: string;
  notes?: string[];
}

const baseUrl = '/api/docker/runtimes/metadata';
const detailPath = (runtimeId: string): string => `${baseUrl}/${encodeURIComponent(runtimeId)}`;

export class DockerHostMetadataAPI {
  static async getMetadata(runtimeId: string): Promise<DockerHostMetadata> {
    return apiFetchJSON<DockerHostMetadata>(detailPath(runtimeId));
  }

  static async updateMetadata(
    runtimeId: string,
    metadata: Partial<DockerHostMetadata>,
  ): Promise<DockerHostMetadata> {
    return apiFetchJSON<DockerHostMetadata>(detailPath(runtimeId), {
      method: 'PUT',
      body: JSON.stringify(metadata),
    });
  }
}
