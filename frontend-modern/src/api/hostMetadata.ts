// Host Metadata API
import { apiFetchJSON } from '@/utils/apiClient';

export interface HostMetadata {
    id: string;
    customUrl?: string;
    description?: string;
    tags?: string[];
    notes?: string[]; // User annotations for AI context
}

export class HostMetadataAPI {
    private static baseUrl = '/api/hosts/metadata';

    // Get metadata for a specific host
    static async getMetadata(hostId: string): Promise<HostMetadata> {
        return apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(hostId)}`);
    }

    // Get all host metadata
    static async getAllMetadata(): Promise<Record<string, HostMetadata>> {
        return apiFetchJSON(this.baseUrl);
    }

    // Update metadata for a host
    static async updateMetadata(
        hostId: string,
        metadata: Partial<HostMetadata>,
    ): Promise<HostMetadata> {
        return apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(hostId)}`, {
            method: 'PUT',
            body: JSON.stringify(metadata),
        });
    }

    // Delete metadata for a host
    static async deleteMetadata(hostId: string): Promise<void> {
        await apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(hostId)}`, {
            method: 'DELETE',
        });
    }
}
