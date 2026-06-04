import { buildMetadataAPI, type ResourceMetadataRecord } from './metadataClient';

export interface DockerMetadata extends ResourceMetadataRecord {}

const dockerMetadataAPI = buildMetadataAPI<DockerMetadata>('/api/docker/metadata');

export class DockerMetadataAPI {
  static async getMetadata(resourceId: string): Promise<DockerMetadata> {
    return dockerMetadataAPI.getMetadata(resourceId);
  }

  static async getAllMetadata(): Promise<Record<string, DockerMetadata>> {
    return dockerMetadataAPI.getAllMetadata();
  }

  static async updateMetadata(
    resourceId: string,
    metadata: Partial<DockerMetadata>,
  ): Promise<DockerMetadata> {
    return dockerMetadataAPI.updateMetadata(resourceId, metadata);
  }

  static async deleteMetadata(resourceId: string): Promise<void> {
    await dockerMetadataAPI.deleteMetadata(resourceId);
  }
}
