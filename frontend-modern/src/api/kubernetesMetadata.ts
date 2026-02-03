// Kubernetes Metadata API
import { apiFetchJSON } from '@/utils/apiClient';

export interface KubernetesMetadata {
    id: string; // resource ID
    customUrl?: string;
    description?: string;
    tags?: string[];
    notes?: string[]; // User annotations for AI context
}

export class KubernetesMetadataAPI {
    private static baseUrl = '/api/kubernetes/metadata';

    // Get metadata for a specific kubernetes resource (pod, deployment, etc.)
    static async getMetadata(resourceId: string): Promise<KubernetesMetadata> {
        return apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(resourceId)}`);
    }

    // Get all kubernetes metadata
    static async getAllMetadata(): Promise<Record<string, KubernetesMetadata>> {
        return apiFetchJSON(this.baseUrl);
    }

    // Update metadata for a kubernetes resource
    static async updateMetadata(
        resourceId: string,
        metadata: Partial<KubernetesMetadata>,
    ): Promise<KubernetesMetadata> {
        return apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(resourceId)}`, {
            method: 'PUT',
            body: JSON.stringify(metadata),
        });
    }

    // Delete metadata for a kubernetes resource
    static async deleteMetadata(resourceId: string): Promise<void> {
        await apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(resourceId)}`, {
            method: 'DELETE',
        });
    }
}
