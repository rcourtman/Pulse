import { describe, expect, it, vi, beforeEach } from 'vitest';
import { HostMetadataAPI, type HostMetadata } from '../hostMetadata';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('HostMetadataAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getMetadata', () => {
    it('fetches metadata for a specific host', async () => {
      const mockMetadata: HostMetadata = {
        id: 'host-1',
        customUrl: 'https://example.com',
        description: 'Test host',
        tags: ['production', 'web'],
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockMetadata);

      const result = await HostMetadataAPI.getMetadata('host-1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/hosts/metadata/host-1');
      expect(result).toEqual(mockMetadata);
    });

    it('encodes special characters in hostId', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ id: 'host+1' });

      await HostMetadataAPI.getMetadata('host+1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/hosts/metadata/host%2B1');
    });
  });

  describe('getAllMetadata', () => {
    it('fetches all host metadata', async () => {
      const mockAllMetadata: Record<string, HostMetadata> = {
        'host-1': { id: 'host-1', description: 'Host 1' },
        'host-2': { id: 'host-2', description: 'Host 2' },
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockAllMetadata);

      const result = await HostMetadataAPI.getAllMetadata();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/hosts/metadata');
      expect(result).toEqual(mockAllMetadata);
    });
  });

  describe('updateMetadata', () => {
    it('updates metadata for a host', async () => {
      const updatedMetadata: HostMetadata = {
        id: 'host-1',
        description: 'Updated description',
        tags: ['updated'],
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(updatedMetadata);

      const result = await HostMetadataAPI.updateMetadata('host-1', {
        description: 'Updated description',
        tags: ['updated'],
      });

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/hosts/metadata/host-1',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify({ description: 'Updated description', tags: ['updated'] }),
        }),
      );
      expect(result).toEqual(updatedMetadata);
    });
  });

  describe('deleteMetadata', () => {
    it('deletes metadata for a host', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(undefined);

      await HostMetadataAPI.deleteMetadata('host-1');

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/hosts/metadata/host-1',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });
  });
});
