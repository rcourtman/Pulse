// Guest Metadata API
import { apiFetchJSON } from '@/utils/apiClient';

export interface GuestMetadata {
  id: string;
  customUrl?: string;
  description?: string;
  tags?: string[];
  notes?: string[]; // User annotations for AI context
}

export class GuestMetadataAPI {
  private static baseUrl = '/api/guests/metadata';

  // Get metadata for a specific guest
  static async getMetadata(guestId: string): Promise<GuestMetadata> {
    return apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(guestId)}`);
  }

  // Get all guest metadata
  static async getAllMetadata(): Promise<Record<string, GuestMetadata>> {
    return apiFetchJSON(this.baseUrl);
  }

  // Update metadata for a guest
  static async updateMetadata(
    guestId: string,
    metadata: Partial<GuestMetadata>,
  ): Promise<GuestMetadata> {
    return apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(guestId)}`, {
      method: 'PUT',
      body: JSON.stringify(metadata),
    });
  }

  // Delete metadata for a guest
  static async deleteMetadata(guestId: string): Promise<void> {
    await apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(guestId)}`, {
      method: 'DELETE',
    });
  }
}
