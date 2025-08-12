// Guest Metadata API
export interface GuestMetadata {
  id: string;
  customUrl?: string;
  description?: string;
  tags?: string[];
}

export class GuestMetadataAPI {
  private static baseUrl = '/api/guests/metadata';

  // Get authentication headers
  private static getHeaders(includeContentType = false): HeadersInit {
    const headers: HeadersInit = {};
    
    const apiToken = localStorage.getItem('apiToken');
    if (apiToken) {
      headers['X-API-Token'] = apiToken;
    }
    
    if (includeContentType) {
      headers['Content-Type'] = 'application/json';
    }
    
    return headers;
  }

  // Get metadata for a specific guest
  static async getMetadata(guestId: string): Promise<GuestMetadata> {
    const response = await fetch(`${this.baseUrl}/${encodeURIComponent(guestId)}`, {
      headers: this.getHeaders()
    });
    if (!response.ok) {
      throw new Error('Failed to fetch guest metadata');
    }
    return response.json();
  }

  // Get all guest metadata
  static async getAllMetadata(): Promise<Record<string, GuestMetadata>> {
    const response = await fetch(this.baseUrl, {
      headers: this.getHeaders()
    });
    if (!response.ok) {
      throw new Error('Failed to fetch all guest metadata');
    }
    return response.json();
  }

  // Update metadata for a guest
  static async updateMetadata(guestId: string, metadata: Partial<GuestMetadata>): Promise<GuestMetadata> {
    const response = await fetch(`${this.baseUrl}/${encodeURIComponent(guestId)}`, {
      method: 'PUT',
      headers: this.getHeaders(true),
      body: JSON.stringify(metadata),
    });
    
    if (!response.ok) {
      throw new Error('Failed to update guest metadata');
    }
    return response.json();
  }

  // Delete metadata for a guest
  static async deleteMetadata(guestId: string): Promise<void> {
    const response = await fetch(`${this.baseUrl}/${encodeURIComponent(guestId)}`, {
      method: 'DELETE',
      headers: this.getHeaders()
    });
    
    if (!response.ok) {
      throw new Error('Failed to delete guest metadata');
    }
  }
}