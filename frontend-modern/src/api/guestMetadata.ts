// Guest Metadata API
import { buildMetadataAPI, type ResourceMetadataRecord } from './metadataClient';

export interface GuestMetadata extends ResourceMetadataRecord {}

const guestMetadataAPI = buildMetadataAPI<GuestMetadata>('/api/guests/metadata');

export class GuestMetadataAPI {
  // Get metadata for a specific guest
  static async getMetadata(guestId: string): Promise<GuestMetadata> {
    return guestMetadataAPI.getMetadata(guestId);
  }

  // Get all guest metadata
  static async getAllMetadata(): Promise<Record<string, GuestMetadata>> {
    return guestMetadataAPI.getAllMetadata();
  }

  // Update metadata for a guest
  static async updateMetadata(
    guestId: string,
    metadata: Partial<GuestMetadata>,
  ): Promise<GuestMetadata> {
    return guestMetadataAPI.updateMetadata(guestId, metadata);
  }

  // Delete metadata for a guest
  static async deleteMetadata(guestId: string): Promise<void> {
    await guestMetadataAPI.deleteMetadata(guestId);
  }
}
