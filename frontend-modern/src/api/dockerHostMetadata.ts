// Docker Host Metadata API - for managing custom URLs on Docker hosts
import { apiFetchJSON } from '@/utils/apiClient';

export interface DockerHostMetadata {
    customDisplayName?: string;
    customUrl?: string;
    notes?: string[];
}

export class DockerHostMetadataAPI {
    private static baseUrl = '/api/docker/hosts/metadata';

    // Get metadata for a specific Docker host
    static async getMetadata(hostId: string): Promise<DockerHostMetadata> {
        return apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(hostId)}`);
    }

    // Get all Docker host metadata
    static async getAllMetadata(): Promise<Record<string, DockerHostMetadata>> {
        return apiFetchJSON(this.baseUrl);
    }

    // Update metadata for a Docker host
    static async updateMetadata(
        hostId: string,
        metadata: Partial<DockerHostMetadata>,
    ): Promise<DockerHostMetadata> {
        return apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(hostId)}`, {
            method: 'PUT',
            body: JSON.stringify(metadata),
        });
    }

    // Delete metadata for a Docker host
    static async deleteMetadata(hostId: string): Promise<void> {
        await apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(hostId)}`, {
            method: 'DELETE',
        });
    }
}
