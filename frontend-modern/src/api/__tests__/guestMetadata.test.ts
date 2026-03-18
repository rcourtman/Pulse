import { describe, expect, it, vi, beforeEach } from 'vitest';
import { GuestMetadataAPI, type GuestMetadata } from '../guestMetadata';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('GuestMetadataAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getMetadata', () => {
    it('fetches metadata for a specific guest', async () => {
      const mockMetadata: GuestMetadata = {
        id: 'guest-1',
        customUrl: 'https://example.com',
        description: 'Test guest',
        tags: ['production', 'web'],
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockMetadata);

      const result = await GuestMetadataAPI.getMetadata('guest-1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/guests/metadata/guest-1');
      expect(result).toEqual(mockMetadata);
    });

    it('encodes special characters in guestId', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ id: 'guest+1' });

      await GuestMetadataAPI.getMetadata('guest+1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/guests/metadata/guest%2B1');
    });
  });

  describe('getAllMetadata', () => {
    it('fetches all guest metadata', async () => {
      const mockAllMetadata: Record<string, GuestMetadata> = {
        'guest-1': { id: 'guest-1', description: 'Guest 1' },
        'guest-2': { id: 'guest-2', description: 'Guest 2' },
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockAllMetadata);

      const result = await GuestMetadataAPI.getAllMetadata();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/guests/metadata');
      expect(result).toEqual(mockAllMetadata);
    });
  });

  describe('updateMetadata', () => {
    it('updates metadata for a guest', async () => {
      const updatedMetadata: GuestMetadata = {
        id: 'guest-1',
        description: 'Updated description',
        tags: ['updated'],
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(updatedMetadata);

      const result = await GuestMetadataAPI.updateMetadata('guest-1', {
        description: 'Updated description',
        tags: ['updated'],
      });

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/guests/metadata/guest-1',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify({ description: 'Updated description', tags: ['updated'] }),
        }),
      );
      expect(result).toEqual(updatedMetadata);
    });
  });

  describe('deleteMetadata', () => {
    it('deletes metadata for a guest', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(undefined);

      await GuestMetadataAPI.deleteMetadata('guest-1');

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/guests/metadata/guest-1',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });
  });
});
