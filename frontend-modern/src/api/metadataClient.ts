import { apiFetchJSON } from '@/utils/apiClient';

export interface ResourceMetadataRecord {
  id: string;
  customUrl?: string;
  description?: string;
  tags?: string[];
  notes?: string[];
}

export interface ResourceMetadataAPI<T extends ResourceMetadataRecord> {
  getMetadata(id: string): Promise<T>;
  getAllMetadata(): Promise<Record<string, T>>;
  updateMetadata(id: string, metadata: Partial<T>): Promise<T>;
  deleteMetadata(id: string): Promise<void>;
}

export function buildMetadataAPI<T extends ResourceMetadataRecord>(
  baseUrl: string,
): ResourceMetadataAPI<T> {
  const buildDetailPath = (id: string): string => `${baseUrl}/${encodeURIComponent(id)}`;

  return {
    getMetadata(id: string): Promise<T> {
      return apiFetchJSON<T>(buildDetailPath(id));
    },

    getAllMetadata(): Promise<Record<string, T>> {
      return apiFetchJSON<Record<string, T>>(baseUrl);
    },

    updateMetadata(id: string, metadata: Partial<T>): Promise<T> {
      return apiFetchJSON<T>(buildDetailPath(id), {
        method: 'PUT',
        body: JSON.stringify(metadata),
      });
    },

    async deleteMetadata(id: string): Promise<void> {
      await apiFetchJSON(buildDetailPath(id), {
        method: 'DELETE',
      });
    },
  };
}
